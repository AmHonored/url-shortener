package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AmHonored/url-shortener/internal/shortener"
	"github.com/AmHonored/url-shortener/internal/store/memory"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3)
	for i := range 3 {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("4th request should be denied")
	}
	// Different IP is independent.
	if !rl.Allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}

func TestRateLimitOnShorten(t *testing.T) {
	h := New(shortener.NewService(memory.New()), testBase, 2)
	body := `{"url":"https://go.dev/"}`

	for i := range 2 {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("request %d: status = %d, want 201", i+1, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: status = %d, want 429", rec.Code)
	}
}
