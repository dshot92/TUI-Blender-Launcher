# Changelog

## v1.0.1 (2026-10-18)

### Fixes

- Keep an in-flight update download on the same row as its local build when
  leaving the settings view; the refresh no longer splits the transferring
  build into a separate row.

## v1.0.0 (2026-10-17)

### Highlights

- Browse, filter, download, and launch Blender builds from a terminal UI
- Version filters with wildcards (`4.x`, `5.x`)
- Multi-select release-type filtering (alpha, beta, candidate, stable)
- Real-time download and extraction progress with cancellation
- Checksum verification for downloaded builds
- Tab-completion for the download directory in settings
- Build information panel with adaptive placement
- Theme support (dracula, catppuccin, nord, gruvbox, tokyonight)
- Cross-platform support (Linux, macOS, Windows)

### Notes

- Configuration is stored in the OS user config directory.
- Downloaded builds are stored under the configured download directory.
EOF