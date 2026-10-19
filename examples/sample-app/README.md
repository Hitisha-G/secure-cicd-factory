# sample-app

Minimal Go HTTP service used to exercise the pipelines in this repo.

- `GET /healthz` returns `{"status":"ok","version":"..."}` (liveness)
- `GET /readyz` returns `{"status":"ready","version":"..."}` (readiness), or
  `503 {"status":"draining",...}` once shutdown has started
- Listens on `:8080` (override with `PORT`)
- Multi-stage build into a distroless, non-root image
- Structured JSON logs to stdout (suitable for container log collectors)

```bash
docker build --build-arg VERSION=0.1.0 -t sample-app:local .
docker run --rm -p 8080:8080 sample-app:local
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
```

## Graceful shutdown

The process traps `SIGINT` and `SIGTERM`, then calls `http.Server.Shutdown` with a
10-second drain so in-flight requests can finish before the process exits. That
matches how orchestrators (Kubernetes, ECS, Docker Compose) stop containers:
they send `SIGTERM`, wait for the grace period, then force-kill.

As soon as the signal arrives, `/readyz` flips to `503 draining` while
`/healthz` stays `200`. The process then waits briefly (2s by default) so load
balancers and Kubernetes readiness checks can observe the failure and stop
sending new traffic before `Shutdown` closes the listener. The liveness probe
does not restart a pod that is already exiting cleanly.

```bash
make run          # Ctrl-C triggers the same path as SIGTERM in a container
# or
docker run --rm -p 8080:8080 sample-app:local
# another terminal: docker stop <container>  # sends SIGTERM
```

## Running on Kubernetes

The shutdown timings above set a floor for the pod's grace period. From
`SIGTERM` to exit the process spends up to `drainDelay` (2s) reporting
not-ready plus up to `shutdownTimeout` (10s) draining in-flight requests, so
`terminationGracePeriodSeconds` should be at least 15 to leave a little
headroom before the kubelet sends `SIGKILL`. The image is distroless and has
no shell, so a `preStop: exec: sleep` hook will not work; the built-in drain
delay covers that role instead.

```yaml
spec:
  terminationGracePeriodSeconds: 15   # >= drainDelay + shutdownTimeout + headroom
  containers:
    - name: sample-app
      image: sample-app:0.1.0
      ports:
        - containerPort: 8080
      securityContext:
        runAsNonRoot: true            # image already runs as nonroot:nonroot
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
      livenessProbe:
        httpGet: { path: /healthz, port: 8080 }
        periodSeconds: 10
        failureThreshold: 3
      readinessProbe:
        httpGet: { path: /readyz, port: 8080 }
        periodSeconds: 1              # notice 503 draining within drainDelay
        failureThreshold: 1
```

Keep the readiness `periodSeconds` times `failureThreshold` below the 2s drain
delay, otherwise the pod can stop accepting connections before the endpoint is
removed and clients see resets during a rollout.

### Server timeouts

`newServer` sets conservative limits so slow or idle clients cannot hold
connections open indefinitely:

| Setting             | Value | Purpose                                      |
|---------------------|-------|----------------------------------------------|
| `ReadHeaderTimeout` | 5s    | Bounds slowloris-style header trickling       |
| `ReadTimeout`       | 10s   | Caps time to read the full request           |
| `WriteTimeout`      | 10s   | Caps time to write the response              |
| `IdleTimeout`       | 60s   | Closes idle keep-alive connections           |

Any handler that needs longer than 10s must be paired with a larger
`shutdownTimeout`, or it will be cut off during a rollout.

## Local checks (mirrors CI)

The `Makefile` mirrors the CI checks so you can run them before pushing:

```bash
make fmt-check vet test   # same gates as the sample-app workflow
make build                # static binary in bin/
make image VERSION=0.1.0  # distroless image
make help                 # list every target
```
