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
	base := flag.String("base", "http://localhost:8080", "public base URL for short_url")
	flag.Parse()

	if u, err := url.Parse(*base); err != nil || u.Scheme == "" || u.Host == "" {
		log.Fatalf("invalid -base %q", *base)
	}

	svc := shortener.NewService(memory.New())
	log.Printf("listening on %s (base %s)", *addr, *base)
	log.Fatal(http.ListenAndServe(*addr, httpapi.New(svc, *base)))
}
