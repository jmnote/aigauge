# AI Gauge

<p align="center">
  <img src="frontend/images/logo.svg" width="96" alt="AI Gauge logo">
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
- Add multiple independent Codex or Claude account instances and reorder their cards.
- Add an Antigravity instance backed by the installed `agy` CLI.
- Connect through the official browser login flow or import an existing local CLI session.
- Remove provider instances and their locally stored AI Gauge credentials.
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
Click the pin button on the title bar to toggle **Always on top**. Open **Settings** to add, connect,
import, remove, and reorder provider instances, configure Warning and Critical thresholds, change the
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
- An OpenAI Codex account, if Codex usage is needed
- An Anthropic Claude Code account, if Claude usage is needed
- The Google Antigravity `agy` command-line tool, if Antigravity usage is needed

## How it works

AI Gauge is a standalone Windows application. It authenticates Codex and Claude Code through their
official browser flows or imports their local CLI sessions, then stores the resulting credentials in
its own protected local token store. For Antigravity it invokes the locally installed `agy` command-line
tool. It then displays the retrieved usage information in the widget.

## Privacy

AI Gauge is a standalone local application. Usage data is processed and displayed on your
Windows device and is not stored by AI Gauge. AI Gauge does not request or store passwords,
payment information, or unrelated personal data. OAuth tokens used for Codex and Claude Code are
stored locally in AI Gauge's protected credential store; Antigravity authentication remains managed
by `agy` and is not copied into AI Gauge.

Any network communication and data handling by connected services are governed by their own
authentication and privacy policies.

See the full [Privacy Policy](docs/privacy-policy.md).

## Troubleshooting

- If Codex data is unavailable, use **Connect** in AI Gauge to complete the official browser login
  flow again, or import the updated `~/.codex/auth.json` session from Settings.
- If Claude data is unavailable, use **Connect** in AI Gauge to complete the official browser login
  flow again, or import the updated `~/.claude/.credentials.json` session from Settings.
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
