# Pipeline threat model

This document lists the supply-chain risks the factory is designed to reduce, and maps each one to the pipeline stage (or existing control) that addresses it. It is a living document: when a stage lands, its row moves from "planned" to "in place".

## Scope

In scope:

- Source changes pushed to `main` or proposed through pull requests
- The GitHub Actions workflows under `.github/workflows/` and, later, the Jenkins path under `pipelines/jenkins/`
- Container images built from `examples/sample-app` (and any service that reuses the workflows)
- Credentials the pipeline uses to reach a registry or cloud account

Out of scope for now: runtime security of the cluster that runs the image, and developer workstation compromise.

## Assets

| Asset | Why it matters |
| --- | --- |
| Source code and workflow files | Whoever controls them controls what gets built and shipped |
| CI runner environment | Executes untrusted code from pull requests |
| Registry and cloud credentials | Allow pushing images or touching infrastructure |
| Built image and its metadata | What actually runs in production |

## Threats and controls

| # | Threat | Control | State |
| --- | --- | --- | --- |
| T1 | Workflow token used to push or tamper with the repo | `permissions: contents: read` at workflow level | In place |
| T2 | Checkout leaves a usable token on disk for later steps | `persist-credentials: false` on every checkout | In place |
| T3 | Runner image drifts and silently changes the toolchain | Pinned `ubuntu-24.04` runners; Go version read from `go.mod` | In place |
| T4 | Hung or abused job burns minutes or hides activity | Per-job `timeout-minutes` and per-ref concurrency cancellation | In place |
| T5 | Vulnerable or malicious dependency or base image | Dependabot for actions, Go modules, and Docker base images | In place (updates); audit gate planned |
| T6 | Container runs with a shell or as root, widening blast radius | Multi-stage build onto distroless `static-debian12:nonroot` | In place |
| T7 | Secrets committed to the repo | Secret scanning on every push and pull request | Planned |
| T8 | Image ships with known critical CVEs | Image scan with a severity gate that fails the build | Planned |
| T9 | Deploy target cannot prove what it is running | SBOM generation and keyless image signing | Planned |
| T10 | Long-lived cloud keys stored as CI secrets get leaked | OIDC-issued, short-lived credentials scoped per repo and branch | Planned |
| T11 | Third-party action is retagged to malicious code | Pin actions to full commit SHAs, kept current by Dependabot | Planned |

## Pull requests from forks

Fork pull requests run with a read-only token and no repository secrets. Workflows here use `pull_request`, not `pull_request_target`, so untrusted code never runs with write access or secrets. Any future job that needs secrets (push, sign, deploy) will be limited to `push` on `main`.

## Residual risks

- Until T11 lands, actions are pinned to major-version tags, which the action owner can move.
- Until T7 and T8 land, a leaked secret or vulnerable image is caught only by review.
- The build is not yet reproducible or attested, so provenance relies on the CI log rather than a signed statement.

## Review

Revisit this table whenever a pipeline stage is added or a workflow gains a new permission or secret.
