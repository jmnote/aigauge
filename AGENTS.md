# Contributor Notes

- Keep Wails bindings in `internal/app`; keep provider implementations in `internal/providers`.
- Test parsing and conversion with fixture JSON; unit tests must not require live network or CLI calls.
- Keep OS-specific process settings in platform-specific files if cross-platform builds are introduced.

See [docs/development.md](docs/development.md) for build, test, and pre-PR check commands, and
[docs/packaging.md](docs/packaging.md) for MSIX packaging and release commands.
