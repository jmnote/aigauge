param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (Get-Content -LiteralPath (Join-Path $repo "VERSION") -Raw).Trim()
}
$numericVersion = $Version.TrimStart('v')
if ($numericVersion -notmatch '^\d+\.\d+\.\d+(\.\d+)?$') {
    throw "Version must contain three or four numeric components: $Version"
}
if ($numericVersion.Split('.').Count -eq 3) { $numericVersion += ".0" }

$winres = Get-Command go-winres -ErrorAction SilentlyContinue
if (-not $winres) {
    $goBin = (go env GOPATH).Trim()
    $candidate = Join-Path $goBin "bin\go-winres.exe"
    if (Test-Path -LiteralPath $candidate -PathType Leaf) { $winres = Get-Command $candidate }
}
if (-not $winres) {
    throw "go-winres was not found. Install it with: go install github.com/tc-hib/go-winres@v0.3.3"
}

& $winres.Source simply --arch amd64 --out (Join-Path $repo "rsrc") --manifest gui `
    --icon (Join-Path $repo "frontend\logo.png") `
    --product-name "AI Gauge" `
    --file-description "AI Gauge" `
    --original-filename "aigauge.exe" `
    --product-version $numericVersion `
    --file-version $numericVersion
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
