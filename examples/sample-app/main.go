package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

var version = "dev"

const defaultPort = "8080"

// listenAddr returns the address to bind, honouring the PORT env var.
func listenAddr() string {
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return ":" + defaultPort
}

// newMux wires the HTTP routes so they can be exercised without a live server.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	return mux
}

// newServer builds the HTTP server with conservative timeouts.
func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version})
}

func main() {
	srv := newServer(listenAddr(), newMux())
	log.Printf("sample-app %s listening on %s", version, srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
