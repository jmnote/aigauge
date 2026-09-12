param(
    [ValidateSet("run", "kill", "test", "logo", "build", "package", "checks", "clean", "live-server", "screenshot", "screenshot-light", "screenshot-dark", "fixtures", "fixtures-json", "fixtures-go", "ai-backup", "ai-restore")]
    [string]$Task = "build",
    [Alias("Provider", "Target")]
    [string]$Version = "",
    [ValidateSet("x64", "x86", "arm64")]
    [string]$Architecture = "x64",
    [string]$MakeAppx = "",
    [string]$ScreenshotPath = "",
    [switch]$SkipWindowsResources,
    [switch]$ReleaseArtifact
)

. (Join-Path $PSScriptRoot "hack\version.ps1")

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
    "test" {
        go test ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        # The frontend's pure rules (frontend/logic.mjs) - notably which
        # provider states may be counted as failures. node's built-in runner,
        # so this needs no test framework or browser stand-in.
        node --test "frontend/*.test.mjs"
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "logo" {
        & (Join-Path $PSScriptRoot "hack\convert-logo.ps1")
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "build" {
        if ([string]::IsNullOrWhiteSpace($Version)) {
            $Version = "0.0.0"
        }
        if (-not $SkipWindowsResources) {
            & $PSCommandPath -Task logo
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
            & (Join-Path $PSScriptRoot "hack\prepare-windows-resources.ps1") -Version $Version
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
        $binDir = Join-Path $PSScriptRoot "dist\bin"
        if (-not (Test-Path -LiteralPath $binDir)) {
            New-Item -ItemType Directory -Force -Path $binDir | Out-Null
        }
        $outputExe = Join-Path $binDir "aigauge.exe"
        $ldflags = "-H=windowsgui -X github.com/jmnote/aigauge/internal/app.AppVersion=$Version"
        go build -ldflags $ldflags -o $outputExe .
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

        node --test "frontend/*.test.mjs"
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

        $checkVersion = Resolve-Version -Requested $Version
        $artifactSuffix = if ($ReleaseArtifact) { "" } else { "_local" }
        $packagePath = Join-Path $PSScriptRoot ("dist\aigauge_{0}_{1}{2}.msix" -f $checkVersion, $Architecture, $artifactSuffix)
        $manifestPath = Join-Path $PSScriptRoot ("dist\staging\{0}\AppxManifest.xml" -f $Architecture)
        if (-not (Test-Path -LiteralPath $packagePath -PathType Leaf)) {
            throw "Expected MSIX was not created: $packagePath"
        }
        $stagedManifest = Get-Content -LiteralPath $manifestPath -Raw
        if ($stagedManifest -notmatch ('Version="{0}"' -f [regex]::Escape($checkVersion))) {
            throw "MSIX manifest version does not match requested version: $checkVersion"
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
        $rootExe = Join-Path $PSScriptRoot "aigauge.exe"
        if (Test-Path -LiteralPath $rootExe) {
            Remove-Item -LiteralPath $rootExe -Force
        }
        $rootSyso = Join-Path $PSScriptRoot "rsrc_windows_amd64.syso"
        if (Test-Path -LiteralPath $rootSyso) {
            Remove-Item -LiteralPath $rootSyso -Force
        }
        $hackTemp = Join-Path $PSScriptRoot "hack\temp"
        if (Test-Path -LiteralPath $hackTemp) {
            Remove-Item -LiteralPath $hackTemp -Recurse -Force
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
    "ai-backup" {
        # Renames this machine's real Codex/Claude/agy credential and
        # executable files to *.bak, so internal/providers sees exactly what
        # a clean certification device with none of them installed would -
        # letting that first-run "no CLI, no sign-in" state be tested here
        # without a second Windows account or a VM. Reversed by ai-restore.
        # Accepts an optional provider argument: all (default), antigravity, claude, codex.
        $provider = if ($Version) { $Version } else { "all" }
        & (Join-Path $PSScriptRoot "hack\ai-credentials.ps1") -Provider $provider
        exit $LASTEXITCODE
    }
    "ai-restore" {
        $provider = if ($Version) { $Version } else { "all" }
        & (Join-Path $PSScriptRoot "hack\ai-credentials.ps1") -Restore -Provider $provider
        exit $LASTEXITCODE
    }
    "fixtures" {
        foreach ($fixturesTask in @("fixtures-json", "fixtures-go")) {
            & $PSCommandPath -Task $fixturesTask
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
    }
    "fixtures-json" {
        # Fetches real usage data (needs local Codex/Claude sign-in and the
        # `agy` CLI) and writes it to hack/fixtures/sample-*.json.
        Push-Location $PSScriptRoot
        try {
            go run hack/fixtures/gen-json.go
        } finally {
            Pop-Location
        }
        exit $LASTEXITCODE
    }
    "fixtures-go" {
        # Compiles hack/fixtures/sample-*.json into
        # internal/app/fixtures/fixtures.go for the app's sample-data preview.
        # Pure local transform - no accounts or network needed - so unlike
        # fixtures-json this is safe to run in CI or by any contributor.
        Push-Location $PSScriptRoot
        try {
            go run hack/fixtures/gen-go.go
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
        $arguments = @{ Theme = "light"; RenderWaitSeconds = 20 }
        if (-not [string]::IsNullOrWhiteSpace($ScreenshotPath)) { $arguments.OutputPath = $ScreenshotPath }
        & $screenshotScript @arguments
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "screenshot-dark" {
        & $PSCommandPath -Task build -Version $Version
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        $screenshotScript = Join-Path $PSScriptRoot "hack\screenshot.ps1"
        $arguments = @{ Theme = "dark"; RenderWaitSeconds = 20 }
        if (-not [string]::IsNullOrWhiteSpace($ScreenshotPath)) { $arguments.OutputPath = $ScreenshotPath }
        & $screenshotScript @arguments
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
