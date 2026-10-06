package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/AmHonored/url-shortener/internal/shortener"
	"github.com/AmHonored/url-shortener/internal/store/memory"
)

const testBase = "http://sho.rt"

func newTestHandler() http.Handler {
	return New(shortener.NewService(memory.New()), testBase)
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func doShorten(h http.Handler, url string) (shortenResponse, error) {
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(fmt.Sprintf(`{"url":%q}`, url)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		return shortenResponse{}, fmt.Errorf("POST %q: status = %d, want 201; body: %s", url, rec.Code, rec.Body)
	}
	var resp shortenResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	return resp, err
}

func shorten(t *testing.T, h http.Handler, url string) shortenResponse {
	t.Helper()
	resp, err := doShorten(h, url)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestShortenThenRedirect(t *testing.T) {
	h := newTestHandler()

	resp := shorten(t, h, "https://go.dev/doc/")
	if len(resp.Code) != shortener.CodeLen || !strings.HasPrefix(resp.Code, shortener.CodePrefix) {
		t.Errorf("code = %q, want %d chars starting with %q", resp.Code, shortener.CodeLen, shortener.CodePrefix)
	}
	if want := testBase + "/" + resp.Code; resp.ShortURL != want {
		t.Errorf("short_url = %q, want %q", resp.ShortURL, want)
	}

	rec := get(h, "/"+resp.Code)
	if rec.Code != http.StatusFound {
		t.Fatalf("GET /%s: status = %d, want 302", resp.Code, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://go.dev/doc/" {
		t.Errorf("Location = %q, want https://go.dev/doc/", loc)
	}
}

func TestShortenSameURLSameCode(t *testing.T) {
	h := newTestHandler()

	first := shorten(t, h, "https://go.dev/doc/")
	second := shorten(t, h, "https://go.dev/doc/")
	equivalent := shorten(t, h, "HTTPS://Go.Dev/doc/#top")

	if second != first || equivalent != first {
		t.Errorf("responses differ: %+v, %+v, %+v", first, second, equivalent)
	}
}

func TestShortenBaseURLTrailingSlash(t *testing.T) {
	h := New(shortener.NewService(memory.New()), testBase+"/")
	resp := shorten(t, h, "https://go.dev/")
	if want := testBase + "/" + resp.Code; resp.ShortURL != want {
		t.Errorf("short_url = %q, want %q", resp.ShortURL, want)
	}
}

func TestShortenBadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"not json", "not json"},
		{"url not a string", `{"url":123}`},
		{"missing url", `{}`},
		{"empty url", `{"url":""}`},
		{"blank url", `{"url":"   "}`},
		{"no scheme", `{"url":"go.dev/doc"}`},
		{"ftp scheme", `{"url":"ftp://go.dev/file"}`},
		{"javascript scheme", `{"url":"javascript:alert(1)"}`},
		{"no host", `{"url":"https:///x"}`},
	}
	h := newTestHandler()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := post(t, h, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
			}
			var resp errorResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil || resp.Error == "" {
				t.Errorf("want JSON error body, got %q (decode err: %v)", rec.Body, err)
			}
		})
	}
}

func TestRedirectUnknownCode(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"well-formed but unknown", "/sgZZZZZZ"},
		{"short garbage", "/x"},
		{"wrong prefix", "/abcdefgh"},
	}
	h := newTestHandler()
	shorten(t, h, "https://go.dev/") // store is not empty
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if rec := get(h, tc.path); rec.Code != http.StatusNotFound {
				t.Errorf("GET %s: status = %d, want 404", tc.path, rec.Code)
			}
		})
	}
}

type failingStore struct{}

var errDB = errors.New("database down")

func (failingStore) Create(context.Context, shortener.Link) (shortener.Link, error) {
	return shortener.Link{}, errDB
}
func (failingStore) Get(context.Context, string) (shortener.Link, error) {
	return shortener.Link{}, errDB
}

func TestInternalErrorIs500(t *testing.T) {
	h := New(shortener.NewService(failingStore{}), testBase)

	if rec := post(t, h, `{"url":"https://go.dev/"}`); rec.Code != http.StatusInternalServerError {
		t.Errorf("POST: status = %d, want 500", rec.Code)
	}
	if rec := get(h, "/sgAAAAAA"); rec.Code != http.StatusInternalServerError {
		t.Errorf("GET: status = %d, want 500", rec.Code)
	}
}

func TestLookup(t *testing.T) {
	h := newTestHandler()
	resp := shorten(t, h, "https://go.dev/doc/")

	rec := get(h, "/api/v1/links/"+resp.Code)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var lr linkResponse
	if err := json.NewDecoder(rec.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if lr.URL != "https://go.dev/doc/" {
		t.Errorf("url = %q, want https://go.dev/doc/", lr.URL)
	}
	if lr.CreatedAt == "" {
		t.Error("created_at is empty")
	}
}

func TestLookupNotFound(t *testing.T) {
	h := newTestHandler()
	rec := get(h, "/api/v1/links/sgZZZZZZ")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestConcurrentShorten runs many parallel requests through the full HTTP stack.
func TestConcurrentShorten(t *testing.T) {
	h := newTestHandler()
	const n = 50

	var wg sync.WaitGroup
	sameCodes := make([]string, n)
	distinctCodes := make([]string, n)
	for i := range n {
		wg.Add(2)
		go func() { // duplicate shorten of the same URL
			defer wg.Done()
			resp, err := doShorten(h, "https://go.dev/same")
			if err != nil {
				t.Error(err)
			}
			sameCodes[i] = resp.Code
		}()
		go func() { // a different URL per goroutine
			defer wg.Done()
			resp, err := doShorten(h, fmt.Sprintf("https://go.dev/%d", i))
			if err != nil {
				t.Error(err)
			}
			distinctCodes[i] = resp.Code
		}()
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	for i, c := range sameCodes {
		if c != sameCodes[0] {
			t.Fatalf("duplicate URL: request %d got %q, request 0 got %q", i, c, sameCodes[0])
		}
	}
	seen := map[string]bool{sameCodes[0]: true}
	for i, c := range distinctCodes {
		if seen[c] {
			t.Fatalf("distinct URL %d reused code %q", i, c)
		}
		seen[c] = true
		if loc := get(h, "/"+c).Header().Get("Location"); loc != fmt.Sprintf("https://go.dev/%d", i) {
			t.Errorf("code %q redirects to %q", c, loc)
		}
	}
}
