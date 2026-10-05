package shortener

import (
	"errors"
	"time"
)

// Link maps a short code to the original long URL.
type Link struct {
	Code      string
	URL       string
	CreatedAt time.Time
}

var (
	ErrInvalidURL = errors.New("invalid url")
	ErrNotFound   = errors.New("link not found")
	ErrCodeExists = errors.New("code already exists")
)
