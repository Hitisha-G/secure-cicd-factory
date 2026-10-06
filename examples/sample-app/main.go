package main

import (
	"encoding/json"
	"log/slog"
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

// healthResponse is the JSON body returned by /healthz.
type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok", Version: version})
}

// newLogger emits structured JSON logs to stdout, which suits container log collectors.
func newLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func main() {
	logger := newLogger()
	srv := newServer(listenAddr(), newMux())
	logger.Info("starting sample-app", "version", version, "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
