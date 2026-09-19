# Contributor Notes

- Keep Wails bindings in `internal/app`; keep provider implementations in `internal/providers`.
- Test parsing and conversion with fixture JSON; unit tests must not require live network or CLI calls.
- Keep OS-specific process settings in platform-specific files if cross-platform builds are introduced.
- Before submitting changes, run `gofmt`, `go test ./...`, and `git diff --check`.
- Before opening a PR, run `./build.ps1 checks` to execute the formatting, test, and diff checks
  together. MSIX packaging is covered separately by CI.
