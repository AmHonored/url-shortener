// Package memory provides an in-memory link store backed by maps + sync.RWMutex.
package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/AmHonored/url-shortener/internal/shortener"
)

// Store keeps byCode and byURL maps guarded by one RWMutex.
type Store struct {
	mu     sync.RWMutex
	byCode map[string]shortener.Link
	byURL  map[string]string
}

func New() *Store {
	return &Store{
		byCode: make(map[string]shortener.Link),
		byURL:  make(map[string]string),
	}
}

// Create saves link. Same URL returns the existing link; same code with
// a different URL returns ErrCodeExists.
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

// Get returns the link for code, or ErrNotFound.
func (s *Store) Get(_ context.Context, code string) (shortener.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.byCode[code]
	if !ok {
		return shortener.Link{}, fmt.Errorf("code %q: %w", code, shortener.ErrNotFound)
	}
	return link, nil
}
