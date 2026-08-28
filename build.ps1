param(
    [ValidateSet("run", "test", "build")]
    [string]$Task = "build",
    [string]$Version = ""
)

switch ($Task) {
    "run"   { Start-Process -FilePath "go" -ArgumentList "run ." -WorkingDirectory (Get-Location) -WindowStyle Hidden }
    "test"  { go test ./... }
    "build" {
        if ([string]::IsNullOrWhiteSpace($Version)) {
            $Version = (Get-Content -LiteralPath (Join-Path $PSScriptRoot "VERSION") -Raw).Trim()
        }
        $Version = $Version.TrimStart('v')
        if ([string]::IsNullOrWhiteSpace($Version)) {
            throw "VERSION must not be empty"
        }
        $ldflags = "-H=windowsgui -X github.com/jmnote/aigauge/internal/app.AppVersion=$Version"
        go build -ldflags $ldflags -o aigauge.exe .
    }
}

if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
