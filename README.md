# AI Gauge

A lightweight Windows system tray widget for real-time OpenAI Codex and Google Antigravity usage monitoring.

---

## Features

- **Real-Time Tracking**: Displays remaining quotas (%) and reset countdowns for OpenAI Codex (5h, 7d) and Google Antigravity (`agy`) model groups.
- **Background Polling**: Automatically refreshes data every 60 seconds with non-overlapping asynchronous requests.
- **Status Indicator**: Visual indicator showing connection health (normal/warning/failed) with live tooltips.
- **Windows Tray Resident**: Runs in the system tray, minimizes to the taskbar, clamps to screen boundaries, and supports dark mode.

---

## How it works

AI Gauge runs as a lightweight Windows desktop process and keeps its small widget available from the system tray.

1. The widget calls the OpenAI Codex usage endpoint using the local Codex login session at `~/.codex/auth.json`.
2. It invokes the local `agy` CLI to retrieve Google Antigravity usage data in JSON format.
3. Both sources are queried independently and refreshed every 60 seconds without overlapping requests.
4. The UI displays remaining quota percentages, reset countdowns, fetch status, and the time of the last successful update.

The app does not use an AI Gauge intermediary server. Codex requests go directly to `chatgpt.com`, while Antigravity requests are handled by the locally installed `agy` CLI. Login credentials remain on the local machine and are used only for the corresponding service request.

---

## Prerequisites

- Windows 10 / 11 (64-bit)
- OpenAI Codex login session (`~/.codex/auth.json`)
- Google Antigravity CLI (`agy`)

---

## Build & Run

### Clone
```powershell
git clone https://github.com/jmnote/aigauge.git
cd aigauge
```

### Build
```powershell
.\build.ps1 -Task build
```

### Run
```powershell
.\build.ps1 -Task run
```

### Test
```powershell
.\build.ps1 -Task test
```

### Build an MSIX package
```powershell
.\package-msix.ps1
```

`VERSION` is the source of truth for the application version. The packaging script converts
the `v0.2.0`-style value to the four-part MSIX version `0.2.0.0` and writes it to the staging
manifest under `build/msix/`. The generated package is written to `dist/`, which is ignored by Git.

---

## Project Structure

```
aigauge/
├── frontend/        # Webview UI and application icon
├── internal/
│   ├── app/          # Wails application bindings and services
│   ├── providers/    # Codex & Antigravity usage providers
│   └── ui/           # Wails window, system tray, and runtime wiring
├── build.ps1        # Build, run, and test script
├── VERSION           # Application semantic version
├── go.mod           # Go module definition
├── go.sum           # Go checksums
├── main.go          # Application entrypoint and embedded assets
├── wails.json       # Wails project configuration
└── README.md        # Documentation
```

---

## Controls

- **Tray Left-Click**: Show and focus widget
- **Tray Right-Click**: Context menu (Show / Exit)
- **Titlebar Settings (`⚙`)**: Choose Light, Dark, or System theme
- **Titlebar Close (`×`)**: Prompts before hiding to the tray; use the tray's **Exit** menu item to quit

---

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
