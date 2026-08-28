param(
    [ValidateSet("run", "test", "build")]
    [string]$Task = "build"
)

switch ($Task) {
    "run"   { Start-Process -FilePath "go" -ArgumentList "run ." -WorkingDirectory (Get-Location) -WindowStyle Hidden }
    "test"  { go test ./... }
    "build" { go build -ldflags "-H=windowsgui" -o aigauge.exe . }
}

if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
