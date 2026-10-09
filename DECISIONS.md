# Design decisions

## Dependencies
I didn't use any external dependencies. Everything is built with the Go standard library (using the new `net/http` routing from Go 1.22). Keeping it simple!

## AI usage
I built this project with the help of an AI coding assistant (Antigravity), but I directed the architecture and reviewed everything to make sure it meets the requirements.

## Part 1

### Package layout
I structured the project like this:
- `cmd/server`: just flags and wiring up the server.
- `internal/shortener`: the core domain stuff. It has the `Link` struct, my custom errors, the logic for generating codes, and the `Store` interface.
- `internal/store/memory`: my in-memory storage implementation (uses two maps and a `sync.RWMutex`).
- `internal/httpapi`: all the HTTP handlers, JSON encoding, and mapping my custom errors to HTTP status codes.

The dependency flows from `httpapi` -> `shortener` <- `store/memory`. This keeps the core logic independent.

### Normalization
To make sure we don't store the exact same link twice, I wrote a `Normalize` function. It lowercases the scheme and host, removes default ports (like 80 or 443), sets an empty path to `/`, and drops the fragment. I purposely kept the path case, query params, and trailing slashes intact because different websites treat those differently.

### Validation
I made sure it only accepts HTTP and HTTPS. It rejects empty hosts, long URLs (capped at 2048 chars), and URLs with credentials (`user:pass@`). Importantly, the server never actually fetches the URL so we are safe from SSRF attacks. Also, I don't echo back the bad URL in the error response just in case.

### Idempotency
I used two maps in my store: `byCode` for fast redirects and `byURL` to act as a reverse index. When someone tries to `Create` a link, I check `byURL` first. If it's already there, I just return the existing code and throw away the newly generated one.

### Code generation
For the short codes, I used `sg` plus 6 random base62 characters. I used `crypto/rand` for secure randomness, and I made sure to use rejection sampling so there's no modulo bias. Because it's random, people can't easily guess the next code, and I didn't need to mess with a shared counter.

### Collision handling
62 chars to the power of 6 gives us about 56.8 billion combinations. If by some crazy chance we hit a collision (`ErrCodeExists`), my service will just retry generating a new code up to 5 times.

### Concurrency
I went with a single `sync.RWMutex` to protect both maps. For redirects (`Get`), I just use an `RLock` so multiple people can get redirected at the same time. For creating links (`Create`), I use a full `Lock` so I can safely check if it exists and insert it in one atomic step.

### HTTP
I used the new Go 1.22 `ServeMux` patterns which makes routing super clean. I also capped request bodies at 1 MB so nobody can crash the server with massive payloads.

## Part 2

### Metadata route
I added `GET /api/v1/links/{code}` to return stats. It gives a 200 OK with `{"url":"...","created_at":"..."}` (using RFC 3339 time format) or a 404 if it doesn't exist. Under the hood, it reuses the same `Service.Resolve` method that the redirect handler uses.

### Error handling
I created some sentinel errors in the domain package: `ErrNotFound`, `ErrInvalidURL`, and `ErrCodeExists`. When errors happen, I wrap them using `%w`, and then my HTTP layer checks for them using `errors.Is` to return the right status code (like 404 or 400).

### Store interface
I put the `Store` interface right in the `shortener` package. This way, the storage depends on my domain rules, not the other way around. It also let me create a `fakeStore` for unit testing the service logic without dealing with real storage.

## Part 3

### Server timeouts
I added some timeouts to the `http.Server` because `http.ListenAndServe` doesn't have any by default. I set `ReadTimeout` to 5s, `WriteTimeout` to 10s, and `IdleTimeout` to 120s. This protects against slow-client DoS attacks.

### Mutex choice
I stuck with `sync.RWMutex`. In a URL shortener, redirects (reads) happen way more often than creating new links (writes). `RLock` lets all those reads happen in parallel. If I used a regular `Mutex`, it would bottleneck all the redirects behind each other.

### Benchmarks
I wrote benchmarks in `benchmark_test.go` for shortening (both new and existing URLs), redirecting, and looking up metadata. I did them through `httptest` so it measures the actual HTTP handler overhead, giving a more realistic picture of performance.

### Eviction
I decided not to implement eviction. For the scale of this project, the in-memory map is fine. If I added an LRU cache, it would mess up my idempotency (if a URL gets evicted and someone shortens it again, they'd get a different code).

## Part 4

### File store
I added `store/file` as my persistent storage option. It uses the exact same two-map logic, but every time `Create` is called, it marshals the whole dataset into JSON and writes it to a file. It reads this file once at startup (`load`). Still no external DB dependencies!

### Persistence guarantee
The `save()` function happens *inside* the write lock, right before `Create` finishes. If writing to the file fails for some reason, I delete the entry from the in-memory maps so the user never gets a success response for data that wasn't actually saved.

### Restart test
I wrote `TestRestart` to prove this works. It makes a store, writes a link, and then creates a totally new `file.Store` pointing to the same temp file to make sure it loads the link back up.

### Config
I added a `-store <path>` flag. If you provide a file path, it uses the file store. If you leave it empty, it defaults to the in-memory store.

### How `created_at` is stored
The `CreatedAt` field is just a standard Go `time.Time`. Because I'm using JSON for the file store, it automatically marshals it into an RFC 3339 string, which parses perfectly back into `time.Time` when loaded.

### Idempotency after restart
Since the `byURL` reverse index is rebuilt from scratch during the `load()` phase at startup, idempotency works perfectly even after restarting the server.

## Part 5

### Scaling out (Stateless apps + DB)
Right now the app runs in one process. To scale it to millions, I'd put a load balancer (like nginx or AWS ALB) in front of multiple instances of my Go app. 

To make this work, the apps need to be stateless. I'd replace my in-memory/file store with a shared **PostgreSQL** database. I'd put a unique index on `original_url` to handle idempotency, and another unique index on `short_code` for fast redirects. Since they are indexed, lookups would be super fast O(log n) no matter how many app instances I run.

If read traffic gets insane, I'd throw **Redis** in front of Postgres. Redirect requests would hit Redis first. If the code isn't there, it checks Postgres, and then caches it in Redis with a TTL (like 24 hours). 

### Edge caching
For redirects, I'd return a `Cache-Control: public, max-age=3600` header and use a CDN like Cloudflare. The CDN caches the 302 redirect at the edge, meaning millions of users get redirected without my servers ever seeing the request. The downside is that if we ever wanted to delete a link, the CDN would still serve it for an hour, but for this project links are permanent so it's a great fit.

### Write scaling
If we get too many `POST /api/shorten` requests, here's what I'd do:
1. **Rate limiting** (which I actually implemented!) to block spammy IPs.
2. **Pre-generated codes**: Instead of generating random codes on the fly, a background worker could generate a bunch of codes and put them in a Redis list. The shorten handler would just `LPOP` a code instantly.
3. **Async queues**: Return a `202 Accepted` immediately and put the URL in a queue (like RabbitMQ) to be processed in the background.

### Bloom filter
To protect the database from people guessing random codes, I could use a Bloom filter. It can tell us if a code *definitely does not exist* in O(1) time without hitting the DB. If it says it doesn't exist, I immediately return a 404. 

### Sharding
If one Postgres instance gets too big, I could shard the database based on the first character of the short code (so 62 different shards). The Go app would look at the first letter and know exactly which DB to talk to.

## Part 6

### Graceful shutdown
I added a `signal.Notify` to listen for `Ctrl-C` (SIGINT/SIGTERM). When it gets the signal, it calls `Server.Shutdown()` with a 10-second timeout. This stops accepting new requests but lets any currently running requests finish up before killing the app.

### Rate limiting
I built a simple fixed-window rate limiter in `internal/httpapi/ratelimit.go`. It limits requests per IP address per minute (configurable with the `-rate` flag). If someone spams the shorten endpoint, they get a **429 Too Many Requests** error. The counters reset every 60 seconds.

### Logging
I just used the standard `log.Printf`. My rules are:
- **Do log:** server starting/stopping, major internal errors (500s), and which store backend is selected.
- **Don't log:** user-submitted URLs (they might have sensitive stuff in the query string) or `Location` headers.

### Load testing
I wrote a quick script in `cmd/loadtest` to hammer the server with concurrent requests and measure requests per second (RPS). On my machine it hits about 9,200 RPS for shortening and 11,500 RPS for redirects!
