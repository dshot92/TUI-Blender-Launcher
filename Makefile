BIN := $(HOME)/.local/bin/tui-blender-launcher
BUILD_BIN := tui-blender-launcher

# Derive the version from git so local builds report something meaningful
# (e.g. v1.0.0, v1.0.0-5-gabc1234, or a short SHA). Override with
# `make build VERSION=1.2.3`. Falls back to "dev" outside a git checkout.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install run uninstall test lint vet fmt release-check snapshot

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_BIN) .

install: build
	mkdir -p $(dir $(BIN))
	rm -f $(BIN)
	ln -s $(PWD)/$(BUILD_BIN) $(BIN)

run: install
	$(BIN)

uninstall:
	rm -f $(BIN)

release-check:
	goreleaser check

snapshot:
	goreleaser release --snapshot --clean

fmt:
	gofmt -w .

test:
	go test -race ./...

vet:
	go vet ./...

lint: fmt vet test
