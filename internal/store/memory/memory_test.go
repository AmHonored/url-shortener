package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AmHonored/url-shortener/internal/shortener"
)

func link(code, url string) shortener.Link {
	return shortener.Link{Code: code, URL: url, CreatedAt: time.Now().UTC()}
}

func TestCreateAndGet(t *testing.T) {
	s := New()
	ctx := context.Background()

	want := link("sgAAAAAA", "https://go.dev/")
	got, err := s.Create(ctx, want)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got != want {
		t.Errorf("Create returned %+v, want %+v", got, want)
	}

	got, err = s.Get(ctx, "sgAAAAAA")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != want {
		t.Errorf("Get returned %+v, want %+v", got, want)
	}
}

func TestCreateSameURLReturnsExisting(t *testing.T) {
	s := New()
	ctx := context.Background()

	first, _ := s.Create(ctx, link("sgAAAAAA", "https://go.dev/"))
	second, err := s.Create(ctx, link("sgBBBBBB", "https://go.dev/"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if second != first {
		t.Errorf("second Create = %+v, want existing %+v", second, first)
	}
	if _, err := s.Get(ctx, "sgBBBBBB"); !errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("discarded code should not be stored, Get err = %v", err)
	}
}

func TestCreateCodeCollision(t *testing.T) {
	s := New()
	ctx := context.Background()

	_, _ = s.Create(ctx, link("sgAAAAAA", "https://go.dev/"))
	_, err := s.Create(ctx, link("sgAAAAAA", "https://example.com/"))
	if !errors.Is(err, shortener.ErrCodeExists) {
		t.Fatalf("Create with taken code: err = %v, want ErrCodeExists", err)
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := New().Get(context.Background(), "sgZZZZZZ")
	if !errors.Is(err, shortener.ErrNotFound) {
		t.Fatalf("Get unknown: err = %v, want ErrNotFound", err)
	}
}

func TestConcurrentCreateSameURL(t *testing.T) {
	s := New()
	ctx := context.Background()
	const n = 100

	codes := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := s.Create(ctx, link(fmt.Sprintf("sg%06d", i), "https://go.dev/"))
			if err != nil {
				t.Errorf("Create: %v", err)
				return
			}
			codes[i] = l.Code
			_, _ = s.Get(ctx, l.Code) // concurrent reads alongside writes
		}()
	}
	wg.Wait()

	for i, c := range codes {
		if c != codes[0] {
			t.Fatalf("goroutine %d got code %q, goroutine 0 got %q", i, c, codes[0])
		}
	}
}
