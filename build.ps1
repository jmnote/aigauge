param(
    [ValidateSet("run", "test", "logo", "build", "package", "checks", "live-server", "snapshot", "screenshot-light", "screenshot-dark")]
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
    "logo" {
        & (Join-Path $PSScriptRoot "hack\convert-logo.ps1")
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "build" {
        if ([string]::IsNullOrWhiteSpace($Version)) {
            $Version = (Get-Content -LiteralPath (Join-Path $PSScriptRoot "VERSION") -Raw).Trim()
        }
        if ([string]::IsNullOrWhiteSpace($Version)) {
            throw "VERSION must not be empty"
        }
        if (-not $SkipWindowsResources) {
            & $PSCommandPath -Task logo
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
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
    "checks" {
        foreach ($asset in @("frontend\logo.svg", "frontend\logo.png")) {
            $assetPath = Join-Path $PSScriptRoot $asset
            if (-not (Test-Path -LiteralPath $assetPath -PathType Leaf)) {
                throw "Required logo asset was not found: $assetPath"
            }
        }

        $formatOutput = @(gofmt -l main.go internal)
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        if ($formatOutput.Count -gt 0) {
            $formatOutput | ForEach-Object { Write-Error "gofmt required: $_" }
            exit 1
        }

        go test ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

        go vet ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

        git diff --check
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

        & $PSCommandPath -Task package -Architecture $Architecture -Version $Version -MakeAppx $MakeAppx
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

        $checkVersion = $Version
        if ([string]::IsNullOrWhiteSpace($checkVersion)) {
            $checkVersion = (Get-Content -LiteralPath (Join-Path $PSScriptRoot "VERSION") -Raw).Trim()
        }
        $checkVersion = $checkVersion.TrimStart('v', 'V')
        $checkParts = @($checkVersion.Split('.'))
        while ($checkParts.Count -lt 4) { $checkParts += '0' }
        $checkVersion = $checkParts -join '.'
        $packagePath = Join-Path $PSScriptRoot ("dist\aigauge_{0}_{1}.msix" -f $checkVersion, $Architecture)
        $manifestPath = Join-Path $PSScriptRoot ("dist\staging\{0}\AppxManifest.xml" -f $Architecture)
        if (-not (Test-Path -LiteralPath $packagePath -PathType Leaf)) {
            throw "Expected MSIX was not created: $packagePath"
        }
        $stagedManifest = Get-Content -LiteralPath $manifestPath -Raw
        if ($stagedManifest -notmatch ('Version="{0}"' -f [regex]::Escape($checkVersion))) {
            throw "MSIX manifest version does not match VERSION: $checkVersion"
        }
        Write-Output "Checks passed: $packagePath"
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
