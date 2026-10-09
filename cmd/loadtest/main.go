// Command loadtest hammers the shortener with concurrent requests and prints RPS.
//
//	go run ./cmd/loadtest -addr http://localhost:8080 -n 1000 -c 20
//  Note: run the server with -rate 0 to prevent 429s from skewing the results
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	addr := flag.String("addr", "http://localhost:8080", "server base URL")
	n := flag.Int("n", 1000, "total requests per phase")
	c := flag.Int("c", 10, "concurrency (goroutines)")
	flag.Parse()

	fmt.Printf("load-testing %s — %d requests, concurrency %d\n\n", *addr, *n, *c)

	runPhase("POST /api/shorten (same URL, idempotent)", *n, *c, func() bool {
		body := strings.NewReader(`{"url":"https://go.dev/doc/"}`)
		resp, err := http.Post(*addr+"/api/shorten", "application/json", body)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusCreated
	})

	// Shorten once to get a real code for redirect benchmarking.
	resp, err := http.Post(*addr+"/api/shorten", "application/json",
		strings.NewReader(`{"url":"https://go.dev/"}`))
	if err != nil {
		fmt.Println("could not shorten seed URL:", err)
		return
	}
	defer resp.Body.Close()
	
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result struct {
		Code string `json:"code"`
	}
	json.Unmarshal(bodyBytes, &result)

	if result.Code == "" {
		fmt.Println("could not extract short code from response")
		return
	}

	runPhase("GET /{code} (redirect, hot code)", *n, *c, func() bool {
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		resp, err := client.Get(*addr + "/" + result.Code)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusFound
	})
}

func runPhase(name string, n, c int, fn func() bool) {
	var ok, fail int64
	jobs := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		jobs <- struct{}{}
	}
	close(jobs)

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < c; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				if fn() {
					atomic.AddInt64(&ok, 1)
				} else {
					atomic.AddInt64(&fail, 1)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	rps := 0.0
	if ok > 0 {
		rps = float64(ok) / elapsed.Seconds()
	}
	fmt.Printf("%-45s  %6.0f req/s  ok=%-5d fail=%d\n", name, rps, ok, fail)
}
