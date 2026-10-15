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
	newMux(&appState{}).ServeHTTP(rr, req)

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
	newMux(&appState{}).ServeHTTP(rr, req)

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
	old := drainDelay
	drainDelay = 0
	defer func() { drainDelay = old }()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	st := &appState{}
	srv := newServer("127.0.0.1:0", newMux(st))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, logger, srv, st) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after context cancel")
	}
	if !st.draining.Load() {
		t.Fatal("draining should be set after shutdown")
	}
}

func TestReadyzDraining(t *testing.T) {
	st := &appState{}
	st.draining.Store(true)
	mux := newMux(st)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want 503", rr.Code)
	}
	var body probeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "draining" {
		t.Fatalf("status = %q, want draining", body.Status)
	}

	// Liveness must stay green while draining so the pod is not restarted.
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200 while draining", rr.Code)
	}
}

func TestRunReturnsListenError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := newServer("127.0.0.1:-1", newMux(&appState{}))

	if err := run(context.Background(), logger, srv, &appState{}); err == nil {
		t.Fatal("run() = nil, want listen error")
	}
}

func TestRunWaitsDrainDelayBeforeShutdown(t *testing.T) {
	old := drainDelay
	drainDelay = 150 * time.Millisecond
	defer func() { drainDelay = old }()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	st := &appState{}
	srv := newServer("127.0.0.1:0", newMux(st))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, logger, srv, st) }()

	// Let ListenAndServe bind before we cancel.
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after context cancel")
	}

	elapsed := time.Since(start)
	if elapsed < drainDelay {
		t.Fatalf("shutdown finished in %v, want at least drainDelay %v", elapsed, drainDelay)
	}
	if !st.draining.Load() {
		t.Fatal("draining should be set after shutdown")
	}
}
