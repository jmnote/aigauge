param(
    [string]$SvgPath = "",
    [string]$PngPath = "",
    [int]$Size = 1024
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($SvgPath)) { $SvgPath = Join-Path $repo "frontend\logo.svg" }
if ([string]::IsNullOrWhiteSpace($PngPath)) { $PngPath = Join-Path $repo "frontend\logo.png" }

$magick = Get-Command magick.exe -ErrorAction SilentlyContinue
if (-not $magick) {
    throw "ImageMagick was not found. Install ImageMagick and ensure magick.exe is on PATH."
}
if (-not (Test-Path -LiteralPath $SvgPath -PathType Leaf)) { throw "SVG source was not found: $SvgPath" }

$parent = Split-Path -Parent $PngPath
New-Item -ItemType Directory -Force -Path $parent | Out-Null
& $magick.Source $SvgPath -background none -resize ("{0}x{0}" -f $Size) $PngPath
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Output "Created: $PngPath"
