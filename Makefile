BIN := $(HOME)/.local/bin/TUI-Blender-Launcher

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
