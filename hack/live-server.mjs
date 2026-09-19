import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import child_process from "node:child_process";
import { repoRoot } from "./lib/paths.mjs";

const frontendRoot = path.join(repoRoot, "frontend");
const fixturesRoot = path.join(repoRoot, "hack", "fixtures");

const PORT = parseInt(process.env.PORT || "8080", 10);

const MIME_TYPES = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".png": "image/png",
  ".svg": "image/svg+xml",
  ".ico": "image/x-icon",
};

const WAILS_RUNTIME = `globalThis.__AIGAUGE_LIVE__ = true;
const params = new URLSearchParams(location.search);
const theme = params.get('theme');

// Each provider's fixture file holds exactly what its Wails RPC method
// returns (DisplayUsage - see hack/fixtures/fixtures.go), so it doubles as
// the live-server fixture with no conversion step between the two. The
// server resolves "latest/<provider>.json" to whichever
// hack/fixtures/usage/display_<provider>.json snapshot is used, so a fresh
// \`.\\build.ps1 fixtures-usage\` capture needs no server restart.
const providers = {
  Codex: 'latest/codex.json',
  Claude: 'latest/claude.json',
  Antigravity: 'latest/antigravity.json',
  Copilot: 'latest/copilot.json',
};

// Every provider state the real app can show, reproducible here with no CLI,
// no account and no network:
//
//   /?state=login_required                        all three cards at once
//   /?codex=not_installed&claude=connected       one provider at a time
//
// The wording only has to be close enough to lay out like the real thing; the
// authoritative copy lives in internal/providers.
const stateMessages = {
  not_installed: 'Install the CLI and log in to monitor your quota.',
  auth_check_required: 'Credentials found. Connect to verify usage.',
  login_required: 'Log in to view quota information.',
  usage_unavailable: 'Quota information is not available for this account.',
  temporary_error: 'Could not reach the service right now. Retry in a moment.',
  unsupported_cli: 'This CLI version is not supported. Update the CLI.',
  connected: '',
};

// A static stand-in for internal/config.Settings: one provider instance per
// fixture, enough for app.js's backend-settings-based dashboard to
// render the three provider cards used by the fixture-backed browser server.
let mockSettings = {
  providers: Object.keys(providers).map(key => ({
    id: key.toLowerCase(), type: key.toLowerCase(), label: key, enabled: true,
  })),
  windowWidth: 250,
  theme: 'system',
  refreshInterval: 120,
  thresholds: { warning: { enabled: true, value: 50 }, critical: { enabled: true, value: 20 } },
  hotkeyShortcut: '',
  startupMode: 'off',
};

const stateFor = key => params.get(key.toLowerCase()) || params.get('state') || '';

const fixture = async key => {
  const response = await fetch(\`/fixtures/\${providers[key]}\`, { cache: 'no-store' });
  const usage = await response.json();
  usage.status = 'connected';
  return usage;
};

const diagnosisFor = key => {
  // Without an explicit state, show the case a configured machine actually
  // lands on: everything found locally, nothing verified over the network yet.
  const status = stateFor(key) || 'auth_check_required';
  return {
    status,
    message: stateMessages[status] ?? '',
    details: status === 'not_installed' ? 'live-server: synthetic provider state' : '',
  };
};

const usageFor = async key => {
  const status = stateFor(key);
  if (status && status !== 'connected') {
    const message = stateMessages[status] ?? '';
    return { status, message, error: message };
  }
  return fixture(key);
};

export const Call = {
  ByName: async (name, ...args) => {
    for (const key of Object.keys(providers)) {
      if (name.endsWith(\`Diagnose\${key}\`)) return diagnosisFor(key);
      if (name.endsWith(\`Get\${key}Usage\`)) return usageFor(key);
    }
    if (name.endsWith('GetSettings')) return mockSettings;
    if (name.endsWith('GetStartWithWindows')) return mockSettings.startupMode;
    if (name.endsWith('SetStartWithWindows')) {
      mockSettings.startupMode = args[0] || 'off';
      return null;
    }
    if (name.endsWith('SetTheme') || name.endsWith('SetSavedWindowWidth') ||
        name.endsWith('SetThresholds') || name.endsWith('SetHotkeyShortcut') ||
        name.endsWith('SetProviderRefreshInterval') || name.endsWith('SetProviderOrder')) return null;
    if (name.endsWith('AddProviderInstance') || name.endsWith('RemoveProviderInstance')) return null;
    if (name.endsWith('GetThemeOverride')) return ['light', 'dark', 'system'].includes(theme) ? theme : '';
    if (name.endsWith('GetVersion')) return 'vDEV';
    if (name.endsWith('SetContentHeight')) return null;
    if (name.endsWith('SetWindowWidth')) return null;
    if (name.endsWith('SetAlwaysOnTop')) return null;
    if (name.endsWith('HideToTray')) return null;
    return null;
  }
};
export const Events = { On: () => () => {}, Emit: () => {} };
export const Window = { Close: () => {}, Hide: () => {}, SetAlwaysOnTop: () => {} };
export const Application = { Quit: () => {} };
export const Browser = { OpenURL: async url => { window.open(url, '_blank'); } };
export const Dialogs = {
  Question: async options => (window.confirm(options.Message) ? 'Yes' : 'No'),
};
`;

function getFilesRecursively(dir) {
  const files = [];
  if (!fs.existsSync(dir)) return files;
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...getFilesRecursively(fullPath));
    } else if (entry.isFile()) {
      files.push(fullPath);
    }
  }
  return files;
}

function getWatchedSnapshot() {
  const watchedDirs = [frontendRoot, fixturesRoot];
  const allFiles = watchedDirs.flatMap(getFilesRecursively);
  allFiles.sort();
  return allFiles
    .map((file) => {
      try {
        const stat = fs.statSync(file);
        return `${file}|${stat.size}|${stat.mtimeMs}`;
      } catch {
        return "";
      }
    })
    .filter(Boolean)
    .join("\n");
}

function isWithin(parent, child) {
  const rel = path.relative(parent, child);
  return !rel.startsWith("..") && !path.isAbsolute(rel);
}

const server = http.createServer((req, res) => {
  const parsedUrl = new URL(req.url, `http://localhost:${PORT}`);
  const pathname = decodeURIComponent(parsedUrl.pathname);

  if (pathname === "/__live-version") {
    const snapshot = getWatchedSnapshot();
    res.writeHead(200, {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "no-cache, no-store",
    });
    res.end(snapshot);
    return;
  }

  if (pathname === "/wails/runtime.js") {
    res.writeHead(200, {
      "Content-Type": "text/javascript; charset=utf-8",
      "Cache-Control": "no-cache, no-store",
    });
    res.end(WAILS_RUNTIME);
    return;
  }

  if (pathname.startsWith("/fixtures/latest/")) {
    const match = /^([a-z]+)\.json$/.exec(pathname.substring("/fixtures/latest/".length));
    const displayDir = path.join(fixturesRoot, "usage");
    const candidates = match && fs.existsSync(displayDir)
      ? fs.readdirSync(displayDir).filter(f => f === `display_${match[1]}.json`)
      : [];
    if (candidates.length === 0) {
      res.writeHead(404);
      res.end("Not Found");
      return;
    }
    candidates.sort((a, b) => fs.statSync(path.join(displayDir, b)).mtimeMs - fs.statSync(path.join(displayDir, a)).mtimeMs);
    const content = fs.readFileSync(path.join(displayDir, candidates[0]));
    res.writeHead(200, {
      "Content-Type": "application/json; charset=utf-8",
      "Cache-Control": "no-cache, no-store",
    });
    res.end(content);
    return;
  }

  if (pathname.startsWith("/fixtures/")) {
    const rel = pathname.substring("/fixtures/".length);
    const target = path.resolve(fixturesRoot, rel);
    if (!isWithin(fixturesRoot, target) || !fs.existsSync(target) || !fs.statSync(target).isFile()) {
      res.writeHead(404);
      res.end("Not Found");
      return;
    }
    const content = fs.readFileSync(target);
    res.writeHead(200, {
      "Content-Type": "application/json; charset=utf-8",
      "Cache-Control": "no-cache, no-store",
    });
    res.end(content);
    return;
  }

  const rel = pathname === "/" ? "index.html" : pathname.replace(/^\/+/, "");
  const target = path.resolve(frontendRoot, rel);

  if (!isWithin(frontendRoot, target) || !fs.existsSync(target) || !fs.statSync(target).isFile()) {
    res.writeHead(404);
    res.end("Not Found");
    return;
  }

  const ext = path.extname(target).toLowerCase();
  const contentType = MIME_TYPES[ext] || "application/octet-stream";
  const content = fs.readFileSync(target);

  res.writeHead(200, {
    "Content-Type": contentType,
    "Cache-Control": "no-cache, no-store",
  });
  res.end(content);
});

function diagnosePortOwners(port) {
  if (process.platform !== "win32") return;
  try {
    const netstat = child_process.execFileSync("netstat", ["-ano", "-p", "tcp"], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    });
    const lines = netstat.split(/\r?\n/);
    const pids = new Set();
    for (const line of lines) {
      const parts = line.trim().split(/\s+/);
      if (parts.length >= 5 && parts[0].toUpperCase() === "TCP") {
        const localAddr = parts[1];
        const state = parts[3];
        const pid = parts[4];
        if (state === "LISTENING" && (localAddr.endsWith(`:${port}`) || localAddr.endsWith(`[::]:${port}`))) {
          pids.add(pid);
        }
      }
    }
    if (pids.size > 0) {
      for (const pid of pids) {
        if (pid === "4") {
          console.error(`  PID 4 - System (HTTP.sys). Stop the server registered for http://localhost:${port}/. Do not kill PID 4.`);
          continue;
        }
        let name = "unknown";
        try {
          const tasklist = child_process.execFileSync("tasklist", ["/FI", `PID eq ${pid}`, "/FO", "CSV", "/NH"], {
            encoding: "utf8",
            stdio: ["ignore", "pipe", "ignore"],
          });
          const match = tasklist.match(/^"([^"]+)"/);
          if (match) name = match[1];
        } catch { }
        console.error(`  PID ${pid} - ${name}`);
        console.error(`  Stop-Process -Id ${pid}`);
      }
    }
  } catch { }
}

server.on("error", (err) => {
  if (err.code === "EADDRINUSE") {
    console.error(`Port ${PORT} is already in use:`);
    diagnosePortOwners(PORT);
    console.error(`Stop the process using port ${PORT} or set PORT=<other_port> and retry.`);
    process.exit(1);
  } else {
    console.error("Server error:", err);
    process.exit(1);
  }
});

server.listen(PORT, () => {
  console.log(`Live server: http://localhost:${PORT}/?theme=light`);
  console.log(`  Provider states: ?state=login_required (all) or ?codex=not_installed (one)`);
  console.log(
    `  States: connected, not_installed, auth_check_required, login_required, usage_unavailable, temporary_error, unsupported_cli`
  );
  console.log(`Press Ctrl+C to stop.`);
});
