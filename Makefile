# Root shortcuts that delegate to examples/sample-app.

SAMPLE_APP ?= examples/sample-app

.PHONY: help fmt fmt-check vet test build image run clean ci-local

help: ## List available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  %-12s %s\n", $$1, $$2}'

fmt: ## Rewrite Go files with gofmt
	$(MAKE) -C $(SAMPLE_APP) fmt

fmt-check: ## Fail if any Go file needs gofmt (same as CI)
	$(MAKE) -C $(SAMPLE_APP) fmt-check

vet: ## Run go vet
	$(MAKE) -C $(SAMPLE_APP) vet

test: ## Run tests with the race detector (same as CI)
	$(MAKE) -C $(SAMPLE_APP) test

build: ## Build a static binary into examples/sample-app/bin/
	$(MAKE) -C $(SAMPLE_APP) build

image: ## Build the distroless container image
	$(MAKE) -C $(SAMPLE_APP) image

run: ## Build and run the service locally
	$(MAKE) -C $(SAMPLE_APP) run

clean: ## Remove sample-app build output
	$(MAKE) -C $(SAMPLE_APP) clean

ci-local: fmt-check vet test ## Reproduce the CI lint-test job locally
