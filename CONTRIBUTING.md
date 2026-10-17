# Contributing

Thank you for considering a contribution to TUI Blender Launcher.

## Development setup

Requirements:

- Go 1.23 or newer
- Git

```bash
git clone https://github.com/dshot92/TUI-Blender-Launcher.git
cd TUI-Blender-Launcher
go mod tidy
go test ./...
```

## Before opening a pull request

Please run:

```bash
make fmt
make vet
make test
```

Or:

```bash
gofmt -w .
go vet ./...
go test -race ./...
```

## Guidelines

- Keep changes focused and minimal.
- Add or update tests for behavior changes.
- Use `gofmt` formatting.
- Prefer clear, user-facing error messages.
- Do not commit generated binaries or local config files.

## Reporting bugs

Open a GitHub issue with:

- operating system and architecture
- Go version if building from source
- version of TUI Blender Launcher (`tui-blender-launcher --version`)
- steps to reproduce
- expected and actual behavior
```

## Security

See `SECURITY.md`.