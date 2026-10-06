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
| [ ] | 5 | Server timeouts configured |
| [ ] | 5 | `Mutex` vs `RWMutex` matches behavior and `DECISIONS.md` |
| [ ] | 5 | Benchmarks for shorten and redirect |
| [ ] | 5 | README: benchmark line + profiling insight |
| [ ] | 5 | Tests and `-race` still green |

## Part 4 — Persistence (25 points)

| Done | Pts | Requirement |
|:----:|:---:|-------------|
| [ ] | 6 | Persistent `Store` (GORM+DB or file-backed) |
| [ ] | 5 | Startup load |
| [ ] | 5 | Create persisted before response |
| [ ] | 4 | Restart test (temp DB or temp files) |
| [ ] | 3 | Config selects memory vs persistent store |
| [ ] | 2 | `-race` clean with persistent store |
