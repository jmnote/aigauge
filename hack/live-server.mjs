import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
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
// returns - no combined/wrapper file - so it doubles as a raw per-provider
// snapshot (see hack/fixtures/gen-samples.go) and the live-server fixture with no
// conversion step between the two.
const providers = {
  Codex: 'samples/sample-codex.json',
  Claude: 'samples/sample-claude.json',
  Antigravity: 'samples/sample-antigravity.json',
};

// Every provider state the real app can show, reproducible here with no CLI,
// no account and no network:
//
//   /?state=login_required                        all three cards at once
//   /?codex=not_installed&claude=connected       one provider at a time
//   /?view=sample                                open in the sample preview
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
  ByName: async name => {
    for (const key of Object.keys(providers)) {
      if (name.endsWith(\`GetSample\${key}Usage\`)) return fixture(key);
      if (name.endsWith(\`Diagnose\${key}\`)) return diagnosisFor(key);
      if (name.endsWith(\`Get\${key}Usage\`)) return usageFor(key);
    }
    if (name.endsWith('GetThemeOverride')) return ['light', 'dark', 'system'].includes(theme) ? theme : '';
    if (name.endsWith('GetVersion')) return 'vDEV';
    if (name.endsWith('SetContentHeight')) return null;
    if (name.endsWith('SetWindowWidth')) return null;
    if (name.endsWith('SetAlwaysOnTop')) return null;
    if (name.endsWith('HideToTray')) return null;
    return null;
  }
};
export const Events = { On: () => () => {} };
export const Window = { Close: () => {}, Hide: () => {}, SetAlwaysOnTop: () => {} };
export const Application = { Quit: () => {} };
export const Browser = { OpenURL: async url => { window.open(url, '_blank'); } };
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

server.on("error", (err) => {
  if (err.code === "EADDRINUSE") {
    console.error(`Port ${PORT} is already in use.`);
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
  console.log(`  Sample preview: ?view=sample`);
  console.log(`Press Ctrl+C to stop.`);
});

