# AI Gauge

A lightweight Windows system tray widget for real-time OpenAI Codex and Google Antigravity usage monitoring.

---

## Features

- **Real-Time Tracking**: Displays remaining quotas (%) and reset countdowns for OpenAI Codex (5h, 7d) and Google Antigravity (`agy`) model groups.
- **Background Polling**: Automatically refreshes data every 60 seconds with non-overlapping asynchronous requests.
- **Status Indicator**: Visual indicator showing connection health (normal/warning/failed) with live tooltips.
- **Windows Tray Resident**: Runs in the system tray, hides on close (`×`), clamps to screen boundaries, and supports dark mode.

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

---

## Project Structure

```
aigauge/
├── frontend/        # Webview UI and application icon
├── build.ps1        # Build, run, and test script
├── go.mod           # Go module definition
├── go.sum           # Go checksums
├── main.go          # Go backend logic and Wails lifecycle
├── wails.json       # Wails project configuration
└── README.md        # Documentation
```

---

## Controls

- **Tray Left-Click**: Show and focus widget
- **Tray Right-Click**: Context menu (Show / Exit)
- **Titlebar Minimize (`−`)**: Minimize to taskbar
- **Titlebar Close (`×`)**: Hide to system tray

---

## License

Personal and internal use.
