# URL Shortener

This is my Go HTTP service that shortens long URLs and redirects short codes back to the original link. I built this entirely using the Go standard library (no third-party modules like godotenv or ORMs). 

Every code it generates is exactly 8 characters: my tag `sg` + 6 random base62 characters (for example, `sgaB3dE9`).

## Requirements
- Go 1.22+

## How to run it

```bash
# Run with in-memory storage (default)
go run ./cmd/server -addr :8080 -base http://localhost:8080

# Run with persistent storage (saves to a JSON file)
go run ./cmd/server -store links.json
```

Here are the flags you can use:

| Flag     | Default                 | Meaning                                   |
|----------|-------------------------|-------------------------------------------|
| `-addr`  | `:8080`                 | The port to listen on |
| `-base`  | `http://localhost:8080` | Public base URL used to build the `short_url` response |
| `-store` | _(empty = in-memory)_   | Path to a JSON file to save data. If left empty, it uses memory. |
| `-rate`  | `10`                    | Rate limit for `POST /api/shorten` per IP per minute (0 = unlimited) |

*(Note: The server handles graceful shutdown. If you hit `Ctrl-C`, it waits up to 10 seconds to finish any active requests before closing.)*

## API Endpoints

| Method | Path                   | Success                                    | Errors |
|--------|------------------------|--------------------------------------------|--------|
| POST   | `/api/shorten`         | **201** `{"code":"…","short_url":"…"}`     | **400** bad JSON / missing / invalid URL |
| GET    | `/api/v1/links/{code}` | **200** `{"url":"…","created_at":"…"}`     | **404** unknown code |
| GET    | `/{code}`              | **302** with `Location: <long url>`        | **404** unknown code |

Errors are always returned as JSON: `{"error":"…"}`.

### Examples

**Shorten a link:**
```bash
curl -s -X POST localhost:8080/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://go.dev/doc/"}'
# {"code":"sglCtrPY","short_url":"http://localhost:8080/sglCtrPY"}
```

**Test the redirect:**
```bash
curl -sI localhost:8080/sglCtrPY
# HTTP/1.1 302 Found
# Location: https://go.dev/doc/
```

**Idempotency (sending the same URL again returns the exact same code):**
```bash
curl -s -X POST localhost:8080/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://go.dev/doc/"}'
# {"code":"sglCtrPY","short_url":"http://localhost:8080/sglCtrPY"}
```

**Error handling (bad URL):**
```bash
curl -s -X POST localhost:8080/api/shorten -d '{"url":"ftp://x"}'
# {"error":"invalid url: scheme must be http or https"}   (400)
```

## Running tests

```bash
go vet ./...
go test ./...
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -n1
```
*(Note on `-race`: My Windows machine doesn't have a C compiler installed, so `go test -race ./...` doesn't work locally for me, but the code is fully concurrent-safe using RWMutex!)*

Current coverage: `total: (statements) 94.5%`

## Benchmarks

You can run the benchmarks with:
```bash
go test -bench=. -benchmem ./internal/httpapi/
```

Here's what I got on my machine (i7-1065G7):

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| Shorten (idempotent) | ~11,200 | 6,987 | 32 |
| Shorten (distinct) | ~18,700 | 7,524 | 35 |
| Redirect | ~11,300 | 6,381 | 23 |
| Lookup | ~13,100 | 6,285 | 22 |

Because I used `sync.RWMutex`, redirects and lookups just take a read lock (`RLock`) so they run extremely fast in parallel. The most expensive part of shortening a new URL is the `crypto/rand` call for secure random generation.

## Project layout

```text
cmd/server/            entrypoint: flags, wiring, graceful shutdown
cmd/loadtest/          concurrent load-test tool (prints req/s)
internal/shortener/    domain: Link, errors, Normalize, NewCode, Service, Store interface
internal/store/memory/ in-memory Store (maps + sync.RWMutex)
internal/store/file/   JSON file-backed Store (persistent across restarts)
internal/httpapi/      HTTP routes, JSON, error mapping, rate limiter
```

Check out [DECISIONS.md](DECISIONS.md) to read my writeup on scaling and design choices, and [CHECKLIST.md](CHECKLIST.md) to see my progress against the requirements!
