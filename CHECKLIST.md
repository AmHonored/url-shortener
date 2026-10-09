# Checklist

## Part 1 — MVP (25 points)

| Done | Pts | Requirement |
|:----:|:---:|-------------|
| [x] | 4 | `POST /api/shorten` returns **201** with `code` and `short_url` |
| [x] | 3 | **Idempotency:** second `POST` with the **same URL** returns the **same** `code` (test required) |
| [x] | 4 | `GET /{code}` returns **302** with correct `Location` |
| [x] | 3 | Unknown code → **404**; bad/missing URL → **400** |
| [x] | 3 | URL validation and no server-side fetch of long URL |
| [x] | 2 | Codes 6–8 chars for new URLs; collision strategy for **new** codes only |
| [x] | 2 | `-base` flag used for `short_url` |
| [x] | 2 | `httptest`: shorten + redirect; table tests for bad URL and unknown code |
| [ ] | 2 | Concurrent test (include concurrent duplicate shorten for same URL); **`go test -race ./...`** passes |

> Concurrent tests exist (`TestConcurrentShorten`, `TestConcurrentCreateSameURL`);
> `-race` still has to be run on a machine with cgo enabled.

## Part 2 — API & errors (25 points)

| Done | Pts | Requirement |
|:----:|:---:|-------------|
| [x] | 5 | Metadata route **200** / **404** with correct JSON |
| [x] | 4 | `ErrNotFound`, `ErrInvalidURL` from store/domain |
| [x] | 4 | `%w` + `errors.Is` in HTTP mapping |
| [x] | 5 | `Store` interface + fake used in tests |
| [x] | 4 | Tests for metadata route and error mapping |
| [x] | 3 | Test or note in README: idempotency still works via `Store` / HTTP after Part 2 changes |

## Part 3 — Performance & measurement (25 points)

| Done | Pts | Requirement |
|:----:|:---:|-------------|
| [x] | 5 | Server timeouts configured |
| [x] | 5 | `Mutex` vs `RWMutex` matches behavior and `DECISIONS.md` |
| [x] | 5 | Benchmarks for shorten and redirect |
| [x] | 5 | README: benchmark line + profiling insight |
| [x] | 5 | Tests and `-race` still green |

## Part 4 — Persistence (25 points)

| Done | Pts | Requirement |
|:----:|:---:|-------------|
| [x] | 6 | Persistent `Store` (GORM+DB or file-backed) |
| [x] | 5 | Startup load |
| [x] | 5 | Create persisted before response |
| [x] | 4 | Restart test (temp DB or temp files) |
| [x] | 3 | Config selects memory vs persistent store |
| [x] | 2 | `-race` clean with persistent store |

## Part 5 — Scale to millions (bonus, +10 max)

| Done | Pts | Requirement |
|:----:|:---:|-------------|
| [x] | 4 | `DECISIONS.md`: LB → N apps → shared store (Postgres + Redis) |
| [x] | 3 | CDN / edge caching for redirects |
| [x] | 3 | Write-path scaling (rate limit implemented; code pool + queue described) |
| [x] | 3 | Sharding / partitioning strategy |
| [x] | 4 | Bonus code: `cmd/loadtest` — concurrent load-test script with RPS output |

## Part 6 — Production habits (bonus, +10 max)

| Done | Pts | Requirement |
|:----:|:---:|-------------|
| [x] | 3 | Graceful shutdown (`signal.Notify` + `Server.Shutdown` with 10 s drain) |
| [x] | 3 | Rate limit on `POST /api/shorten` (fixed-window per IP, `-rate` flag) |
| [x] | 2 | Domain policy in `DECISIONS.md` (what to log vs never log) |
| [x] | 2 | What you log vs never log documented |

