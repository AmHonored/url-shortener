// Package file provides a JSON-file-backed link store.
// Data survives restarts; the file is rewritten on every Create.
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/AmHonored/url-shortener/internal/shortener"
)

// Store persists links to a JSON file. It keeps the same in-memory
// maps as memory.Store for fast reads and writes the full state to
// disk on every Create so data is durable before the response.
type Store struct {
	mu     sync.RWMutex
	path   string
	byCode map[string]shortener.Link
	byURL  map[string]string
}

// New loads existing data from path (creating the file if absent)
// and returns a ready-to-use Store.
func New(path string) (*Store, error) {
	s := &Store{
		path:   path,
		byCode: make(map[string]shortener.Link),
		byURL:  make(map[string]string),
	}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("file store: %w", err)
	}
	return s, nil
}

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

	if err := s.save(); err != nil {
		delete(s.byCode, link.Code)
		delete(s.byURL, link.URL)
		return shortener.Link{}, fmt.Errorf("persist: %w", err)
	}
	return link, nil
}

func (s *Store) Get(_ context.Context, code string) (shortener.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.byCode[code]
	if !ok {
		return shortener.Link{}, fmt.Errorf("code %q: %w", code, shortener.ErrNotFound)
	}
	return link, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var links []shortener.Link
	if err := json.Unmarshal(data, &links); err != nil {
		return err
	}
	for _, l := range links {
		s.byCode[l.Code] = l
		s.byURL[l.URL] = l.Code
	}
	return nil
}

func (s *Store) save() error {
	links := make([]shortener.Link, 0, len(s.byCode))
	for _, l := range s.byCode {
		links = append(links, l)
	}
	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
