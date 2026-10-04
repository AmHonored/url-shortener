// Package memory provides an in-memory, concurrency-safe link store.
// It satisfies shortener.Store; data is lost when the process exits.
package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/AmHonored/url-shortener/internal/shortener"
)

// Store keeps links in two maps guarded by one RWMutex:
//
//	byCode: code → Link        (redirect / lookup path)
//	byURL:  normalized URL → code  (idempotency index: same URL → same code)
//
// Reads (Get) take the shared lock so redirects run in parallel; Create takes
// the exclusive lock because it must check and insert atomically.
type Store struct {
	mu     sync.RWMutex
	byCode map[string]shortener.Link
	byURL  map[string]string
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		byCode: make(map[string]shortener.Link),
		byURL:  make(map[string]string),
	}
}

// Create saves link and returns the stored link.
//   - If link.URL is already stored, the existing link is returned unchanged
//     (idempotent: the new code is discarded).
//   - If link.Code is used by a different URL, it returns shortener.ErrCodeExists.
//
// Both checks and the insert happen under one write lock, so concurrent calls
// for the same URL can never end up with two different codes.
func (s *Store) Create(_ context.Context, link shortener.Link) (shortener.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if code, ok := s.byURL[link.URL]; ok {
		return s.byCode[code], nil
	}
	if _, ok := s.byCode[link.Code]; ok {
		return shortener.Link{}, fmt.Errorf("code %q: %w", link.Code, shortener.ErrCodeExists)
	}
	s.byCode[link.Code] = link
	s.byURL[link.URL] = link.Code
	return link, nil
}

// Get returns the link for code, or an error wrapping shortener.ErrNotFound.
func (s *Store) Get(_ context.Context, code string) (shortener.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.byCode[code]
	if !ok {
		return shortener.Link{}, fmt.Errorf("code %q: %w", code, shortener.ErrNotFound)
	}
	return link, nil
}
