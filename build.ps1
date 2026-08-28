param(
    [ValidateSet("run", "test", "build")]
    [string]$Task = "build",
    [string]$Version = "dev"
)

switch ($Task) {
    "run"   { Start-Process -FilePath "go" -ArgumentList "run ." -WorkingDirectory (Get-Location) -WindowStyle Hidden }
    "test"  { go test ./... }
    "build" {
        $ldflags = "-H=windowsgui -X main.appVersion=$Version"
        go build -ldflags $ldflags -o aigauge.exe .
    }
}

if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
