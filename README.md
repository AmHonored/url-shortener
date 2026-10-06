# URL Shortener

A small Go HTTP service that shortens long URLs and redirects short codes to
the original link. Standard library only (no third-party modules).

Every code is 8 characters: the tag `sg` (sysgrp) + 6 random base62 chars,
e.g. `sgaB3dE9`.

## Requirements

- Go 1.22+

## Run

```bash
go run ./cmd/server -addr :8080 -base http://localhost:8080
```

| Flag    | Default                 | Meaning                                   |
|---------|-------------------------|-------------------------------------------|
| `-addr` | `:8080`                 | Listen address                            |
| `-base` | `http://localhost:8080` | Public base URL used to build `short_url` |

## API

| Method | Path                   | Success                                    | Errors |
|--------|------------------------|--------------------------------------------|--------|
| POST   | `/api/shorten`         | **201** `{"code":"…","short_url":"…"}`     | **400** bad JSON / missing / invalid URL |
| GET    | `/api/v1/links/{code}` | **200** `{"url":"…","created_at":"…"}`     | **404** unknown code |
| GET    | `/{code}`              | **302** with `Location: <long url>`        | **404** unknown code |

Errors have a JSON body: `{"error":"…"}`.

### Examples

```bash
curl -s -X POST localhost:8080/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://go.dev/doc/"}'
# {"code":"sglCtrPY","short_url":"http://localhost:8080/sglCtrPY"}

curl -sI localhost:8080/sglCtrPY
# HTTP/1.1 302 Found
# Location: https://go.dev/doc/

# same URL again → same code (still 201)
curl -s -X POST localhost:8080/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://go.dev/doc/"}'
# {"code":"sglCtrPY","short_url":"http://localhost:8080/sglCtrPY"}

curl -s -X POST localhost:8080/api/shorten -d '{"url":"ftp://x"}'
# {"error":"invalid url: scheme must be http or https"}   (400)
```

## Test

```bash
go vet ./...
go test ./...
go test -race ./...          # needs cgo (a C compiler, e.g. gcc) on the machine
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -n1
```

Current coverage: `total: (statements) 85.9%`

## Benchmarks

```bash
go test -bench=. -benchmem ./internal/httpapi/
```

Sample output (i7-1065G7):

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| Shorten (idempotent) | ~11,200 | 6,987 | 32 |
| Shorten (distinct) | ~18,700 | 7,524 | 35 |
| Redirect | ~11,300 | 6,381 | 23 |
| Lookup | ~13,100 | 6,285 | 22 |

Redirect and lookup are read-only (`RLock`) and run in parallel. The `crypto/rand` call dominates shorten cost.


## Project layout

```text
cmd/server/            entrypoint: flags + wiring (kept thin)
internal/shortener/    domain: Link, errors, Normalize, NewCode, Service, Store interface
internal/store/memory/ in-memory Store (maps + sync.RWMutex)
internal/httpapi/      HTTP routes, JSON, error → status mapping
```

See [DECISIONS.md](DECISIONS.md) for design choices and
[CHECKLIST.md](CHECKLIST.md) for progress.
