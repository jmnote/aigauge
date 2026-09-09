$Port = 8080

$root = Split-Path -Parent $PSScriptRoot
$frontendRoot = (Resolve-Path -LiteralPath (Join-Path $root 'frontend')).Path.TrimEnd([IO.Path]::DirectorySeparatorChar)
$frontendPrefix = $frontendRoot + [IO.Path]::DirectorySeparatorChar
# sample-*.json lives under hack/fixtures (hack/fixtures/gen-json.go writes it there;
# hack/fixtures/gen-go.go separately compiles it into
# internal/app/fixtures/fixtures.go for the app's sample-data preview) rather
# than under frontend/, so it needs its own prefix/route below instead of
# falling out of the generic frontend$path mapping.
$fixturesRoot = (Resolve-Path -LiteralPath (Join-Path $root 'hack/fixtures')).Path.TrimEnd([IO.Path]::DirectorySeparatorChar)
$fixturesPrefix = $fixturesRoot + [IO.Path]::DirectorySeparatorChar
$listener = [System.Net.HttpListener]::new()
$listener.Prefixes.Add("http://localhost:$Port/")

function Get-WatchedSnapshot {
    (Get-ChildItem @((Join-Path $root 'frontend'), $fixturesRoot) -Recurse -File |
        Sort-Object FullName |
        ForEach-Object { "$($_.FullName)|$($_.Length)|$($_.LastWriteTimeUtc.Ticks)" }) -join "`n"
}

$runtime = @'
globalThis.__AIGAUGE_LIVE__ = true;
const params = new URLSearchParams(location.search);
const theme = params.get('theme');

// Each provider's fixture file holds exactly what its Wails RPC method
// returns - no combined/wrapper file - so it doubles as a raw per-provider
// snapshot (see hack/fixtures/gen-json.go) and the live-server fixture with no
// conversion step between the two.
const providers = {
  Codex: 'sample-codex.json',
  Claude: 'sample-claude.json',
  Antigravity: 'sample-antigravity.json',
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
  const response = await fetch(`/fixtures/${providers[key]}`, { cache: 'no-store' });
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
      if (name.endsWith(`GetSample${key}Usage`)) return fixture(key);
      if (name.endsWith(`Diagnose${key}`)) return diagnosisFor(key);
      if (name.endsWith(`Get${key}Usage`)) return usageFor(key);
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
'@

try {
    $portInUse = $false
    $probe = [System.Net.Sockets.TcpClient]::new()
    try {
        $connection = $probe.ConnectAsync('localhost', $Port)
        if ($connection.Wait(500) -and $probe.Connected) { $portInUse = $true }
    } catch {
        $portInUse = $false
    } finally {
        $probe.Dispose()
    }
    if ($portInUse) {
        $owners = @(Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
            Select-Object -ExpandProperty OwningProcess -Unique)
        Write-Host "Port $Port is already in use:" -ForegroundColor Yellow
        foreach ($owner in $owners) {
            if ($owner -eq 4) {
                $serviceState = netsh http show servicestate | Out-String
                $pattern = "ID:\s+(\d+), image:\s*([^\r\n]+)\r?\n\s*Registered URLs:\s*\r?\n\s*HTTP://LOCALHOST:$Port/"
                $registration = [regex]::Match($serviceState, $pattern, [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
                if ($registration.Success) {
                    $registeredPid = $registration.Groups[1].Value
                    $registeredImage = $registration.Groups[2].Value.Trim()
                    Write-Host "  HTTP.sys -> PID $registeredPid - $registeredImage" -ForegroundColor Yellow
                    Write-Host "  Stop-Process -Id $registeredPid" -ForegroundColor Yellow
                } else {
                    Write-Host "  PID 4 - System (HTTP.sys). Stop the server registered for http://localhost:$Port/. Do not kill PID 4." -ForegroundColor Yellow
                }
                continue
            }
            $process = Get-Process -Id $owner -ErrorAction SilentlyContinue
            $name = if ($process) { $process.ProcessName } else { 'unknown' }
            Write-Host "  PID $owner - $name" -ForegroundColor Yellow
            Write-Host "  Stop-Process -Id $owner" -ForegroundColor Yellow
        }
        exit 1
    }
    try {
        $listener.Start()
    } catch [System.Net.HttpListenerException] {
        Write-Host "Port $Port became unavailable while starting. Stop the existing local server and try again." -ForegroundColor Yellow
        exit 1
    }
    Write-Host "Live server: http://localhost:$Port/?theme=light"
    Write-Host "  Provider states: ?state=login_required (all) or ?codex=not_installed (one)"
    Write-Host "  States: connected, not_installed, auth_check_required, login_required, usage_unavailable, temporary_error, unsupported_cli"
    Write-Host "  Sample preview: ?view=sample"
    Write-Host "Press Ctrl+C to stop."
    while ($listener.IsListening) {
        $contextTask = $listener.GetContextAsync()
        while (-not $contextTask.Wait(250)) { }
        $context = $contextTask.Result
        $path = [Uri]::UnescapeDataString($context.Request.Url.AbsolutePath)
        if ($path -eq '/__live-version') {
            $content = Get-WatchedSnapshot
            $contentType = 'text/plain; charset=utf-8'
        } elseif ($path -eq '/wails/runtime.js') {
            $content = $runtime
            $contentType = 'text/javascript; charset=utf-8'
        } elseif ($path.StartsWith('/fixtures/')) {
            $file = Join-Path $fixturesRoot $path.Substring('/fixtures/'.Length)
            if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
                $context.Response.StatusCode = 404
                $context.Response.Close()
                continue
            }
            $resolvedFile = (Resolve-Path -LiteralPath $file).Path
            if (-not $resolvedFile.StartsWith($fixturesPrefix, [StringComparison]::OrdinalIgnoreCase)) {
                $context.Response.StatusCode = 404
                $context.Response.Close()
                continue
            }
            $content = [IO.File]::ReadAllBytes($resolvedFile)
            $contentType = 'application/json; charset=utf-8'
        } else {
            $relative = if ($path -eq '/') { 'frontend/index.html' } else { "frontend$path" }
            $file = Join-Path $root $relative.TrimStart('/')
            if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
                $context.Response.StatusCode = 404
                $context.Response.Close()
                continue
            }
            $resolvedFile = (Resolve-Path -LiteralPath $file).Path
            if (-not $resolvedFile.StartsWith($frontendPrefix, [StringComparison]::OrdinalIgnoreCase)) {
                $context.Response.StatusCode = 404
                $context.Response.Close()
                continue
            }
            $content = [IO.File]::ReadAllBytes($resolvedFile)
            $contentType = switch ([IO.Path]::GetExtension($file).ToLowerInvariant()) {
                '.html' { 'text/html; charset=utf-8' }
                '.js' { 'text/javascript; charset=utf-8' }
                '.mjs' { 'text/javascript; charset=utf-8' }
                '.css' { 'text/css; charset=utf-8' }
                '.json' { 'application/json; charset=utf-8' }
                '.png' { 'image/png' }
                '.svg' { 'image/svg+xml' }
                default { 'application/octet-stream' }
            }
        }
        if ($content -is [string]) { $content = [Text.Encoding]::UTF8.GetBytes($content) }
        $context.Response.ContentType = $contentType
        $context.Response.ContentLength64 = $content.Length
        $context.Response.OutputStream.Write($content, 0, $content.Length)
        $context.Response.Close()
    }
} finally {
    $listener.Stop()
    $listener.Close()
}
