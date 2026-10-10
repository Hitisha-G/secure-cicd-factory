# sample-app

Minimal Go HTTP service used to exercise the pipelines in this repo.

- `GET /healthz` returns `{"status":"ok","version":"..."}`
- Listens on `:8080` (override with `PORT`)
- Multi-stage build into a distroless, non-root image
- Structured JSON logs to stdout (suitable for container log collectors)

```bash
docker build --build-arg VERSION=0.1.0 -t sample-app:local .
docker run --rm -p 8080:8080 sample-app:local
curl -s localhost:8080/healthz
```

## Graceful shutdown

The process traps `SIGINT` and `SIGTERM`, then calls `http.Server.Shutdown` with a
10-second drain so in-flight requests can finish before the process exits. That
matches how orchestrators (Kubernetes, ECS, Docker Compose) stop containers:
they send `SIGTERM`, wait for the grace period, then force-kill.

```bash
make run          # Ctrl-C triggers the same path as SIGTERM in a container
# or
docker run --rm -p 8080:8080 sample-app:local
# another terminal: docker stop <container>  # sends SIGTERM
```

## Local checks (mirrors CI)

The `Makefile` mirrors the CI checks so you can run them before pushing:

```bash
make fmt-check vet test   # same gates as the sample-app workflow
make build                # static binary in bin/
make image VERSION=0.1.0  # distroless image
make help                 # list every target
```
