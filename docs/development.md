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

`hack/live-server.ps1` reproduces every provider state the UI can show without a CLI, an account,
or a network:

```text
/?state=login_required                  all three cards at once
/?codex=not_installed&claude=connected     one provider at a time
/?view=sample                              open in the sample preview
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

`frontend/logo.svg` is the source logo. The checked-in `frontend/logo.png` is the raster asset used
by Windows executable resources and MSIX package icons. Windows builds also generate an ignored
`rsrc_windows_amd64.syso` file from `frontend/logo.png`. The resource embeds the AI Gauge icon and Windows file metadata into `aigauge.exe`. Install the
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
The preview uses `hack/fixtures/sample-codex.json`, `sample-claude.json`, and
`sample-antigravity.json` - one fixture per provider, each holding exactly what that provider's
Wails RPC method returns - does not call Codex or Antigravity, and watches both the `frontend/`
and `hack/fixtures/` directories. Saving any frontend file or fixture causes the browser preview
to reload.

To refresh those fixtures with real data (using your own local Codex/Claude session and the local
`agy` CLI), run:

```powershell
.\build.ps1 fixtures-json
```

Because the output reflects your own account (plan tier, usage percentages, reset times), review
the diff before committing `hack/fixtures/sample-*.json`.

The app's sample-data preview does not read `hack/fixtures/*.json` directly - it imports
`internal/app/fixtures`, a small generated package (`internal/app/fixtures/fixtures.go`) that
embeds each fixture's JSON as a Go byte-slice constant, so the sample data compiles straight into
the binary with no file read of any kind at runtime. Regenerate it after changing
`hack/fixtures/*.json`:

```powershell
.\build.ps1 fixtures-go
```

Unlike `fixtures-json`, this is a pure local transform (no accounts, no network), so it's safe to
run in CI or by any contributor. `.\build.ps1 fixtures` runs both `fixtures-json` and `fixtures-go`
in sequence. `internal/app/fixtures/fixtures.go` is generated code and is committed to git like any
other generated file - `go build`/`go test` do not regenerate it on their own, so remember to run
`fixtures-go` and commit the result whenever `hack/fixtures/*.json` changes.

## Listing screenshots

Capture the native Wails window in both themes:

```powershell
.\build.ps1 screenshot
```

`screenshot-light`/`screenshot-dark` launch the app in its current configured state. The
application intentionally has no startup switch that bypasses its user-visible navigation. To
capture the sample-data preview, first disable all providers in Settings. Then run the individual
capture helper with a long enough render wait and select **Preview with sample data** in the window
it launches before the capture occurs. For automated browser-based visual work, the live server's
`?view=sample` route remains available.

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

### Release and Microsoft Store publishing

The release workflow runs only when a `v*` tag is pushed. Create and push a tag manually after
merging the release commit:

```powershell
git tag v0.6.2
git push origin v0.6.2
```

The workflow builds the MSIX using that tag, creates the GitHub Release, and publishes the package
to Microsoft Store. Configure these repository or environment secrets before using Store publishing:

- `AZURE_AD_TENANT_ID`
- `SELLER_ID`
- `AZURE_AD_APPLICATION_CLIENT_ID`
- `AZURE_AD_APPLICATION_SECRET`

These four values are the Partner Center app credentials used by `msstore reconfigure`. The Store
product ID passed to `msstore publish` is configured as the app's public Store ID in the workflow.
The workflow uses Microsoft's
[`microsoft-store-apppublisher`](https://github.com/microsoft/microsoft-store-apppublisher) action
to install the Microsoft Store Developer CLI.

Store submission overrides and certification test instructions are maintained in
[`docs/partner-center/submission-overrides.yaml`](partner-center/submission-overrides.yaml). When the YAML file exists,
the workflow leaves the Store submission as a draft, applies only the declared overrides, and then
commits the submission. This keeps the Partner Center metadata reviewable alongside the code while
preserving undeclared Store settings.

The package uses the Partner Center identity in `Package.appxmanifest`. Do not replace its
`Identity Name` or `Publisher` with an arbitrary certificate or publisher value. Microsoft Store
submission handles Store package signing; local sideloading requires a certificate matching the
package Publisher.

The manifest's `runFullTrust` capability is required because AI Gauge is a native Wails/Win32
application with a system tray UI and it invokes the locally installed `agy` CLI.

### Optional Store certification smoke test

When preparing a Microsoft Store resubmission, the following quick checks may be useful. They are
guidance only and are not a required gate for ordinary pull requests:

- On a clean device, verify that providers without local setup show clear guidance.
- Open the sample preview and confirm it works without credentials or network access.
- Check quota percentages, reset times, refresh, settings, themes, and always-on-top behavior.
- Confirm setup-screen navigation and system-tray minimize/restore behavior.

The packaging script locates `makeappx.exe` from the Windows SDK. If it is not on `PATH`, pass its
full path through the existing packaging script parameter. `signtool.exe` is only needed when
creating a locally signed package.

The `Release` GitHub Actions workflow (`.github/workflows/release.yml`) additionally copies the
same build's `aigauge.exe` into `dist/` under the MSIX's own name (e.g. `aigauge_0.5.1.0_x64.exe`
alongside `aigauge_0.5.1.0_x64.msix`) and attaches both, plus `SHA256SUMS.txt`, to the GitHub
release. It runs unsigned and needs no installation - unlike the MSIX, which either goes through
Store certification or needs a certificate matching the package Publisher installed and trusted
first. Running the portable `.exe` still triggers SmartScreen on a machine that has not seen it
before; that is a separate, much smaller prompt than installing a certificate.

## Theme behavior

The application supports `--theme=light`, `--theme=dark`, and `--theme=system`. A forced command-line
theme is used for screenshots without overwriting the user's stored theme preference. Choosing a
theme in Settings updates the local preference.
