BIN := $(HOME)/.local/bin/TUI-Blender-Launcher

.PHONY: build install run uninstall test lint vet fmt

build:
	go build

install: build
	mkdir -p $(dir $(BIN))
	rm -f $(BIN)
	ln -s $(PWD)/TUI-Blender-Launcher $(BIN)

run: install
	$(BIN)

uninstall:
	rm $(BIN)

fmt:
	gofmt -w .

test:
	go test -race ./...

vet:
	go vet ./...

lint: fmt vet test
