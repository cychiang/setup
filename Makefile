VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse HEAD 2>/dev/null)
DATE    ?= $(shell TZ=UTC git show -s --date=format-local:%Y-%m-%dT%H:%M:%SZ --format=%cd HEAD 2>/dev/null)
PKG     := github.com/cychiang/setup/internal/version
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

STATICCHECK := honnef.co/go/tools/cmd/staticcheck@v0.8.2
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.10.0

.DEFAULT_GOAL := help
.PHONY: help build run test lint vuln e2e dist release install clean

help: ## list targets
	@awk -F':.*## ' '/^[a-z0-9]+:.*## /{printf "  %-8s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## build bin/setup with version stamping
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/setup ./cmd/setup

run: build ## run bin/setup, for example: make run ARGS="check --json"
	./bin/setup $(ARGS)

test: ## run unit and in-process end-to-end tests
	go test ./...

lint: ## gofmt check, go vet, staticcheck
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...
	go run $(STATICCHECK) ./...

vuln: ## scan dependencies and stdlib usage with govulncheck
	go run $(GOVULNCHECK) ./...

e2e: ## replay the full workflow against the real catalog (needs network)
	sh scripts/e2e.sh

dist: ## cross-compile release archives into dist/, for example: make dist VERSION=v0.1.0
	sh scripts/release.sh $(VERSION)

release: test dist ## publish a GitHub release from dist/ with gh, for example: make release VERSION=v0.1.0
	gh release create $(VERSION) dist/* --title $(VERSION) --generate-notes

install: ## install setup into GOBIN with version stamping
	go install -trimpath -ldflags '$(LDFLAGS)' ./cmd/setup

clean: ## remove build outputs
	$(RM) -r bin dist
