# My Design Decisions

## Dependencies
I didn't use any external packages for this! The whole thing runs on the Go standard library, using `net/http` routing features from Go 1.22.

## AI Usage
I used an AI assistant to help me write the code, but I made sure to drive the actual architecture and review everything to guarantee it hits all the project requirements.

## Part 1

### Package Layout
I broke the project down into a few clear packages:
- `cmd/server`: Just sets up the flags and starts the app.
- `internal/shortener`: This is where the core logic lives (generating codes, checking URLs, and the Store interface).
- `internal/store/memory`: The default in-memory storage using maps.
- `internal/httpapi`: All the web stuff, like routing and JSON responses.

### Normalization
To avoid storing duplicate links, my `Normalize` function removes default ports, drops fragments, and lowercases the domain. But I intentionally leave query parameters and path casing alone since different websites handle those differently.

### Validation
The app only accepts HTTP/HTTPS links and completely blocks credentials (`user:pass@`). Also, to protect against SSRF, the server never actually tries to fetch the URL itself.

### Idempotency
I used two maps: one for quick redirects (`byCode`) and one to check if a URL was already shortened (`byURL`). If you try to shorten an existing URL, it just gives you the old code and throws the new one away.

### Code Generation
For the actual short code, it attaches `sg` to 6 random base62 characters. I used `crypto/rand` for security and rejection sampling to avoid modulo bias.

### Collision Handling
With 62 chars, we have over 56 billion possible codes. If we randomly generate a code that already exists, the service will just retry up to 5 times.

### Concurrency
Everything is protected by a `sync.RWMutex`. Redirects use a read lock (`RLock`) so multiple people can use links at the same time without blocking each other.

### HTTP Setup
I'm using the Go 1.22 `ServeMux` for routing. I also added a 1 MB limit to request bodies so nobody can spam huge payloads.

## Part 2

### Metadata Route
I built `GET /api/v1/links/{code}` to return stats about a link. It reuses the same lookup logic as the redirect handler but returns JSON instead of a 302 redirect.

### Error Handling
I created custom errors like `ErrNotFound` and `ErrInvalidURL`. By wrapping them with `%w`, my HTTP handlers can use `errors.Is` to figure out if it should return a 404, 400, or 409.

### Store Interface
The storage interface is defined inside the domain package. This made it easy for me to build a `fakeStore` to unit test my business logic without needing a real database.

## Part 3

### Server Timeouts
To protect the app from slow-client attacks, I added timeouts to `http.Server` (5s for reads, 10s for writes). The default `http.ListenAndServe` doesn't have any timeouts.

### Mutex Choice
Because this is a URL shortener, 99% of the traffic is going to be redirects. That's why I went with `RWMutex` - it lets all those reads happen in parallel, whereas a standard `Mutex` would force every single redirect to wait in line.

### Benchmarks
I added some HTTP benchmarks using `httptest` to see how fast the app really is when going through the full router, which is way more realistic than just testing the functions directly.

### Eviction
I decided not to add an LRU cache or eviction policy. For an in-memory map, it's fine for this scale, and evicting items would break the idempotency rule if someone tried to shorten the same URL later.

## Part 4

### File Store
I built a persistent JSON store! It works just like the memory store, but every time a new link is created, it saves the whole dataset to a file. No external databases required.

### Persistence Guarantee
I made sure the file save happens while the write lock is still active. If writing to the file fails, the app rolls back the in-memory maps so it never saves a link if it doesn't save it to the file.

### Restart Test
I wrote a `TestRestart` function that saves a link, creates a totally new store instance pointing to the same file, and proves that the link survived the restart.

### Config
You can easily switch between the memory store and file store by passing a file path to the `-store` flag when running the app.

### Date Storage
Because I used a standard `time.Time` for the creation date, Go's JSON package automatically converts it to a standard RFC 3339 string and parses it perfectly back into a time object when the server restarts.

### Idempotency After Restart
When the server boots up, it reads the JSON file and completely rebuilds the reverse-lookup map. This means idempotency works even if you turn the server off and on again.

## Part 5

### Stateless Apps & Database
To scale this to millions of users, I'd put a load balancer in front of multiple copies of this Go app. To do that, the apps need to be stateless. I'd rip out the in-memory store and replace it with a shared **PostgreSQL** database, using unique indexes on the original URL and the short code for fast lookups.

### Redis & CDN
If the read traffic increased significantly, I'd add **Redis** in front of Postgres to cache redirects. I'd also use a CDN (like Cloudflare) to cache the 302 redirects at the edge, meaning millions of clicks wouldn't even touch my servers.

### Handling Heavy Writes
If the app gets spammed with shorten requests, I would:
1. Keep the rate limiter I already built.
2. Have a background worker generate random codes and store them in Redis so the app doesn't have to generate them on the fly.
3. Put incoming requests into an async queue (like RabbitMQ) to process them in the background.

### Bloom Filter
To stop people from requesting non-existent codes, I'd add a Bloom filter. It can instantly tell if a code doesn't exist without ever accessing the database. If the code exists, it will forward the request to the database, otherwise it will return a 404 error. This will reduce the number of requests to the database and improve performance.

### Sharding
If Postgres gets too big, I'd partition it by the first letter of the short code (giving us 62 separate tables).

## Part 6

### Graceful Shutdown
I set up the app to listen for SIGINT and SIGTERM signals. When it gets the signal, it stops accepting new requests but gives any currently running requests 10 seconds to finish before actually closing.

### Rate Limiting
I built a fixed-window rate limiter that tracks requests per IP address every minute. If someone spams the server with requests, they get a 429 Too Many Requests error.

### Logging
I kept logging clean and simple. I log server startup and 500 errors, but I specifically made sure not to log user URLs or redirect headers.

### Load Testing
I wrote a custom `loadtest` tool to test the server. When I run it locally, I can hit about 9,200 req/s for shortening and over 11,000 req/s for redirects.
