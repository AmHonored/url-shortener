// Package httpapi exposes the shortener over HTTP using net/http routing.
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/AmHonored/url-shortener/internal/shortener"
)

// maxBodyBytes caps request bodies; a URL is at most 2 KB, so 1 MB is generous.
const maxBodyBytes = 1 << 20

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type handler struct {
	svc     *shortener.Service
	baseURL string // without trailing slash
}

// New returns the HTTP handler with all routes registered.
// baseURL (e.g. "http://localhost:8080") is used to build short_url.
func New(svc *shortener.Service, baseURL string) http.Handler {
	h := &handler{svc: svc, baseURL: strings.TrimRight(baseURL, "/")}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/shorten", h.shorten)
	mux.HandleFunc("GET /{code}", h.redirect) // GET patterns also match HEAD
	return mux
}

// shorten handles POST /api/shorten: {"url": "..."} → 201 {"code","short_url"}.
// Repeating the same URL returns the same code, still with 201.
func (h *handler) shorten(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, `request body must be JSON: {"url":"https://..."}`)
		return
	}

	link, err := h.svc.Shorten(r.Context(), req.URL)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, shortenResponse{
		Code:     link.Code,
		ShortURL: h.baseURL + "/" + link.Code,
	})
}

// redirect handles GET /{code}: 302 to the long URL, or 404.
func (h *handler) redirect(w http.ResponseWriter, r *http.Request) {
	link, err := h.svc.Resolve(r.Context(), r.PathValue("code"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	http.Redirect(w, r, link.URL, http.StatusFound)
}

// writeServiceError maps domain errors to HTTP status codes with errors.Is,
// so wrapped errors are matched too. Unknown errors become 500 and are logged
// without exposing details to the client.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, shortener.ErrInvalidURL):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, shortener.ErrNotFound):
		writeError(w, http.StatusNotFound, "short code not found")
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}
