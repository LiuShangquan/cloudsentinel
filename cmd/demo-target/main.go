// Command demo-target is an optional Compose test fixture, not a production service.
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, "CloudSentinel MVP target is healthy")
	})
	mux.HandleFunc("/delay", func(w http.ResponseWriter, r *http.Request) {
		delay, err := time.ParseDuration(r.URL.Query().Get("duration"))
		if err != nil || delay < 0 || delay > time.Minute {
			http.Error(w, "duration must be between 0s and 1m", http.StatusBadRequest)
			return
		}
		select {
		case <-time.After(delay):
			_, _ = fmt.Fprintln(w, "delayed response completed")
		case <-r.Context().Done():
		}
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       65 * time.Second,
		WriteTimeout:      65 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("demo target listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
