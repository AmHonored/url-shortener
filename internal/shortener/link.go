// Package shortener holds the domain model and business rules of the URL
// shortener: URL normalization, short-code generation and the Shorten /
// Resolve use cases. It knows nothing about HTTP or about concrete storage.
package shortener

import (
	"errors"
	"time"
)

// Link maps a short code to the (normalized) long URL it redirects to.
type Link struct {
	Code      string
	URL       string
	CreatedAt time.Time
}

var (
	// ErrInvalidURL means the long URL failed validation (HTTP 400).
	ErrInvalidURL = errors.New("invalid url")
	// ErrNotFound means no link exists for the requested code (HTTP 404).
	ErrNotFound = errors.New("link not found")
	// ErrCodeExists is returned by a Store when the code is already used by
	// a different URL. The service reacts by retrying with a new code.
	ErrCodeExists = errors.New("code already exists")
)
