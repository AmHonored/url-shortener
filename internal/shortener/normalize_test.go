package shortener

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"valid url unchanged", "https://go.dev/doc/", "https://go.dev/doc/"},
		{"trims surrounding spaces", "  https://go.dev/doc/  ", "https://go.dev/doc/"},
		{"lower-cases scheme and host, keeps path case", "HTTPS://GO.DEV/Doc", "https://go.dev/Doc"},
		{"empty path becomes slash", "https://go.dev", "https://go.dev/"},
		{"removes default https port", "https://go.dev:443/x", "https://go.dev/x"},
		{"removes default http port", "http://go.dev:80/x", "http://go.dev/x"},
		{"removes dangling colon", "https://go.dev:/x", "https://go.dev/x"},
		{"keeps non-default port", "http://go.dev:8080/x", "http://go.dev:8080/x"},
		{"keeps ipv6 host", "http://[::1]:80/x", "http://[::1]/x"},
		{"drops fragment", "https://go.dev/doc#top", "https://go.dev/doc"},
		{"keeps query", "https://go.dev/s?q=a&b=1", "https://go.dev/s?q=a&b=1"},
		{"trailing slash is significant", "https://go.dev/doc", "https://go.dev/doc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Normalize(tc.in)
			if err != nil {
				t.Fatalf("Normalize(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeInvalid(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"only spaces", "   "},
		{"no scheme", "go.dev/doc"},
		{"ftp scheme", "ftp://go.dev/file"},
		{"javascript scheme", "javascript:alert(1)"},
		{"missing host", "https:///path"},
		{"port without host", "http://:80/"},
		{"malformed", "http://[::1"},
		{"credentials", "https://google.com@evil.com/"},
		{"too long", "https://a.com/" + strings.Repeat("a", maxURLLength)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Normalize(tc.in)
			if !errors.Is(err, ErrInvalidURL) {
				t.Errorf("Normalize(%q) error = %v, want ErrInvalidURL", tc.in, err)
			}
		})
	}
}
