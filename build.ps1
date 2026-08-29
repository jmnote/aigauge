param(
    [ValidateSet("run", "test", "build", "package", "live-server", "snapshot", "screenshot-light", "screenshot-dark")]
    [string]$Task = "build",
    [string]$Version = "",
    [ValidateSet("x64", "x86", "arm64")]
    [string]$Architecture = "x64",
    [string]$MakeAppx = "",
    [string]$ScreenshotPath = "",
    [switch]$SkipWindowsResources
)

switch ($Task) {
    "run"   { Start-Process -FilePath "go" -ArgumentList "run ." -WorkingDirectory (Get-Location) -WindowStyle Hidden }
    "test"  { go test ./... }
    "build" {
        if ([string]::IsNullOrWhiteSpace($Version)) {
            $Version = (Get-Content -LiteralPath (Join-Path $PSScriptRoot "VERSION") -Raw).Trim()
        }
        if ([string]::IsNullOrWhiteSpace($Version)) {
            throw "VERSION must not be empty"
        }
        if (-not $SkipWindowsResources) {
            & (Join-Path $PSScriptRoot "hack\prepare-windows-resources.ps1") -Version $Version
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
        $ldflags = "-H=windowsgui -X github.com/jmnote/aigauge/internal/app.AppVersion=$Version"
        go build -ldflags $ldflags -o aigauge.exe .
    }
    "package" {
        $packageScript = Join-Path $PSScriptRoot "hack\package-msix.ps1"
        $arguments = @{ Version = $Version; Architecture = $Architecture }
        if (-not [string]::IsNullOrWhiteSpace($MakeAppx)) { $arguments.MakeAppx = $MakeAppx }
        if ($SkipWindowsResources) { $arguments.SkipWindowsResources = $true }
        & $packageScript @arguments
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "live-server" {
        & (Join-Path $PSScriptRoot "hack\live-server.ps1")
        exit $LASTEXITCODE
    }
    "snapshot" {
        foreach ($snapshotTask in @("screenshot-light", "screenshot-dark")) {
            & $PSCommandPath -Task $snapshotTask
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
    }
    "screenshot-light" {
        & $PSCommandPath -Task build -Version $Version
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        $screenshotScript = Join-Path $PSScriptRoot "hack\capture-window.ps1"
        $arguments = @{ Theme = "light" }
        if (-not [string]::IsNullOrWhiteSpace($ScreenshotPath)) { $arguments.OutputPath = $ScreenshotPath }
        & $screenshotScript @arguments
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "screenshot-dark" {
        & $PSCommandPath -Task build -Version $Version
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        $screenshotScript = Join-Path $PSScriptRoot "hack\capture-window.ps1"
        $arguments = @{ Theme = "dark" }
        if (-not [string]::IsNullOrWhiteSpace($ScreenshotPath)) { $arguments.OutputPath = $ScreenshotPath }
        & $screenshotScript @arguments
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
