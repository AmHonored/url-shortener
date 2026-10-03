package shortener

import (
	"fmt"
	"net/url"
	"strings"
)

// maxURLLength caps accepted URLs; 2048 is the de-facto browser limit.
const maxURLLength = 2048

// defaultPorts lists the allowed schemes and their default port.
var defaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

// Normalize validates raw and returns its canonical form. Two inputs with the
// same canonical form are considered the same URL and get the same code.
//
// Rules:
//   - only http and https with a non-empty host are accepted;
//   - credentials (user:pass@) are rejected (common phishing trick);
//   - scheme and host are lower-cased, the default port is removed;
//   - an empty path becomes "/" and the #fragment is dropped;
//   - path, query and trailing slashes are kept: servers may treat them
//     differently, so changing them could change the destination.
//
// The URL is only parsed, never fetched (no SSRF).
func Normalize(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("%w: url is empty", ErrInvalidURL)
	}
	if len(s) > maxURLLength {
		return "", fmt.Errorf("%w: url is longer than %d characters", ErrInvalidURL, maxURLLength)
	}

	u, err := url.Parse(s)
	if err != nil {
		// The parse error echoes the full URL, so it is not wrapped here
		// to avoid leaking query-string secrets into responses or logs.
		return "", fmt.Errorf("%w: malformed url", ErrInvalidURL)
	}

	defaultPort, ok := defaultPorts[u.Scheme] // url.Parse already lower-cases the scheme
	if !ok {
		return "", fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("%w: host is missing", ErrInvalidURL)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: credentials in url are not allowed", ErrInvalidURL)
	}

	u.Host = strings.ToLower(u.Host)
	if port := u.Port(); port == "" || port == defaultPort {
		u.Host = strings.TrimSuffix(u.Host, ":"+port) // also drops a dangling ":"
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Fragment, u.RawFragment = "", ""

	return u.String(), nil
}
