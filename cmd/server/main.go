package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AmHonored/url-shortener/internal/httpapi"
	"github.com/AmHonored/url-shortener/internal/shortener"
	"github.com/AmHonored/url-shortener/internal/store/file"
	"github.com/AmHonored/url-shortener/internal/store/memory"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	base := flag.String("base", "http://localhost:8080", "public base URL for short_url")
	storePath := flag.String("store", "", "path to JSON file for persistent storage (empty = in-memory)")
	rateLimit := flag.Int("rate", 10, "max POST /api/shorten requests per IP per minute (0 = unlimited)")
	flag.Parse()

	if u, err := url.Parse(*base); err != nil || u.Scheme == "" || u.Host == "" {
		log.Fatalf("invalid -base %q", *base)
	}

	var store shortener.Store
	if *storePath != "" {
		var err error
		store, err = file.New(*storePath)
		if err != nil {
			log.Fatalf("file store: %v", err)
		}
		log.Printf("using file store: %s", *storePath)
	} else {
		store = memory.New()
		log.Print("using in-memory store")
	}

	svc := shortener.NewService(store)
	handler := httpapi.New(svc, *base, *rateLimit)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in background.
	go func() {
		log.Printf("listening on %s (base %s)", *addr, *base)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for interrupt signal, then drain in-flight requests.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("received %v, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Print("server stopped")
}
