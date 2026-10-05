// Command server runs the URL shortener HTTP service.
//
//	go run ./cmd/server -addr :8080 -base http://localhost:8080
package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"

	"github.com/AmHonored/url-shortener/internal/httpapi"
	"github.com/AmHonored/url-shortener/internal/shortener"
	"github.com/AmHonored/url-shortener/internal/store/memory"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	base := flag.String("base", "http://localhost:8080", "public base URL used to build short_url")
	flag.Parse()

	if u, err := url.Parse(*base); err != nil || u.Scheme == "" || u.Host == "" {
		log.Fatalf("invalid -base %q: want an absolute URL like http://localhost:8080", *base)
	}

	svc := shortener.NewService(memory.New())
	handler := httpapi.New(svc, *base)

	log.Printf("listening on %s (base %s)", *addr, *base)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
