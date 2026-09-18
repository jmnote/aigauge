param(
    [ValidateSet("run", "kill", "test", "logo", "build", "package", "checks", "clean", "live-server", "screenshot", "screenshot-light", "screenshot-dark", "fixtures-usage", "fixtures-tokens", "submission-snapshot", "submission-validate")]
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

function Ensure-HackNpm([string]$PackageName) {
    $node = Get-Command node -ErrorAction SilentlyContinue
    if (-not $node) {
        throw "Node.js is required. Install Node.js to continue."
    }
    $hackDir = Join-Path $PSScriptRoot "hack"
    $modulePath = if ($PackageName) { Join-Path $hackDir "node_modules\$PackageName" } else { Join-Path $hackDir "node_modules" }
    if (-not (Test-Path -LiteralPath $modulePath -PathType Container)) {
        $npm = Get-Command npm -ErrorAction SilentlyContinue
        if (-not $npm) {
            throw "npm was not found. Install Node.js/npm to continue."
        }
        Push-Location $hackDir
        try {
            & $npm.Source ci --ignore-scripts --no-audit --no-fund
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        } finally {
            Pop-Location
        }
    }
}

function Resolve-Version {
    param([string]$Requested = "0.0.0")
    $res = & node (Join-Path $PSScriptRoot "hack\package.mjs") version --version $Requested
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    return $res.Trim()
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
        Ensure-HackNpm "@resvg/resvg-js"
        & node (Join-Path $PSScriptRoot "hack\package.mjs") logo
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "build" {
        if ([string]::IsNullOrWhiteSpace($Version)) {
            $Version = "0.0.0"
        }
        $Version = Resolve-Version -Requested $Version
        if (-not $SkipWindowsResources) {
            & $PSCommandPath -Task logo
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
            & node (Join-Path $PSScriptRoot "hack\package.mjs") winres --version $Version
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
        $binDir = Join-Path $PSScriptRoot "dist\bin"
        if (-not (Test-Path -LiteralPath $binDir)) {
            New-Item -ItemType Directory -Force -Path $binDir | Out-Null
        }
        $outputExe = Join-Path $binDir "aigauge.exe"
        $ldflags = "-H=windowsgui -X github.com/jmnote/aigauge/internal/app.AppVersion=$Version"
        go build -ldflags $ldflags -o $outputExe .
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "package" {
        Ensure-HackNpm "@resvg/resvg-js"
        $buildArgs = @{
            Task = "build"
            Version = $Version
        }
        if ($SkipWindowsResources) { $buildArgs.SkipWindowsResources = $true }
        & $PSCommandPath @buildArgs
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

        $msixArgs = @((Join-Path $PSScriptRoot "hack\package.mjs"), "msix", "--version", $Version, "--arch", $Architecture)
        if (-not [string]::IsNullOrWhiteSpace($MakeAppx)) { $msixArgs += @("--makeappx", $MakeAppx) }
        if ($ReleaseArtifact) { $msixArgs += "--release" }
        & node @msixArgs
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    "checks" {
        foreach ($asset in @("frontend\images\logo.svg", "frontend\images\logo.png")) {
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
        if (Test-Path -LiteralPath $distPath) {
            Remove-Item -LiteralPath $distPath -Recurse -Force
            Write-Output "Cleaned build artifacts: $distPath"
        } else {
            Write-Output "No build artifacts to clean: $distPath"
        }
    }
    "live-server" {
        & node (Join-Path $PSScriptRoot "hack\live-server.mjs")
        exit $LASTEXITCODE
    }
    "fixtures-usage" {
        # Captures a usage snapshot using AI Gauge's own stored credentials
        # (one API call per provider) into hack/fixtures/usage/ as both the
        # API's raw response and the converted DisplayUsage response.
        # Needs a connected provider
        # instance of the requested type. Accepts an optional provider
        # argument via -Version: all (default), codex, claude, antigravity.
        $target = if ($Version) { $Version } else { "all" }
        Push-Location $PSScriptRoot
        try {
            go run hack/fixtures/fixtures.go $target
        } finally {
            Pop-Location
        }
        exit $LASTEXITCODE
    }
    "fixtures-tokens" {
        # Fetches/extracts token samples for each provider into hack/fixtures/tokens/.
        # Accepts an optional provider argument via -Version: all (default), codex, claude.
        # Antigravity is not covered - AI Gauge holds no OAuth token of its own for it.
        $target = if ($Version) { $Version } else { "all" }
        Push-Location $PSScriptRoot
        try {
            go run hack/fixtures/fixtures.go tokens $target
        } finally {
            Pop-Location
        }
        exit $LASTEXITCODE
    }
    "submission-snapshot" {
        Ensure-HackNpm "yaml"
        & node (Join-Path $PSScriptRoot "hack\msstore\submission.mjs") snapshot
        exit $LASTEXITCODE
    }
    "submission-validate" {
        Ensure-HackNpm "yaml"
        & node (Join-Path $PSScriptRoot "hack\msstore\submission.mjs") validate
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
