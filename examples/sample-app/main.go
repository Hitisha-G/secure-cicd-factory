package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
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

// appState holds process-wide lifecycle flags shared by handlers and run.
type appState struct {
	// draining flips to true once shutdown starts so readiness fails and
	// load balancers stop routing new traffic before the listener closes.
	draining atomic.Bool
}

// newMux wires the HTTP routes so they can be exercised without a live server.
func newMux(st *appState) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/readyz", readyz(st))
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

func writeProbe(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(probeResponse{Status: status, Version: version})
}

// healthz is the liveness probe: process is up and serving.
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeProbe(w, http.StatusOK, "ok")
}

// readyz is the readiness probe: process can accept traffic. It returns 503
// with status "draining" once shutdown has begun, while liveness stays green
// so the orchestrator does not restart a pod that is exiting cleanly.
func readyz(st *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if st.draining.Load() {
			writeProbe(w, http.StatusServiceUnavailable, "draining")
			return
		}
		writeProbe(w, http.StatusOK, "ready")
	}
}

// newLogger emits structured JSON logs to stdout, which suits container log collectors.
func newLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

// shutdownTimeout bounds how long in-flight requests get to drain on shutdown.
const shutdownTimeout = 10 * time.Second

// run serves until ctx is cancelled or the listener fails, then drains
// in-flight requests. Keeping this out of main makes shutdown testable.
func run(ctx context.Context, logger *slog.Logger, srv *http.Server, st *appState) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	st.draining.Store(true)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped")
	return nil
}

func main() {
	logger := newLogger()
	st := &appState{}
	srv := newServer(listenAddr(), newMux(st))
	logger.Info("starting sample-app", "version", version, "addr", srv.Addr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, srv, st); err != nil {
		logger.Error("server stopped", "err", err)
		stop()
		os.Exit(1)
	}
}
