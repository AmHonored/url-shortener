# Design decisions

## Dependencies

None. Only the Go standard library is used (`net/http` routing from Go 1.22).

## AI usage

The code in this repository was written with an AI coding assistant
(Antigravity), directed and reviewed by me.

## Part 1

### Package layout
- `cmd/server`: flags and wiring only.
- `internal/shortener`: domain — `Link`, sentinel errors, `Normalize`, `NewCode`, `Service`, `Store` interface.
- `internal/store/memory`: in-memory `Store` (two maps + `sync.RWMutex`).
- `internal/httpapi`: HTTP handlers, JSON encoding, error → status mapping.
- Dependency direction: `httpapi → shortener ← store/memory`.

### Normalization
`Normalize` builds a canonical string used as the identity key: lowercase scheme/host, strip default port, empty path → `/`, drop fragment. Path case, query, and trailing slashes are preserved because the target server may treat them differently.

### Validation
HTTP(S) only, non-empty host, no credentials (`user:pass@`), max 2048 chars. URL is never fetched (no SSRF). Errors don't echo the URL.

### Idempotency
Two maps: `byCode` (redirect path) and `byURL` (reverse index). `Create` returns the existing link if the URL is already stored; the generated code is discarded.

### Code generation
`sg` + 6 random base62 chars from `crypto/rand` with rejection sampling (no modulo bias). Random codes can't be enumerated and need no shared counter.

### Collision handling
62⁶ ≈ 56.8B codes. On `ErrCodeExists` the service retries up to 5 times.

### Concurrency
One `sync.RWMutex` guards both maps. `Get` takes `RLock`; `Create` takes `Lock` for atomic check-and-insert.

### HTTP
Go 1.22 `ServeMux` patterns. Request bodies capped at 1 MB. `-base` flag validated at startup.

## Part 2

### Metadata route
`GET /api/v1/links/{code}` returns 200 with `{"url":"…","created_at":"…"}` (RFC 3339) or 404. Reuses the same `Service.Resolve` path as the redirect handler.

### Error handling
`ErrNotFound`, `ErrInvalidURL`, `ErrCodeExists` are sentinel errors in the domain package. All wrapping uses `%w`; the HTTP layer maps them with `errors.Is`.

### Store interface
Defined in the domain package (`shortener.Store`) so storage depends on the domain, not the other way around. Tests use a `fakeStore` to verify service logic in isolation.

## Part 3

### Server timeouts
`http.Server` has `ReadTimeout` (5 s), `WriteTimeout` (10 s), and `IdleTimeout` (120 s) to prevent slow-client DoS. Bare `http.ListenAndServe` has none.

### Mutex choice
`sync.RWMutex` — redirects (`Get`) are the hot path and only need `RLock`, so they run in parallel. `Create` needs exclusive `Lock` for the atomic check-and-insert. A plain `Mutex` would serialize all reads unnecessarily.

### Benchmarks
`benchmark_test.go` covers idempotent shorten, distinct-URL shorten, redirect, and metadata lookup via `httptest`. These are end-to-end through the router so they measure realistic handler cost.

### Eviction
Not implemented. At this scale the in-memory map is small enough. Adding an LRU eviction would complicate the idempotency guarantee (a second POST after eviction would mint a new code for the same URL).

## Part 4

### File store
`store/file` keeps the same two-map structure as the memory store but writes the full link set to a JSON file on every `Create`. The file is read once at startup (`load`) and rewritten atomically on each write (`save`). No external dependencies needed.

### Persistence guarantee
`save()` runs inside the write lock, before `Create` returns. If the write fails, the in-memory maps are rolled back, so the response is never sent for data that wasn't persisted.

### Restart test
`TestRestart` creates a store, writes a link, then opens a **new** `file.Store` from the same path and verifies the link survived.

### Config
`-store <path>` selects the file-backed store; omitting it keeps the default in-memory store.

### How `created_at` is stored
`Link.CreatedAt` is a `time.Time` field. JSON marshals it as an RFC 3339 string, so it round-trips through the file store without loss.

### Idempotency after restart
The `byURL` reverse index is rebuilt during `load()`, so the same URL returns the same code even after a restart.

## Part 5

### Stateless app — load balancer → N replicas → shared store

Each Go replica is stateless: it holds no per-request data. A load balancer (nginx, HAProxy, AWS ALB) distributes requests across N replicas by IP-hash or round-robin. Sticky sessions are not needed because every replica reads from the same shared store.

The shared store replaces the in-memory map and file store. The best choice at scale is **PostgreSQL** with a `(original_url)` unique index for idempotency and a `(short_code)` unique index for redirect lookups. Both paths become a single indexed lookup — O(log n) — regardless of how many replicas exist.

For extreme read throughput a **Redis** layer sits in front of Postgres: redirect reads (`GET /{code}`) check Redis first (`GET code`), falling back to Postgres on a miss, and then writing the result back to Redis with a TTL (e.g. 24 h). The write path (`POST /api/shorten`) always goes to Postgres first and then invalidates or pre-warms the Redis key.

### CDN / edge caching for redirects

`GET /{code}` responses carry `Cache-Control: public, max-age=3600`. A CDN (Cloudflare, CloudFront) caches the **302** response at the edge and serves millions of redirects without hitting the origin at all. Tradeoff: if a link is deleted or updated, the cached redirect stays stale until the TTL expires. For this use-case (links are immutable) that is acceptable.

### Write-path scaling

Three options ranked by complexity:
1. **Rate limiting** (implemented) — reject abusive clients early; stops runaway traffic from a single IP.
2. **Pre-generated code pool** — a background worker fills a Redis list with random codes. Shorten picks one atomically (`LPOP`), eliminating the `crypto/rand` call from the hot path and reducing per-request latency.
3. **Async queue** — POST returns 202 Accepted immediately; a worker persists and indexes in the background. Tradeoff: the short URL is not immediately usable after the 202.

### Bloom filter

A Bloom filter in front of the redirect store answers "does this code definitely not exist?" in O(1) with zero DB load. A negative answer (code definitely absent) returns 404 immediately. A positive answer (code may exist) does a normal DB lookup. False-positive rate is tunable by filter size. This is most valuable when the code space is large and many lookup requests are for codes that don't exist (e.g. typos, scanners).

### Sharding

When one Postgres instance cannot handle write throughput, shard by the first character of the code (62 shards possible). Each shard owns its own DB. The application layer routes by `code[0]`. Shard-local uniqueness is sufficient because `crypto/rand` codes are globally unique with overwhelming probability (62^6 ≈ 56 B codes across all shards).

## Part 6

### Graceful shutdown

`signal.Notify` listens for `SIGINT`/`SIGTERM`. On signal, `Server.Shutdown(ctx)` stops accepting new connections and waits up to **10 seconds** for in-flight requests to finish before returning. This means a rolling deploy or `Ctrl-C` never cuts a redirect mid-flight.

### Rate limiting

A fixed-window counter per IP per minute is implemented in `internal/httpapi/ratelimit.go`. The `-rate` flag (default 10) sets the limit; `0` disables it. When a client exceeds the limit the server returns **429 Too Many Requests** with a JSON error body. The window resets every 60 seconds.

Tradeoffs: fixed windows allow a burst of 2× the limit across a window boundary. A sliding-window or token-bucket would be fairer but adds complexity. For this use-case (protecting the shorten endpoint) fixed-window is sufficient.

### Logging

`log.Printf` is used for structured-enough output without dependencies. The rules:
- **Log:** server start/stop, internal errors (status 5xx), store selection.
- **Never log:** submitted URLs (may contain query-string secrets), request bodies, `Location` headers (contain the long URL).

### Load testing

`cmd/loadtest` runs concurrent shorten and redirect phases and prints req/s. Sample output on the development machine:

```
POST /api/shorten (same URL, idempotent)          ~9,200 req/s  ok=1000 fail=0
GET /{code} (redirect, hot code)                  ~11,500 req/s ok=1000 fail=0
```

