# Development guide

## Repository layout

```text
aigauge/
├── frontend/          # Webview UI, icon, and sanitized preview fixture
├── docs/               # Documentation and listing screenshots
├── hack/               # Packaging, capture, and local preview scripts
├── internal/app/       # Wails application bindings and services
├── internal/config/    # Local preferences and settings migrations
├── internal/providers/ # Codex, Claude, and Antigravity usage providers
├── internal/ui/        # Window, tray, and runtime wiring
├── build.ps1           # Build task entrypoint
├── Package.appxmanifest
└── wails.json
```

Keep Wails bindings in `internal/app` and provider implementations in `internal/providers`. Test
parsing and conversion with fixture JSON; unit tests must not require live network or CLI calls.
Keep OS-specific process settings in platform-specific files if cross-platform builds are introduced.

## Build, run, and test

```powershell
.\build.ps1 build
.\build.ps1 run
.\build.ps1 test
```

If script execution is blocked, run with a process-scoped execution policy override:

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass; .\build.ps1 build
```

The app enforces a single running instance, so a leftover one from a previous `run` or `build`
silently blocks a new one from starting. `.\build.ps1 kill` stops any running `aigauge.exe`.

Before submitting changes, run:

```powershell
gofmt -w main.go internal
go test ./...
go test hack/fixtures/fixtures.go hack/fixtures/fixtures_test.go
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

See [packaging.md](packaging.md) for what this also verifies locally and how to clean up
generated packaging output.

## Frontend preview

Start the fixture-backed browser preview:

```powershell
.\build.ps1 live-server
```

Open `http://localhost:8080/?theme=light` or `http://localhost:8080/?theme=dark`.
The preview serves `hack/fixtures/usage/display_<provider>.json` per
provider - each holding exactly what that provider's Wails RPC method returns (`DisplayUsage`) -
does not call Codex, Claude or Antigravity, and watches both the `frontend/` and `hack/fixtures/`
directories. Saving any frontend file or fixture causes the browser preview to reload.

To capture a fresh snapshot (using AI Gauge's own stored credentials for an already-connected
provider instance), run:

```powershell
.\build.ps1 fixtures-usage
```

Existing fixture files are skipped and never overwritten. Delete the relevant files first when
you intentionally want to capture a fresh snapshot.

If parsing or display conversion fails, the sanitized raw `usage_<provider>.json` is still
saved and the command reports the error (including available diagnostic details). No display
fixture is written on failure. Sanitization failures prevent saving the response. Move or delete
the saved raw fixture before retrying, since it also causes subsequent captures to be skipped.

One API call per provider writes two files: `hack/fixtures/usage/usage_<provider>.json`, the API's raw
response byte for byte (Codex's `user_id`/`email` obfuscated) - useful on its own as a reference for
what that (often undocumented) endpoint actually returns - and `hack/fixtures/usage/display_<provider>.json`,
that same response parsed and converted (`ParseXUsage` + `ToDisplay` - `internal/providers`) into the
`DisplayUsage` shape the app renders. Because the output reflects your own account (plan tier, usage
percentages, reset times), review it before committing either directory.

The token fixture task also captures Claude's authenticated profile response, including the
account display name used to identify browser-authenticated sessions:

```powershell
.\build.ps1 fixtures-tokens
```

This writes `hack/fixtures/tokens/profile-claude.json` alongside the token fixtures, skipping
existing files rather than overwriting them. UUID, email, and name fields are obfuscated before
the response is written.

## Listing screenshots

Capture the native Wails window in both themes:

```powershell
.\build.ps1 screenshot
```

`screenshot-light`/`screenshot-dark` launch the app with its configured provider instances.
The combined task runs the Light and Dark captures sequentially and writes:

- `docs/screenshots/aigauge-native-light.png`
- `docs/screenshots/aigauge-native-dark.png`

Individual captures can be run with `screenshot-light` or `screenshot-dark`. To adjust the render
wait, invoke the capture helper directly, for example:

```powershell
.\hack\screenshot.ps1 -Theme light -RenderWaitSeconds 5
```

Capturing live provider data needs real logged-in accounts and a sufficiently long
`-RenderWaitSeconds` to give the fetch time to finish.

## Windows startup behavior

Settings offers **Off** (the default), **Show window**, and **Start in tray**. The mode is saved as
`startupMode` in the existing `aigauge/settings.json` under `os.UserConfigDir()`. For MSIX installations,
Windows owns the startup task's enabled/disabled state; the settings screen reads it again after every
change and when focused.

The MSIX `desktop:StartupTask` named `AIGaugeStartup` launches `aigauge.exe` directly. Before creating
any windows, the app reads `Windows.ApplicationModel.AppInstance.GetActivatedEventArgs()` and applies
the tray preference only for `ActivationKind.StartupTask`. A normal Start menu launch shows the window.
This uses the OS WinRT API on the package's minimum Windows version (10.0.17763.0); there is no separate
startup executable or Windows App SDK runtime dependency. See the
[Windows activation documentation](https://learn.microsoft.com/en-us/windows/apps/desktop/modernize/get-activation-info-for-packaged-apps).

Portable builds register the full executable path under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
as `AIGauge`, adding `--hidden` for tray startup. The explicit `--hidden`, `--tray`, and `--minimized`
flags also hide the initial window during manual testing. **Off** removes the portable Run entry.

WinRT calls and COM object releases stay on one locked OS thread. Successful initialization, including
`S_FALSE`, is balanced by `RoUninitialize`. Enabling a startup task checks its returned state; a user or
policy block is reported instead of being treated as success. The app saves the new mode only after
Windows accepts it and attempts to restore the previous OS state if saving fails, reporting rollback
errors if necessary.

Automated tests use `internal/app/testdata/startup-activation.json` and injected handlers without
registering a real startup task. Before release, also test an installed MSIX:

1. On a clean installation, verify that **Start with Windows** is **Off**.
2. Select **Show window**, sign out and back in, and verify that the window appears.
3. Select **Start in tray**, sign out and back in, and verify that only the tray icon appears.
4. Exit the app, then launch it from the Start menu with **Start in tray** still selected; verify that
   the window appears. Repeat opening Settings to check subsequent state queries.
5. Disable AI Gauge in Task Manager or Windows Settings, return to the app, and try to enable it.
   Verify that the app reports the block and displays **Off** until it is re-enabled in Windows.
6. Select **Off** and verify no automatic launch at the next sign-in.

## Threshold preferences

Warning and Critical are independent dropdowns with **Disabled** or 5% through 100% in 5% steps.
Critical takes precedence when both match, so an enabled Critical threshold must be at or below Warning;
Critical at 100% marks every remaining-usage level as critical.
Loading an older settings file rounds and clamps numeric values to this range in memory; the normalized
values are persisted on the next explicit settings save, keeping each threshold's enabled flag. For
example, 98/99 becomes 100 and 0/1 becomes 5.

Go and JavaScript each cover these conversions with table-driven test cases.
If a settings write fails, the UI reloads the saved values and displays a notification.

## Theme behavior

The application supports `--theme=light`, `--theme=dark`, and `--theme=system`. A forced command-line
theme is used for screenshots without overwriting the user's stored theme preference. Choosing a
theme in Settings updates the local preference.
