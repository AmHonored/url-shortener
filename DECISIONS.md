# Design decisions

## Dependencies

None. Only the Go standard library is used (`net/http` routing from Go 1.22).

## AI usage

The code in this repository was written with an AI coding assistant
(Antigravity), directed and reviewed by me. Design choices were discussed
with the assistant before implementation.

## Part 1

### Package layout
- `cmd/server`: `main` only parses flags and wires the pieces together.
- `internal/shortener`: the domain. It holds `Link`, the sentinel errors,
  URL normalization, code generation, the `Service` (shorten/resolve
  use cases) and the `Store` interface. It imports nothing from HTTP or storage.
- `internal/store/memory`: the in-memory `Store` implementation.
- `internal/httpapi`: HTTP handlers, JSON encoding, and mapping errors to status codes.
- Dependency direction: `httpapi → shortener ← store/memory`. Part 4 can
  add a persistent store next to `memory` without changing the domain or HTTP code.
- I used packages named after what they provide instead of a `db/repo/table`
  plus `service` layering. This is the more common Go style. The "service"
  is `shortener.Service`, the "repo" is `store/memory`.

### When are two URLs "the same"? (normalization)
`shortener.Normalize` builds a canonical string that is used as the identity key:
- Surrounding whitespace is trimmed.
- Scheme and host are lower-cased (both are case-insensitive by RFC 3986).
- The default port is removed (`:80` for http, `:443` for https), as is a dangling `:`.
- An empty path becomes `/` (`https://go.dev` ≡ `https://go.dev/`).
- The `#fragment` is dropped, because browsers never send it to the server.
- **Not** changed: path case, trailing slash on non-root paths, query string
  and parameter order. The target server may treat these differently, so
  merging them could send users to the wrong page.
- The stored and redirected URL is the normalized one.

### URL validation
- Must be non-empty and at most 2048 characters.
- Scheme must be `http` or `https`, and the host must be non-empty.
- Credentials (`user:pass@host`) are rejected. `https://google.com@evil.com`
  really goes to `evil.com`, which is a common phishing trick.
- The URL is only parsed and **never fetched** (no SSRF).
- Error messages never echo the submitted URL, so secrets in query strings
  are not leaked.

### How "same URL → same code" is stored
The store keeps two maps:
- `byCode: code → Link`, used for redirects.
- `byURL: normalized URL → code`, the reverse index used for idempotency.

`Store.Create(link)` returns the **existing** link if `link.URL` is already
in `byURL`, and the freshly generated code is discarded. Every POST of the
same URL therefore gets the same code and `short_url`, with status 201.

### How codes are generated (new URLs only)
- Format: the fixed tag `sg` (short for "sysgrp") + 6 random base62
  characters `[0-9a-zA-Z]` = 8 characters, within the required 6–8.
- Randomness comes from `crypto/rand`. Bytes ≥ 248 are skipped (rejection
  sampling), so `byte % 62` stays uniform with no modulo bias.
- Random rather than counter-based: codes can't be guessed or enumerated,
  and no shared counter is needed when scaling to many instances (Part 5).
- The service generates a candidate code for each Create call. If the URL
  already exists, the store returns the existing link and the candidate is
  thrown away. **Only new URLs ever receive a new code.**

### Collision handling
- 62⁶ ≈ 56.8 billion possible codes, so collisions are extremely rare.
- If the store reports `ErrCodeExists` (code taken by a *different* URL),
  `Service.Shorten` generates a new code and retries, up to **5 attempts**.
  After that it returns an error that maps to HTTP 500.
- Distinct URLs can never share a code. The store checks `byCode` before
  inserting, inside the same lock.

### Mutex type and which methods lock
- One `sync.RWMutex` per `memory.Store` guards both maps together, so they
  can never go out of sync.
- `Get` takes `RLock`. Redirects are the hot path and can run in parallel.
- `Create` takes `Lock`. The "URL exists?", "code exists?" and "insert"
  steps happen in **one** critical section. Two concurrent POSTs for the
  same URL therefore cannot both miss and create two codes. This is covered
  by concurrent tests at the store and HTTP levels.
- The service and handlers hold no shared mutable state, so they need no locks.

### HTTP
- Routing uses the Go 1.22 `ServeMux` patterns: `POST /api/shorten` and
  `GET /{code}` (which also answers `HEAD`, so `curl -I` works).
- Request bodies are limited to 1 MB with `http.MaxBytesReader`.
- `short_url` = the `-base` flag (trailing `/` trimmed) + `/` + code.
- The `-base` flag is validated at startup and must be an absolute URL.
