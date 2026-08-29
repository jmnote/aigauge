$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$sourcePath = Join-Path $repo "frontend\logo.svg"
$outputPath = Join-Path $repo "frontend\logo.png"

# Update this value after changing frontend/logo.svg. Get the new value with:
# (Get-FileHash frontend/logo.svg -Algorithm SHA256).Hash.ToLowerInvariant()
$expectedSourceHash = "85e14c2328a97674fdde7c896155180a24ccdf29b5f4a46d23938d44de649e59"
$sourceHash = (Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash.ToLowerInvariant()

if ((Test-Path -LiteralPath $outputPath -PathType Leaf) -and $expectedSourceHash -eq $sourceHash) {
    Write-Output "Logo PNG is up to date."
    exit 0
}

$node = Get-Command node -ErrorAction SilentlyContinue
if (-not $node) {
    throw "Node.js was not found. Install Node.js to regenerate frontend/logo.png."
}

$toolDir = $PSScriptRoot
$modulePath = Join-Path $toolDir "node_modules\@resvg\resvg-js"
if (-not (Test-Path -LiteralPath $modulePath -PathType Container)) {
    $npm = Get-Command npm -ErrorAction SilentlyContinue
    if (-not $npm) {
        throw "npm was not found. Install Node.js/npm to regenerate frontend/logo.png."
    }
    Push-Location $toolDir
    try {
        & $npm.Source ci --ignore-scripts --no-audit --no-fund
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }
}

& $node.Source (Join-Path $PSScriptRoot "convert-logo.mjs")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
