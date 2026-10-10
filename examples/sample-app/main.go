package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	mux.HandleFunc("/readyz", readyz)
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

// probeResponse is the JSON body returned by liveness and readiness probes.
type probeResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func writeProbe(w http.ResponseWriter, status string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(probeResponse{Status: status, Version: version})
}

// healthz is the liveness probe: process is up and serving.
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeProbe(w, "ok")
}

// readyz is the readiness probe: process can accept traffic.
// Today that matches liveness; extend here when dependencies are added.
func readyz(w http.ResponseWriter, _ *http.Request) {
	writeProbe(w, "ready")
}

// newLogger emits structured JSON logs to stdout, which suits container log collectors.
func newLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func main() {
	logger := newLogger()
	srv := newServer(listenAddr(), newMux())
	logger.Info("starting sample-app", "version", version, "addr", srv.Addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "err", err)
			os.Exit(1)
		}
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "err", err)
			os.Exit(1)
		}
		logger.Info("server stopped")
	}
}
