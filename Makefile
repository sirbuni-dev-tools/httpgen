VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date +%Y-%m-%dT%H:%M:%S%z)
PKG     := github.com/sirbuni-dev-tools/httpgen/internal/version
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

.PHONY: build test vet fmt

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/httpgen ./cmd/httpgen

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

PLUGIN_RULES ?= ../../INTELLIJ/custom-plugins/api-http-generator/src/main/resources/rules.json

.PHONY: sync-rules
# Copy the shared fake-value rules from the plugin, their source of truth.
sync-rules:
	cp $(PLUGIN_RULES) internal/rules/rules.json
