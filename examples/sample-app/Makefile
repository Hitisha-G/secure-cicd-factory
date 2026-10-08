# Local shortcuts that mirror the sample-app CI workflow.

IMAGE   ?= sample-app:local
VERSION ?= dev

.PHONY: help fmt fmt-check vet test build image run clean

help: ## List available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  %-10s %s\n", $$1, $$2}'

fmt: ## Rewrite Go files with gofmt
	gofmt -w .

fmt-check: ## Fail if any Go file needs gofmt (same as CI)
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "Files need gofmt:"; echo "$$out"; exit 1; fi

vet: ## Run go vet
	go vet ./...

test: ## Run tests with the race detector (same as CI)
	go test -race -count=1 ./...

build: ## Build a static binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/sample-app .

image: ## Build the distroless container image
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

run: build ## Build and run the service locally
	./bin/sample-app

clean: ## Remove build output
	rm -rf bin
