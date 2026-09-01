param(
    [string]$Executable = "",
    [string]$OutputPath = "",
    [int]$WaitSeconds = 15,
    [int]$RenderWaitSeconds = 20,
    [ValidateSet("light", "dark", "system")]
    [string]$Theme = "light",
    # When set, launches the app with --fixtures=<dir> so it renders the given
    # directory's saved sample-*.json fixtures (see hack/gensample) instead of
    # calling the real Codex/Claude/Antigravity APIs - no live accounts or
    # network round-trips needed, and $RenderWaitSeconds can be turned way
    # down since there's no real fetch to wait on.
    [string]$FixturesDir = ""
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($Executable)) { $Executable = Join-Path $repo "aigauge.exe" }
if ([string]::IsNullOrWhiteSpace($OutputPath)) { $OutputPath = Join-Path $repo "docs\screenshots\aigauge-native-$Theme.png" }
if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) { throw "Executable not found: $Executable" }

$outputDirectory = Split-Path -Parent $OutputPath
New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

Add-Type -AssemblyName System.Drawing
Add-Type -ReferencedAssemblies System.Drawing @'
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;

public static class WindowCapture {
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }

    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);

    [DllImport("user32.dll")]
    public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdcBlt, uint flags);

    [DllImport("user32.dll")]
    public static extern bool SetProcessDPIAware();

    public static void Capture(IntPtr handle, string path) {
        SetProcessDPIAware();
        RECT rect;
        if (!GetWindowRect(handle, out rect)) throw new InvalidOperationException("Could not get window bounds.");
        int width = rect.Right - rect.Left;
        int height = rect.Bottom - rect.Top;
        if (width <= 0 || height <= 0) throw new InvalidOperationException("Window has invalid bounds.");

        using (var bitmap = new Bitmap(width, height, PixelFormat.Format32bppArgb)) {
            using (var graphics = Graphics.FromImage(bitmap)) {
                IntPtr hdc = graphics.GetHdc();
                try {
                    if (!PrintWindow(handle, hdc, 2)) throw new InvalidOperationException("PrintWindow failed.");
                } finally { graphics.ReleaseHdc(hdc); }
            }
            bitmap.Save(path, ImageFormat.Png);
        }
    }
}
'@

function New-RoundedRectanglePath {
    param([System.Drawing.Rectangle]$Rectangle, [int]$Radius)
    $diameter = $Radius * 2
    $path = New-Object System.Drawing.Drawing2D.GraphicsPath
    $path.AddArc($Rectangle.X, $Rectangle.Y, $diameter, $diameter, 180, 90)
    $path.AddArc($Rectangle.Right - $diameter, $Rectangle.Y, $diameter, $diameter, 270, 90)
    $path.AddArc($Rectangle.Right - $diameter, $Rectangle.Bottom - $diameter, $diameter, $diameter, 0, 90)
    $path.AddArc($Rectangle.X, $Rectangle.Bottom - $diameter, $diameter, $diameter, 90, 90)
    $path.CloseFigure()
    return $path
}

function Add-RoundedShadow {
    param([string]$SourcePath, [string]$DestinationPath, [int]$Radius = 5, [int]$Padding = 24)
    $source = [System.Drawing.Image]::FromFile($SourcePath)
    try {
        $canvas = New-Object System.Drawing.Bitmap(
            ($source.Width + ($Padding * 2)),
            ($source.Height + ($Padding * 2)),
            [System.Drawing.Imaging.PixelFormat]::Format32bppArgb
        )
        try {
            $graphics = [System.Drawing.Graphics]::FromImage($canvas)
            try {
                $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
                $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceOver
                $windowRect = [System.Drawing.Rectangle]::new($Padding, $Padding, $source.Width, $source.Height)

                for ($offset = 10; $offset -ge 2; $offset -= 2) {
                    $shadowRect = [System.Drawing.Rectangle]::new(
                        ($Padding - [int]($offset / 2)),
                        ($Padding + $offset),
                        $source.Width + $offset,
                        $source.Height + $offset
                    )
                    $path = New-RoundedRectanglePath -Rectangle $shadowRect -Radius ($Radius + [int]($offset / 3))
                    $brush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(10, 0, 0, 0))
                    try { $graphics.FillPath($brush, $path) } finally { $brush.Dispose(); $path.Dispose() }
                }

                $clip = New-RoundedRectanglePath -Rectangle $windowRect -Radius $Radius
                try {
                    $graphics.SetClip($clip)
                    $graphics.DrawImage($source, $windowRect)
                } finally { $clip.Dispose() }
            } finally { $graphics.Dispose() }
            $canvas.Save($DestinationPath, [System.Drawing.Imaging.ImageFormat]::Png)
        } finally { $canvas.Dispose() }
    } finally { $source.Dispose() }
}

$processArgs = @("--theme=$Theme")
if (-not [string]::IsNullOrWhiteSpace($FixturesDir)) { $processArgs += "--fixtures=$FixturesDir" }
$process = Start-Process -FilePath $Executable -ArgumentList $processArgs -PassThru
try {
    $deadline = (Get-Date).AddSeconds($WaitSeconds)
    do {
        Start-Sleep -Milliseconds 250
        $process.Refresh()
        $handle = $process.MainWindowHandle
    } while ($handle -eq 0 -and (Get-Date) -lt $deadline)

    if ($handle -eq 0) { throw "AI Gauge window was not found within $WaitSeconds seconds." }
    Start-Sleep -Seconds $RenderWaitSeconds
    $rawPath = Join-Path $env:TEMP ("aigauge-window-" + [System.IO.Path]::GetRandomFileName())
    try {
        [WindowCapture]::Capture($handle, $rawPath)
        Add-RoundedShadow -SourcePath $rawPath -DestinationPath $OutputPath
    } finally {
        if (Test-Path -LiteralPath $rawPath) { [System.IO.File]::Delete($rawPath) }
    }
    Write-Output "Created: $OutputPath"
} finally {
    if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
}
