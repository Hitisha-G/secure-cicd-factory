# secure-cicd-factory

Reusable, security-first CI/CD building blocks for container workloads: build, scan, sign, and ship with policy gates instead of hope.

Aimed at DevOps / platform teams that want every pipeline to produce an image they can actually trust, whether it runs in GitHub Actions or Jenkins.

## Goals

- Reusable GitHub Actions workflows for build, test, and release of container images
- Supply-chain checks on every change: secret scanning, dependency and image vulnerability scans, IaC misconfiguration scans
- Signed images and SBOMs so deploy targets can verify what they run
- Short-lived cloud credentials via OIDC (no long-lived AWS keys in CI)
- A Jenkins shared-library path for teams that are not on Actions yet

## Planned layout

```
.github/workflows/          # reusable build / scan / release workflows
pipelines/
  jenkins/                  # declarative Jenkinsfile + shared library steps
policies/                   # scan thresholds and policy-as-code rules
examples/
  sample-app/               # small service + Dockerfile wired to the pipelines
scripts/                    # local helpers to reproduce CI checks
docs/                       # pipeline design notes and threat model
```

## Pipeline stages (target)

1. Lint and unit test
2. Secret scan and dependency audit
3. Build container image (multi-stage, non-root)
4. Image vulnerability scan with a severity gate
5. Generate SBOM and sign the image
6. Push to registry using OIDC-issued credentials

## Quick start

Everything runs from the repo root; the root `Makefile` delegates to `examples/sample-app`.

```bash
make help        # list every target
make ci-local    # gofmt check, go vet, go test -race (same gates as CI)
make build       # static binary in examples/sample-app/bin/
make image       # distroless, non-root container image
make run         # build and run the service on :8080
```

Requires Go (version pinned in `examples/sample-app/go.mod`) and Docker for `make image`.

## Status

Early work in progress; stages land incrementally with tests and docs.

| Stage | State |
| --- | --- |
| Lint and unit test | Done: [`sample-app-ci.yml`](.github/workflows/sample-app-ci.yml) runs gofmt, go vet, and `go test -race` |
| Build container image | Done: multi-stage distroless build (no push) after tests pass |
| Secret scan and dependency audit | Planned |
| Image vulnerability scan with severity gate | Planned |
| SBOM and image signing | Planned |
| Push to registry via OIDC | Planned |

CI hardening already in place: read-only `contents` permission, `persist-credentials: false` on checkout, pinned `ubuntu-24.04` runners, job timeouts, per-ref concurrency cancellation, and Go module caching.

The risks each stage is meant to close, and which controls are already live, are tracked in the [pipeline threat model](docs/threat-model.md).

## License

MIT, see [LICENSE](LICENSE).
