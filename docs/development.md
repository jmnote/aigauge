# Development guide

## Repository layout

```text
aigauge/
├── frontend/          # Webview UI, icon, and sanitized preview fixture
├── docs/               # Documentation and listing screenshots
├── hack/               # Packaging, capture, and local preview scripts
├── internal/app/       # Wails application bindings and services
├── internal/providers/ # Codex and Antigravity usage providers
├── internal/ui/        # Window, tray, and runtime wiring
├── build.ps1           # Build task entrypoint
├── Package.appxmanifest
├── VERSION
└── wails.json
```

## Build, run, and test

```powershell
.\build.ps1 build
.\build.ps1 run
.\build.ps1 test
```

Before submitting changes, run:

```powershell
gofmt -w main.go internal
go test ./...
git diff --check
```

Before opening a PR, the combined local gate can be run with:

```powershell
.\build.ps1 checks
```

This also creates a local MSIX and verifies that its package name and staged manifest version
match `VERSION`.

Windows builds also generate an ignored `rsrc_windows_amd64.syso` file from `frontend/icon.png`.
The resource embeds the AI Gauge icon and Windows file metadata into `aigauge.exe`. Install the
resource generator once with `go install github.com/tc-hib/go-winres@v0.3.3` if it is not already
available.
The executable-resource step is local-only; the CI MSIX workflow skips it because the MSIX
manifest supplies the Store icons and CI does not need the optional executable icon.

## Frontend preview

Start the fixture-backed browser preview:

```powershell
.\build.ps1 live-server
```

Open `http://localhost:8080/?theme=light` or `http://localhost:8080/?theme=dark`.
The preview uses `frontend/fixtures/sample.json`, does not call Codex or Antigravity, and watches
the entire `frontend/` directory. Saving any frontend file causes the browser preview to reload.

## Listing screenshots

Capture the native Wails window in both themes:

```powershell
.\build.ps1 snapshot
```

This runs the Light and Dark captures sequentially and writes:

- `docs/screenshots/aigauge-native-light.png`
- `docs/screenshots/aigauge-native-dark.png`

Individual captures can be run with `screenshot-light` or `screenshot-dark`. Use
`-RenderWaitSeconds` to adjust the render wait when needed.

## MSIX packaging

```powershell
.\build.ps1 package
```

`VERSION` is the application version source of truth. For example, `v0.2.1` becomes the four-part
MSIX version `0.2.1.0`. The staging directory is `dist/staging/`; the generated package is written
to `dist/` and ignored by Git.

The package uses the Partner Center identity in `Package.appxmanifest`. Do not replace its
`Identity Name` or `Publisher` with an arbitrary certificate or publisher value. Microsoft Store
submission handles Store package signing; local sideloading requires a certificate matching the
package Publisher.

The packaging script locates `makeappx.exe` from the Windows SDK. If it is not on `PATH`, pass its
full path through the existing packaging script parameter. `signtool.exe` is only needed when
creating a locally signed package.

## Theme behavior

The application supports `--theme=light`, `--theme=dark`, and `--theme=system`. A forced command-line
theme is used for screenshots without overwriting the user's stored theme preference. Choosing a
theme in Settings updates the local preference.
