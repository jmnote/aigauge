$Port = 8080

$root = Split-Path -Parent $PSScriptRoot
$frontendRoot = (Resolve-Path -LiteralPath (Join-Path $root 'frontend')).Path.TrimEnd([IO.Path]::DirectorySeparatorChar)
$frontendPrefix = $frontendRoot + [IO.Path]::DirectorySeparatorChar
$listener = [System.Net.HttpListener]::new()
$listener.Prefixes.Add("http://localhost:$Port/")

function Get-FrontendSnapshot {
    (Get-ChildItem (Join-Path $root 'frontend') -Recurse -File |
        Sort-Object FullName |
        ForEach-Object { "$($_.FullName)|$($_.Length)|$($_.LastWriteTimeUtc.Ticks)" }) -join "`n"
}

$runtime = @'
globalThis.__AIGAUGE_LIVE__ = true;
const getFixture = () => fetch('/fixtures/sample.json', { cache: 'no-store' }).then(response => response.json());
const theme = new URLSearchParams(location.search).get('theme');
export const Call = {
  ByName: async name => {
    const fixture = await getFixture();
    if (name.endsWith('GetCodexUsage')) return fixture.codex;
    if (name.endsWith('GetAntigravityUsage')) return fixture.antigravity;
    if (name.endsWith('GetClaudeUsage')) return fixture.claude;
    if (name.endsWith('GetThemeOverride')) return ['light', 'dark', 'system'].includes(theme) ? theme : '';
    if (name.endsWith('GetVersion')) return fixture.version;
    if (name.endsWith('SetContentHeight')) return null;
    if (name.endsWith('SetAlwaysOnTop')) return null;
    if (name.endsWith('HideToTray')) return null;
    return null;
  }
};
export const Events = { On: () => () => {} };
export const Window = { Close: () => {}, Hide: () => {}, SetAlwaysOnTop: () => {} };
export const Application = { Quit: () => {} };
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
    Write-Host "Press Ctrl+C to stop."
    while ($listener.IsListening) {
        $contextTask = $listener.GetContextAsync()
        while (-not $contextTask.Wait(250)) { }
        $context = $contextTask.Result
        $path = [Uri]::UnescapeDataString($context.Request.Url.AbsolutePath)
        if ($path -eq '/__live-version') {
            $content = Get-FrontendSnapshot
            $contentType = 'text/plain; charset=utf-8'
        } elseif ($path -eq '/wails/runtime.js') {
            $content = $runtime
            $contentType = 'text/javascript; charset=utf-8'
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
