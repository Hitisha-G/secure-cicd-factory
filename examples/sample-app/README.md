# sample-app

Minimal Go HTTP service used to exercise the pipelines in this repo.

- `GET /healthz` returns `{"status":"ok","version":"..."}`
- Listens on `:8080` (override with `PORT`)
- Multi-stage build into a distroless, non-root image

```bash
docker build --build-arg VERSION=0.1.0 -t sample-app:local .
docker run --rm -p 8080:8080 sample-app:local
curl -s localhost:8080/healthz
```

The `Makefile` mirrors the CI checks so you can run them before pushing:

```bash
make fmt-check vet test   # same gates as the sample-app workflow
make build                # static binary in bin/
make image VERSION=0.1.0  # distroless image
make help                 # list every target
```
