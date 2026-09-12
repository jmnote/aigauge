# AI Gauge

<p align="center">
  <img src="frontend/logo.svg" width="96" alt="AI Gauge logo">
</p>

<p align="center">A lightweight Windows tray widget for monitoring OpenAI Codex, Claude Code, and Google Antigravity usage.</p>

<p align="center">
  <a href="https://apps.microsoft.com/detail/9MT65KM56P99">
    <img src="https://get.microsoft.com/images/en-us%20dark.svg" width="200" alt="Get AI Gauge from Microsoft Store">
  </a>
</p>

<p align="center">
  <img src="docs/screenshots/aigauge-native-light.png" width="320" alt="AI Gauge Light theme">
  <img src="docs/screenshots/aigauge-native-dark.png" width="320" alt="AI Gauge Dark theme">
</p>

## Features

- View remaining Codex quotas and reset times for the 5-hour and 7-day windows.
- View remaining Claude Code quotas and reset times for the 5-hour and 7-day (weekly) windows.
- View Google Antigravity (`agy`) model-group quotas and reset times.
- Enable or disable Codex, Claude, and Antigravity monitoring independently.
- Reorder the provider cards to match your preference.
- Automatically adjusts window size to fit active content.
- Keep the widget always on top with the title bar pin button.
- Refresh usage automatically in the background at a configurable interval.
- Keep the widget in the Windows system tray.
- Show or hide the widget from anywhere with an optional global hotkey.
- Choose Light, Dark, or System appearance.
- Configure Warning and Critical thresholds for usage bars.
- Persist settings locally between sessions.

## Usage

Open AI Gauge from the Start menu or system tray. Left-click the tray icon to show the widget.
Click the pin button on the title bar to toggle **Always on top**. Open **Settings** to enable or
disable providers and reorder their cards, configure Warning and Critical thresholds, change the
refresh interval, choose Light, Dark, or System appearance, and optionally configure a global hotkey.
The optional global hotkey shows or hides the widget even while another application is active. To
configure it, open **Settings** and choose a shortcut from the **Hotkey** dropdown. It supports
<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>G</kbd>, <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Q</kbd>, and
<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>E</kbd>; choose **Disabled** to turn it off. The hotkey is
disabled by default.
The widget's `×` button hides it to the tray. Closing it from the taskbar, pressing Alt+F4, or
choosing **Exit** from the tray menu quits the application.

## Requirements

- Windows 10 or Windows 11 (64-bit)
- A local OpenAI Codex login session
- A local Claude Code login session, if Claude usage is needed
- The Google Antigravity `agy` command-line tool, if Antigravity usage is needed

## How it works

AI Gauge is a standalone Windows application. It reads the local Codex and Claude Code login
sessions and invokes the locally installed `agy` command-line tool when Antigravity usage is
enabled. It then displays the retrieved usage information in the widget.

## Privacy

AI Gauge is a standalone local application. Usage data is processed and displayed on your
Windows device and is not stored by AI Gauge. AI Gauge does not request or store passwords,
payment information, or unrelated personal data. It uses the existing local Codex and Claude Code
login sessions and the authentication managed by `agy` without storing a separate copy of their
credentials.

Any network communication and data handling by connected services are governed by their own
authentication and privacy policies.

## Troubleshooting

- If Codex data is unavailable, verify that the local Codex login session is active.
- AI Gauge uses the access token maintained by Codex and does not refresh it itself. If the token
  has expired, log in again with Codex so that `~/.codex/auth.json` is updated.
- If Claude data is unavailable, verify that the local Claude Code login session is active.
  AI Gauge uses the access token maintained by Claude Code and does not refresh it itself. If the
  token has expired, log in again with Claude Code so that `~/.claude/.credentials.json` is
  updated.
- If Antigravity data is unavailable, verify that `agy` is installed and available to the app.
- If the selected global hotkey is already used by another application, choose a different shortcut
  or free the shortcut, then select **Retry** in Settings.
- Check the status dot tooltip for failure count, last successful fetch, last error, and next fetch.

## For developers

See [docs/development.md](docs/development.md) for build, test, frontend preview, screenshot,
and MSIX packaging instructions.

When building directly on Windows, run the build from PowerShell with a process-scoped execution
policy override if script execution is blocked:

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\build.ps1 build
```

The same build can be started as a single command:

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass; .\build.ps1 build
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
