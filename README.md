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

## Status

Early work in progress; stages land incrementally with tests and docs.

## License

MIT, see [LICENSE](LICENSE).
