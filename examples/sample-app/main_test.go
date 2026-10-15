package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestListenAddrDefault(t *testing.T) {
	t.Setenv("PORT", "")
	// Unset so empty env is truly absent on some platforms.
	_ = os.Unsetenv("PORT")
	if got := listenAddr(); got != ":8080" {
		t.Fatalf("listenAddr() = %q, want :8080", got)
	}
}

func TestListenAddrFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	if got := listenAddr(); got != ":9090" {
		t.Fatalf("listenAddr() = %q, want :9090", got)
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	newMux().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body probeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.Status)
	}
	if body.Version == "" {
		t.Fatal("version should be set")
	}
}

func TestReadyz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	newMux().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body probeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ready" {
		t.Fatalf("status = %q, want ready", body.Status)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := newServer("127.0.0.1:0", newMux())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, logger, srv) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after context cancel")
	}
}

func TestRunReturnsListenError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := newServer("127.0.0.1:-1", newMux())

	if err := run(context.Background(), logger, srv); err == nil {
		t.Fatal("run() = nil, want listen error")
	}
}
