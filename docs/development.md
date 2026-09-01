# Development guide

## Repository layout

```text
aigauge/
├── frontend/          # Webview UI, icon, and sanitized preview fixture
├── docs/               # Documentation and listing screenshots
├── hack/               # Packaging, capture, and local preview scripts
├── internal/app/       # Wails application bindings and services
├── internal/providers/ # Codex, Antigravity, and Claude usage providers
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

The app enforces a single running instance, so a leftover one from a previous `run` or `build`
silently blocks a new one from starting. `.\build.ps1 kill` stops any running `aigauge.exe`.

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
match `VERSION`. Local packages use an explicit suffix such as
`dist/aigauge_0.2.4.0_x64_local.msix`; the release workflow alone produces the canonical
`aigauge_0.2.4.0_x64.msix` asset. The versioned local package remains in `dist/` for inspection.
Remove generated packaging output explicitly when it is no longer needed:

```powershell
.\build.ps1 clean
```

`frontend/logo.svg` is the source logo. The checked-in `frontend/logo.png` is the raster asset used
by Windows executable resources and MSIX package icons. Windows builds also generate an ignored
`rsrc_windows_amd64.syso` file from `frontend/logo.png`. The resource embeds the AI Gauge icon and Windows file metadata into `aigauge.exe`. Install the
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
The preview uses `frontend/fixtures/sample-codex.json`, `sample-claude.json`, and
`sample-antigravity.json` - one fixture per provider, each holding exactly what that provider's
Wails RPC method returns - does not call Codex or Antigravity, and watches the entire `frontend/`
directory. Saving any frontend file (including a fixture) causes the browser preview to reload.

To refresh those fixtures with real data (using your own local Codex/Claude session and the local
`agy` CLI), run:

```powershell
.\build.ps1 fixtures
```

Because the output reflects your own account (plan tier, usage percentages, reset times), review
the diff before committing `frontend/fixtures/sample-*.json`.

## Listing screenshots

Capture the native Wails window in both themes:

```powershell
.\build.ps1 screenshot
```

`screenshot-light`/`screenshot-dark` launch the app with `--fixtures=frontend\fixtures`, so the
window renders the same `sample-*.json` fixtures the frontend preview uses instead of calling the
real provider APIs - no logged-in Codex/Claude/Antigravity account needed on the capturing machine,
and no waiting on a live fetch. Run `.\build.ps1 fixtures` first if those fixtures don't exist yet.

This runs the Light and Dark captures sequentially and writes:

- `docs/screenshots/aigauge-native-light.png`
- `docs/screenshots/aigauge-native-dark.png`

Individual captures can be run with `screenshot-light` or `screenshot-dark`. To adjust the render
wait or point at a different fixture set, invoke the capture helper directly, for example:

```powershell
.\hack\screenshot.ps1 -Theme light -RenderWaitSeconds 5 -FixturesDir frontend\fixtures
```

Omit `-FixturesDir` to capture against live provider data instead (needs real logged-in accounts,
and a longer `-RenderWaitSeconds` to give the real fetch time to finish).

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
