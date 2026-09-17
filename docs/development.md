# Development guide

## Repository layout

```text
aigauge/
├── frontend/          # Webview UI, icon, and sanitized preview fixture
├── docs/               # Documentation and listing screenshots
├── hack/               # Packaging, capture, and local preview scripts
├── internal/app/       # Wails application bindings and services
├── internal/providers/ # Codex, Claude, and Antigravity usage providers
├── internal/ui/        # Window, tray, and runtime wiring
├── build.ps1           # Build task entrypoint
├── Package.appxmanifest
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
node --test "frontend/*.test.mjs"
git diff --check
```

The frontend's pure rules live in `frontend/logic.mjs` (which provider states may be counted as
failures, how a stored config is normalized, how the retry backoff is capped) and are covered by
`frontend/logic.test.mjs` under node's built-in test runner - no test framework and no browser
stand-in. `.\build.ps1 test` runs the Go and JavaScript suites in sequence.

`hack/live-server.mjs` reproduces every provider state the UI can show without a CLI, an account,
or a network:

```text
/?state=login_required                  all three cards at once
/?codex=not_installed&claude=connected     one provider at a time
```

Before opening a PR, the combined local gate can be run with:

```powershell
.\build.ps1 checks
```

This also creates a local MSIX and verifies that its package name and staged manifest version
match the requested version (local builds default to `0.0.0`). Local packages use an explicit suffix such as
`dist/aigauge_0.2.4.0_x64_local.msix`; the release workflow alone produces the canonical
`aigauge_0.2.4.0_x64.msix` asset. The versioned local package remains in `dist/` for inspection.
Remove generated packaging output explicitly when it is no longer needed:

```powershell
.\build.ps1 clean
```

`frontend/images/logo.svg` is the source logo. The checked-in `frontend/images/logo.png` is the raster asset used
by Windows executable resources and MSIX package icons. Windows builds also generate an ignored
`rsrc_windows_amd64.syso` file from `frontend/images/logo.png`. The resource embeds the AI Gauge icon and Windows file metadata into `aigauge.exe`. Install the
resource generator once with `go install github.com/tc-hib/go-winres@v0.3.3` if it is not already
available.
The MSIX manifest supplies the Store icons on its own, so the PR-check workflows (`msix.yml`,
`pull-request.yml`) skip this step for speed. The release workflow (`release.yml`) does not skip
it: `aigauge.exe` is also uploaded to GitHub Releases as the portable executable, and without the
embedded resource that file has no icon at all in Explorer/the taskbar.

## Frontend preview

Start the fixture-backed browser preview:

```powershell
.\build.ps1 live-server
```

Open `http://localhost:8080/?theme=light` or `http://localhost:8080/?theme=dark`.
The preview serves whichever `hack/fixtures/display/display_<provider>_*.json` snapshot is newest per
provider - each holding exactly what that provider's Wails RPC method returns (`DisplayUsage`) -
does not call Codex, Claude or Antigravity, and watches both the `frontend/` and `hack/fixtures/`
directories. Saving any frontend file or fixture causes the browser preview to reload.

To capture a fresh snapshot (using AI Gauge's own stored credentials for an already-connected
provider instance), run:

```powershell
.\build.ps1 fixtures-usage
```

One API call per provider writes two files: `hack/fixtures/usage/usage_<provider>_*.json`, the API's raw
response byte for byte (Codex's `user_id`/`email` redacted) - useful on its own as a reference for
what that (often undocumented) endpoint actually returns - and `hack/fixtures/display/display_<provider>_*.json`,
that same response parsed and converted (`ParseXUsage` + `ToDisplay` - `internal/providers`) into the
`DisplayUsage` shape the app renders. Because the output reflects your own account (plan tier, usage
percentages, reset times), review it before committing either directory.

## Listing screenshots

Capture the native Wails window in both themes:

```powershell
.\build.ps1 screenshot
```

`screenshot-light`/`screenshot-dark` launch the app in its current configured state. The
This runs the Light and Dark captures sequentially and writes:

- `docs/screenshots/aigauge-native-light.png`
- `docs/screenshots/aigauge-native-dark.png`

Individual captures can be run with `screenshot-light` or `screenshot-dark`. To adjust the render
wait, invoke the capture helper directly, for example:

```powershell
.\hack\screenshot.ps1 -Theme light -RenderWaitSeconds 5
```

Capturing live provider data needs real logged-in accounts and a sufficiently long
`-RenderWaitSeconds` to give the fetch time to finish.

## MSIX packaging

```powershell
.\build.ps1 package
```

Release tags are the application version source of truth. For example, `v0.2.1` becomes the four-part
MSIX version `0.2.1.0`; pass the same tag to `build.ps1 -Version` for a matching local package. The staging directory is `dist/staging/`; the generated package is written
to `dist/` and ignored by Git.

The packaging script locates `makeappx.exe` from the Windows SDK. If it is not on `PATH`, pass its
full path through the existing packaging script parameter. `signtool.exe` is only needed when
creating a locally signed package.

### Releases

Releases are triggered by pushing a stable `vX.Y.Z` tag:

```powershell
git tag v0.6.2
git push origin v0.6.2
```

The `Release` GitHub Actions workflow (`.github/workflows/release.yml`) builds the MSIX, attaches both
the MSIX and the standalone portable executable (`aigauge_<version>_x64.exe`) plus `SHA256SUMS.txt` to
the GitHub release, and publishes the package to the Microsoft Store.

The portable `.exe` runs unsigned and needs no installation - unlike the MSIX, which either goes through
Store certification or needs a certificate matching the package Publisher installed and trusted
first. Running the portable `.exe` still triggers SmartScreen on a machine that has not seen it
before; that is a separate, much smaller prompt than installing a certificate.

> For Microsoft Store publishing, Partner Center credentials, and submission details, see
> [`hack/msstore/msstore.md`](../hack/msstore/msstore.md).

## Theme behavior

The application supports `--theme=light`, `--theme=dark`, and `--theme=system`. A forced command-line
theme is used for screenshots without overwriting the user's stored theme preference. Choosing a
theme in Settings updates the local preference.
