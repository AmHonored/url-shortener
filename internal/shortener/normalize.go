package shortener

import (
	"fmt"
	"net/url"
	"strings"
)

const maxURLLength = 2048

var defaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

// Normalize validates raw and returns its canonical form.
// Two inputs with the same canonical form get the same short code.
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
		return "", fmt.Errorf("%w: malformed url", ErrInvalidURL)
	}

	defaultPort, ok := defaultPorts[u.Scheme]
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
		u.Host = strings.TrimSuffix(u.Host, ":"+port)
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Fragment, u.RawFragment = "", ""

	return u.String(), nil
}
