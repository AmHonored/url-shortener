package shortener

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const maxCodeAttempts = 5

// Store persists links. Defined here so storage packages depend on the domain.
type Store interface {
	Create(ctx context.Context, link Link) (Link, error)
	Get(ctx context.Context, code string) (Link, error)
}

// Service implements the shorten and resolve use cases.
type Service struct {
	store   Store
	newCode func() (string, error)
}

// NewService returns a Service backed by store.
func NewService(store Store) *Service {
	return &Service{store: store, newCode: NewCode}
}

// Shorten normalizes rawURL, creates a link (idempotent), and retries on code collision.
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
			continue
		}
		if err != nil {
			return Link{}, fmt.Errorf("create link: %w", err)
		}
		return link, nil
	}
	return Link{}, fmt.Errorf("no free code after %d attempts: %w", maxCodeAttempts, ErrCodeExists)
}

// Resolve returns the link for code, or ErrNotFound.
func (s *Service) Resolve(ctx context.Context, code string) (Link, error) {
	return s.store.Get(ctx, code)
}
