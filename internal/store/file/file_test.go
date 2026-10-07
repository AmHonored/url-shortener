package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmHonored/url-shortener/internal/shortener"
)

func tmpPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "links.json")
}

func link(code, url string) shortener.Link {
	return shortener.Link{Code: code, URL: url, CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
}

func TestCreateAndGet(t *testing.T) {
	s, err := New(tmpPath(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	want := link("sgAAAAAA", "https://go.dev/")
	got, err := s.Create(ctx, want)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got != want {
		t.Errorf("Create = %+v, want %+v", got, want)
	}

	got, err = s.Get(ctx, "sgAAAAAA")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != want {
		t.Errorf("Get = %+v, want %+v", got, want)
	}
}

func TestIdempotent(t *testing.T) {
	s, _ := New(tmpPath(t))
	ctx := context.Background()

	first, _ := s.Create(ctx, link("sgAAAAAA", "https://go.dev/"))
	second, err := s.Create(ctx, link("sgBBBBBB", "https://go.dev/"))
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("second = %+v, want %+v", second, first)
	}
}

func TestCodeCollision(t *testing.T) {
	s, _ := New(tmpPath(t))
	ctx := context.Background()

	s.Create(ctx, link("sgAAAAAA", "https://go.dev/"))
	_, err := s.Create(ctx, link("sgAAAAAA", "https://example.com/"))
	if !errors.Is(err, shortener.ErrCodeExists) {
		t.Fatalf("err = %v, want ErrCodeExists", err)
	}
}

func TestGetNotFound(t *testing.T) {
	s, _ := New(tmpPath(t))
	_, err := s.Get(context.Background(), "sgZZZZZZ")
	if !errors.Is(err, shortener.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestRestart(t *testing.T) {
	path := tmpPath(t)
	ctx := context.Background()

	s1, _ := New(path)
	want := link("sgAAAAAA", "https://go.dev/")
	s1.Create(ctx, want)

	// Simulate restart: create a new Store from the same file.
	s2, err := New(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, err := s2.Get(ctx, "sgAAAAAA")
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if got != want {
		t.Errorf("after restart = %+v, want %+v", got, want)
	}
}

func TestNewCreatesFile(t *testing.T) {
	path := tmpPath(t)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not exist before New")
	}
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Create(context.Background(), link("sgAAAAAA", "https://go.dev/"))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist after Create: %v", err)
	}
}
