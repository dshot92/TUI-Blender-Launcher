BIN := $(HOME)/.local/bin/tui-blender-launcher
BUILD_BIN := tui-blender-launcher

.PHONY: build install run uninstall test lint vet fmt release-check

build:
	go build -o $(BUILD_BIN) .

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

fmt:
	gofmt -w .

test:
	go test -race ./...

vet:
	go vet ./...

lint: fmt vet test
