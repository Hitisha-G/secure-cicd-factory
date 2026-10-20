package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
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

// freeAddr reserves a loopback port, releases it, and returns the address so
// run can bind it via ListenAndServe while the test still knows where to dial.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return addr
}

// noKeepAliveClient opens a fresh connection per request so no idle
// keep-alive connection lingers and stretches out graceful shutdown.
func noKeepAliveClient() *http.Client {
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
}

// waitForServer polls /healthz until the listener accepts connections.
func waitForServer(t *testing.T, client *http.Client, base string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/healthz")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server did not start listening in time")
}

func TestRunDrainsInFlightRequests(t *testing.T) {
	old := drainDelay
	drainDelay = 0
	defer func() { drainDelay = old }()

	const slowFor = 200 * time.Millisecond
	started := make(chan struct{})
	st := &appState{}
	mux := newMux(st)
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(slowFor)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "done")
	})

	addr := freeAddr(t)
	base := "http://" + addr
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := newServer(addr, mux)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- run(ctx, logger, srv, st) }()
	client := noKeepAliveClient()
	waitForServer(t, client, base)

	type result struct {
		code int
		body string
		err  error
	}
	reqDone := make(chan result, 1)
	go func() {
		resp, err := client.Get(base + "/slow")
		if err != nil {
			reqDone <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		reqDone <- result{code: resp.StatusCode, body: string(b), err: err}
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("slow handler never started")
	}
	cancel()

	var res result
	select {
	case res = <-reqDone:
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request did not complete")
	}
	if res.err != nil {
		t.Fatalf("in-flight request failed during shutdown: %v", res.err)
	}
	if res.code != http.StatusOK || res.body != "done" {
		t.Fatalf("in-flight response = %d %q, want 200 \"done\"", res.code, res.body)
	}

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("run() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after draining")
	}

	// The listener is closed once run returns, so new connections must fail.
	if resp, err := client.Get(base + "/healthz"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected new connections to be refused after shutdown")
	}
}

func TestNewServerTimeouts(t *testing.T) {
	srv := newServer(":0", http.NewServeMux())
	checks := []struct {
		name string
		got  time.Duration
	}{
		{"ReadHeaderTimeout", srv.ReadHeaderTimeout},
		{"ReadTimeout", srv.ReadTimeout},
		{"WriteTimeout", srv.WriteTimeout},
		{"IdleTimeout", srv.IdleTimeout},
	}
	for _, c := range checks {
		if c.got <= 0 {
			t.Errorf("%s = %v, want a positive timeout to guard against slow clients", c.name, c.got)
		}
	}
	if srv.ReadHeaderTimeout > srv.ReadTimeout {
		t.Errorf("ReadHeaderTimeout %v should not exceed ReadTimeout %v", srv.ReadHeaderTimeout, srv.ReadTimeout)
	}
	if srv.WriteTimeout > shutdownTimeout {
		t.Errorf("WriteTimeout %v exceeds shutdownTimeout %v; in-flight requests could outlive the drain window", srv.WriteTimeout, shutdownTimeout)
	}
}

func TestProbesReportBuildVersion(t *testing.T) {
	old := version
	version = "1.2.3-test"
	defer func() { version = old }()

	draining := &appState{}
	draining.draining.Store(true)

	cases := []struct {
		name   string
		st     *appState
		path   string
		code   int
		status string
	}{
		{"healthz", &appState{}, "/healthz", http.StatusOK, "ok"},
		{"readyz", &appState{}, "/readyz", http.StatusOK, "ready"},
		{"readyz draining", draining, "/readyz", http.StatusServiceUnavailable, "draining"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			newMux(tc.st).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if rr.Code != tc.code {
				t.Fatalf("status code = %d, want %d", rr.Code, tc.code)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
			var body probeResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Status != tc.status {
				t.Fatalf("status = %q, want %q", body.Status, tc.status)
			}
			if body.Version != "1.2.3-test" {
				t.Fatalf("version = %q, want the -ldflags injected value", body.Version)
			}
		})
	}
}

func TestUnknownRouteNotFound(t *testing.T) {
	for _, path := range []string{"/", "/metrics", "/healthz/extra"} {
		rr := httptest.NewRecorder()
		newMux(&appState{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, rr.Code)
		}
	}
}
