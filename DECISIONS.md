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
