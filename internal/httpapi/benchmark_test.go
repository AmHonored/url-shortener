package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AmHonored/url-shortener/internal/shortener"
	"github.com/AmHonored/url-shortener/internal/store/memory"
)

func BenchmarkShorten(b *testing.B) {
	h := New(shortener.NewService(memory.New()), testBase)
	body := `{"url":"https://go.dev/doc/"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			b.Fatalf("status = %d", rec.Code)
		}
	}
}

func BenchmarkShortenDistinct(b *testing.B) {
	h := New(shortener.NewService(memory.New()), testBase)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := fmt.Sprintf(`{"url":"https://go.dev/%d"}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			b.Fatalf("status = %d", rec.Code)
		}
	}
}

func BenchmarkRedirect(b *testing.B) {
	h := New(shortener.NewService(memory.New()), testBase)
	resp, _ := doShorten(h, "https://go.dev/doc/")
	path := "/" + resp.Code

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			b.Fatalf("status = %d", rec.Code)
		}
	}
}

func BenchmarkLookup(b *testing.B) {
	h := New(shortener.NewService(memory.New()), testBase)
	resp, _ := doShorten(h, "https://go.dev/doc/")
	path := "/api/v1/links/" + resp.Code

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d", rec.Code)
		}
	}
}
