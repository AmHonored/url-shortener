package shortener

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// maxCodeAttempts bounds retries when a freshly generated code is already
// taken. With 62^6 codes a single retry is already astronomically rare.
const maxCodeAttempts = 5

// Store persists links. It is defined here, at the consumer, so storage
// packages depend on the domain and not the other way round.
type Store interface {
	// Create saves link. If link.URL already exists it returns the existing
	// link (idempotent). If link.Code is used by another URL it returns an
	// error wrapping ErrCodeExists.
	Create(ctx context.Context, link Link) (Link, error)
	// Get returns the link for code or an error wrapping ErrNotFound.
	Get(ctx context.Context, code string) (Link, error)
}

// Service implements the shorten and resolve use cases.
type Service struct {
	store   Store
	newCode func() (string, error) // swappable in tests to force collisions
}

// NewService returns a Service backed by store.
func NewService(store Store) *Service {
	return &Service{store: store, newCode: NewCode}
}

// Shorten returns the link for rawURL, creating it on first use.
// The same URL (after Normalize) always yields the same code; a new code is
// generated only for URLs not seen before, and regenerated on collision.
func (s *Service) Shorten(ctx context.Context, rawURL string) (Link, error) {
	normalized, err := Normalize(rawURL)
	if err != nil {
		return Link{}, err
	}

	for range maxCodeAttempts {
		code, err := s.newCode()
		if err != nil {
			return Link{}, err
		}
		link, err := s.store.Create(ctx, Link{Code: code, URL: normalized, CreatedAt: time.Now().UTC()})
		if errors.Is(err, ErrCodeExists) {
			continue // collision with a different URL: try another code
		}
		if err != nil {
			return Link{}, fmt.Errorf("create link: %w", err)
		}
		return link, nil
	}
	return Link{}, fmt.Errorf("no free code after %d attempts: %w", maxCodeAttempts, ErrCodeExists)
}

// Resolve returns the link for code, or an error wrapping ErrNotFound.
func (s *Service) Resolve(ctx context.Context, code string) (Link, error) {
	return s.store.Get(ctx, code)
}
