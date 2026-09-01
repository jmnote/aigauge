param(
    [ValidateSet("run", "kill", "test", "logo", "build", "package", "checks", "clean", "live-server", "screenshot", "screenshot-light", "screenshot-dark", "fixtures")]
    [string]$Task = "build",
    [string]$Version = "",
    [ValidateSet("x64", "x86", "arm64")]
    [string]$Architecture = "x64",
    [string]$MakeAppx = "",
    [string]$ScreenshotPath = "",
    [switch]$SkipWindowsResources,
    [switch]$ReleaseArtifact
)

# Screenshots render whatever's in frontend/fixtures/sample-*.json (the same
# files `.\build.ps1 fixtures` writes and hack/live-server.ps1 serves) rather
# than calling the real provider APIs, so capturing doesn't need a logged-in
# Codex/Claude/Antigravity account or the tens of seconds a live fetch takes.
function Get-ScreenshotFixturesDir {
    $fixturesDir = Join-Path $PSScriptRoot "frontend\fixtures"
    $missing = @("sample-codex.json", "sample-claude.json", "sample-antigravity.json") |
        Where-Object { -not (Test-Path -LiteralPath (Join-Path $fixturesDir $_) -PathType Leaf) }
    if ($missing.Count -gt 0) {
        throw "Missing fixture(s) for screenshots: $($missing -join ', '). Run '.\build.ps1 fixtures' first."
    }
    return $fixturesDir
}

switch ($Task) {
    "run"   { Start-Process -FilePath "go" -ArgumentList "run ." -WorkingDirectory (Get-Location) -WindowStyle Hidden }
    "kill"  {
        # The app enforces a single running instance, so a leftover one from
        # a previous run/build silently blocks a new one from starting.
        $processes = @(Get-Process -Name "aigauge" -ErrorAction SilentlyContinue)
        if ($processes.Count -gt 0) {
            $processes | Stop-Process -Force
            Write-Output ("Killed {0} aigauge.exe process(es)" -f $processes.Count)
        } else {
            Write-Output "No running aigauge.exe process found"
        }
    }
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
        if ($ReleaseArtifact) { $arguments.ReleaseArtifact = $true }
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

        $packageArguments = @{
            Task = "package"
            Architecture = $Architecture
            Version = $Version
            MakeAppx = $MakeAppx
        }
        if ($SkipWindowsResources) { $packageArguments.SkipWindowsResources = $true }
        if ($ReleaseArtifact) { $packageArguments.ReleaseArtifact = $true }
        & $PSCommandPath @packageArguments
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

        $checkVersion = $Version
        if ([string]::IsNullOrWhiteSpace($checkVersion)) {
            $checkVersion = (Get-Content -LiteralPath (Join-Path $PSScriptRoot "VERSION") -Raw).Trim()
        }
        $checkVersion = $checkVersion.TrimStart('v', 'V')
        $checkParts = @($checkVersion.Split('.'))
        while ($checkParts.Count -lt 4) { $checkParts += '0' }
        $checkVersion = $checkParts -join '.'
        $artifactSuffix = if ($ReleaseArtifact) { "" } else { "_local" }
        $packagePath = Join-Path $PSScriptRoot ("dist\aigauge_{0}_{1}{2}.msix" -f $checkVersion, $Architecture, $artifactSuffix)
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
    "clean" {
        $repoRoot = [System.IO.Path]::GetFullPath($PSScriptRoot).TrimEnd([System.IO.Path]::DirectorySeparatorChar)
        $distPath = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "dist"))
        $expectedPrefix = $repoRoot + [System.IO.Path]::DirectorySeparatorChar
        if (-not $distPath.StartsWith($expectedPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to clean dist outside the repository: $distPath"
        }
        if (Test-Path -LiteralPath $distPath) {
            Remove-Item -LiteralPath $distPath -Recurse -Force
            Write-Output "Cleaned build artifacts: $distPath"
        } else {
            Write-Output "No build artifacts to clean: $distPath"
        }
    }
    "live-server" {
        & (Join-Path $PSScriptRoot "hack\live-server.ps1")
        exit $LASTEXITCODE
    }
    "fixtures" {
        Push-Location $PSScriptRoot
        try {
            go run ./hack/gensample
        } finally {
            Pop-Location
        }
        exit $LASTEXITCODE
    }
    "screenshot" {
        foreach ($screenshotTask in @("screenshot-light", "screenshot-dark")) {
            & $PSCommandPath -Task $screenshotTask
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
    }
    "screenshot-light" {
        & $PSCommandPath -Task build -Version $Version
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        $screenshotScript = Join-Path $PSScriptRoot "hack\screenshot.ps1"
        $arguments = @{ Theme = "light"; FixturesDir = (Get-ScreenshotFixturesDir); RenderWaitSeconds = 3 }
        if (-not [string]::IsNullOrWhiteSpace($ScreenshotPath)) { $arguments.OutputPath = $ScreenshotPath }
        & $screenshotScript @arguments
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "screenshot-dark" {
        & $PSCommandPath -Task build -Version $Version
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        $screenshotScript = Join-Path $PSScriptRoot "hack\screenshot.ps1"
        $arguments = @{ Theme = "dark"; FixturesDir = (Get-ScreenshotFixturesDir); RenderWaitSeconds = 3 }
        if (-not [string]::IsNullOrWhiteSpace($ScreenshotPath)) { $arguments.OutputPath = $ScreenshotPath }
        & $screenshotScript @arguments
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
