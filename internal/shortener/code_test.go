package shortener

import (
	"strings"
	"testing"
)

func TestNewCodeFormat(t *testing.T) {
	for range 1000 {
		code, err := NewCode()
		if err != nil {
			t.Fatalf("NewCode() error: %v", err)
		}
		if len(code) != CodeLen {
			t.Fatalf("len(%q) = %d, want %d", code, len(code), CodeLen)
		}
		if !strings.HasPrefix(code, CodePrefix) {
			t.Fatalf("code %q does not start with %q", code, CodePrefix)
		}
		for _, r := range code {
			if !strings.ContainsRune(base62, r) {
				t.Fatalf("code %q contains non-base62 rune %q", code, r)
			}
		}
	}
}

func TestNewCodeIsRandom(t *testing.T) {
	const n = 1000
	seen := make(map[string]bool, n)
	for range n {
		code, err := NewCode()
		if err != nil {
			t.Fatalf("NewCode() error: %v", err)
		}
		if seen[code] {
			// Birthday bound: n²/(2·62^6) ≈ 1e-5, i.e. ~1 failing run in 100k.
			t.Fatalf("duplicate code %q after %d draws", code, len(seen))
		}
		seen[code] = true
	}
}
