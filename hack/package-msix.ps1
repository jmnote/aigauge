param(
    [ValidateSet("x64", "x86", "arm64")]
    [string]$Architecture = "x64",
    [string]$Version = "",
    [string]$MakeAppx = "",
    [switch]$SkipBuild,
    [switch]$SkipWindowsResources
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot

function Resolve-Version {
    param([string]$Requested)
    if ([string]::IsNullOrWhiteSpace($Requested)) {
        $Requested = (Get-Content -LiteralPath (Join-Path $repo "VERSION") -Raw).Trim()
    }
    $value = $Requested.TrimStart('v', 'V')
    if ($value -notmatch '^\d+\.\d+\.\d+(\.\d+)?$') {
        throw "Version must be semantic numeric version text such as 0.1.1 or 0.1.1.0. Received: $Requested"
    }
    $parts = @($value.Split('.'))
    while ($parts.Count -lt 4) { $parts += '0' }
    return ($parts -join '.')
}

function Find-MakeAppx {
    param([string]$Requested)
    if ($Requested) {
        if (Test-Path -LiteralPath $Requested -PathType Leaf) { return (Resolve-Path -LiteralPath $Requested).Path }
        throw "makeappx.exe was not found at: $Requested"
    }
    $command = Get-Command makeappx.exe -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    $roots = @(
        "${env:ProgramFiles(x86)}\Windows Kits\10\bin",
        "$env:ProgramFiles\Windows Kits\10\bin"
    ) | Where-Object { $_ -and (Test-Path $_) }
    $candidate = Get-ChildItem -Path $roots -Filter makeappx.exe -File -Recurse -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -match '\\(x64|x86)\\makeappx\.exe$' } |
        Sort-Object FullName -Descending | Select-Object -First 1
    if ($candidate) { return $candidate.FullName }
    throw "makeappx.exe was not found. Install the Windows 10/11 SDK (App Certification Kit / MSIX tools) or pass -MakeAppx C:\\path\\makeappx.exe."
}

function New-IconAsset {
    param([System.Drawing.Image]$Source, [string]$Path, [int]$Size)
    $bitmap = New-Object System.Drawing.Bitmap($Size, $Size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceCopy
    $graphics.Clear([System.Drawing.Color]::Transparent)
    $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceOver
    $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $graphics.DrawImage($Source, 0, 0, $Size, $Size)
    $bitmap.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)
    $graphics.Dispose()
    $bitmap.Dispose()
}

$appVersion = $Version
if ([string]::IsNullOrWhiteSpace($appVersion)) {
    $appVersion = (Get-Content -LiteralPath (Join-Path $repo "VERSION") -Raw).Trim()
}
$msixVersion = Resolve-Version $appVersion
$sourceExe = Join-Path $repo "aigauge.exe"
if (-not $SkipBuild) {
    $buildArguments = @{ Task = "build"; Version = $appVersion }
    if ($SkipWindowsResources) { $buildArguments.SkipWindowsResources = $true }
    & (Join-Path $repo "build.ps1") @buildArguments
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
if (-not (Test-Path -LiteralPath $sourceExe -PathType Leaf)) { throw "Build output not found: $sourceExe" }

$staging = Join-Path $repo "dist\staging\$Architecture"
$assets = Join-Path $staging "Assets"
$dist = Join-Path $repo "dist"
New-Item -ItemType Directory -Force -Path $assets, $dist | Out-Null
if (Test-Path $staging) { Get-ChildItem -LiteralPath $staging -Force | Remove-Item -Recurse -Force }
New-Item -ItemType Directory -Force -Path $assets | Out-Null

Copy-Item -LiteralPath $sourceExe -Destination (Join-Path $staging "aigauge.exe")
Copy-Item -LiteralPath (Join-Path $repo "LICENSE") -Destination (Join-Path $staging "LICENSE")
$manifest = Get-Content -LiteralPath (Join-Path $repo "Package.appxmanifest") -Raw
$versionReplacement = '${1}' + $msixVersion + '${2}'
$manifest = $manifest -replace '(?s)(<Identity\b.*?\bVersion=")[^"]+(")', $versionReplacement
$manifest = $manifest -replace 'ProcessorArchitecture="[^"]+"', ('ProcessorArchitecture="{0}"' -f $Architecture)
Set-Content -LiteralPath (Join-Path $staging "AppxManifest.xml") -Value $manifest -Encoding utf8

Add-Type -AssemblyName System.Drawing
$icon = [System.Drawing.Image]::FromFile((Join-Path $repo "frontend\logo.png"))
try {
    foreach ($asset in @(@("Square44x44Logo.png", 44), @("Square150x150Logo.png", 150), @("StoreLogo.png", 50), @("Square310x310Logo.png", 310))) {
        New-IconAsset -Source $icon -Path (Join-Path $assets $asset[0]) -Size $asset[1]
    }
} finally { $icon.Dispose() }

$makeAppxPath = Find-MakeAppx $MakeAppx
$output = Join-Path $dist ("aigauge_{0}_{1}.msix" -f $msixVersion, $Architecture)
if (Test-Path $output) { Remove-Item -LiteralPath $output -Force }
& $makeAppxPath pack /d $staging /p $output /o
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Output "Created: $output"
