param(
    [switch]$Restore,
    [ValidateSet("all", "antigravity", "claude", "codex", "agy")]
    [string]$Provider = "all"
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot

if ($Provider -eq "agy") { $Provider = "antigravity" }

function Get-ProviderFromLabel([string]$label) {
    if ($label -match "(?i)claude") { return "claude" }
    if ($label -match "(?i)codex") { return "codex" }
    if ($label -match "(?i)agy|antigravity") { return "antigravity" }
    return "unknown"
}

# Where AI Gauge itself looks for each provider - kept in sync with
# internal/providers/claude.go (findClaudeCredentials), codex.go
# (findCodexCredentials), and antigravity_windows.go
# (antigravityFallbackPath). Renaming these to *.bak is what turns this
# machine into the "no CLI installed, no local sign-in" state a clean
# certification device starts from, without needing a second Windows
# account or a VM: internal_providers reads none of them, so every
# provider reports not_installed exactly as it would on that clean device.
function Get-AiBackupTargets {
    param([string]$TargetProvider = "all")

    $targets = @()

    if ($TargetProvider -in @("all", "claude")) {
        $targets += @{ Provider = "claude"; Label = "Claude credentials"; Path = (Join-Path $env:USERPROFILE ".claude\.credentials.json") }
        $claudeCmd = Get-Command "claude" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($claudeCmd) {
            $targets += @{ Provider = "claude"; Label = "claude executable"; Path = $claudeCmd.Source }
        }
    }

    if ($TargetProvider -in @("all", "codex")) {
        $targets += @{ Provider = "codex"; Label = "Codex credentials"; Path = (Join-Path $env:USERPROFILE ".codex\auth.json") }
        $codexCmd = Get-Command "codex" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($codexCmd) {
            $targets += @{ Provider = "codex"; Label = "codex executable"; Path = $codexCmd.Source }
        }
    }

    if ($TargetProvider -in @("all", "antigravity")) {
        $agyCmd = Get-Command "agy" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($agyCmd) {
            $targets += @{ Provider = "antigravity"; Label = "agy executable"; Path = $agyCmd.Source }
        }
        $agyFallback = Join-Path $env:LOCALAPPDATA "agy\bin\agy.exe"
        if ((Test-Path -LiteralPath $agyFallback -PathType Leaf) -and
            -not ($targets | Where-Object { $_.Path -eq $agyFallback })) {
            $targets += @{ Provider = "antigravity"; Label = "agy executable (fallback path)"; Path = $agyFallback }
        }
    }

    return $targets
}

# Local-only bookkeeping (gitignored) of exactly what a backup moved, so
# restore does not have to re-derive paths that backup can no longer see.
$manifestPath = Join-Path $repo ".ai-credentials-backup.local.json"

function Invoke-Backup {
    param([string]$TargetProvider = "all")

    $existingMoved = @()
    if (Test-Path -LiteralPath $manifestPath -PathType Leaf) {
        $raw = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
        $existingMoved = @($raw | ForEach-Object { $_ })
    }

    if ($existingMoved.Count -gt 0) {
        $alreadyBackedUp = $existingMoved | Where-Object {
            $prov = if ($_.Provider) { $_.Provider } else { Get-ProviderFromLabel $_.Label }
            $TargetProvider -eq "all" -or $prov -eq $TargetProvider
        }
        if ($alreadyBackedUp) {
            $provLabel = if ($TargetProvider -eq "all") { "Some or all providers are" } else { "Provider '$TargetProvider' is" }
            Write-Warning "$provLabel already backed up in $manifestPath. Run '.\build.ps1 ai-restore $TargetProvider' first before backing up again."
            exit 1
        }
    }

    $targets = Get-AiBackupTargets -TargetProvider $TargetProvider
    $moved = @()
    foreach ($target in $targets) {
        $livePath = $target.Path
        if (-not (Test-Path -LiteralPath $livePath -PathType Leaf)) { continue }
        $backupPath = "$livePath.bak"
        if (Test-Path -LiteralPath $backupPath -PathType Leaf) {
            Write-Warning "Skipping $($target.Label): $backupPath already exists. Not touching it."
            continue
        }
        try {
            Move-Item -LiteralPath $livePath -Destination $backupPath
            Write-Output "Backed up: $($target.Label) -> $backupPath"
            $moved += @{
                Provider   = $target.Provider
                Label      = $target.Label
                LivePath   = $livePath
                BackupPath = $backupPath
            }
        } catch {
            Write-Warning "Could not back up $($target.Label) ($livePath): $_. Still in use by a running process?"
        }
    }

    if ($moved.Count -eq 0) {
        Write-Output "Nothing to back up for '$TargetProvider' - no live credential or executable files were found."
        return
    }

    $allMoved = @($existingMoved) + @($moved)
    $allMoved | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Write-Output ""
    $restoreHint = if ($TargetProvider -eq "all") { ".\build.ps1 ai-restore" } else { ".\build.ps1 ai-restore $TargetProvider" }
    Write-Output "Backed up $($moved.Count) item(s) for '$TargetProvider'. Run '$restoreHint' when done testing."
}

function Invoke-Restore {
    param([string]$TargetProvider = "all")

    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        Write-Output "Nothing to restore - no backup manifest found at $manifestPath."
        return
    }

    $raw = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    $allMoved = @($raw | ForEach-Object { $_ })

    $toRestore = @()
    $remaining = @()
    foreach ($item in $allMoved) {
        $prov = if ($item.Provider) { $item.Provider } else { Get-ProviderFromLabel $item.Label }
        if ($TargetProvider -eq "all" -or $prov -eq $TargetProvider) {
            $toRestore += $item
        } else {
            $remaining += $item
        }
    }

    if ($toRestore.Count -eq 0) {
        Write-Output "Nothing to restore - no backed-up items found for '$TargetProvider' in $manifestPath."
        return
    }

    $allRestored = $true
    $notRestored = @()
    foreach ($item in $toRestore) {
        if (-not (Test-Path -LiteralPath $item.BackupPath -PathType Leaf)) {
            Write-Warning "Skipping $($item.Label): backup file $($item.BackupPath) is gone."
            continue
        }
        if (Test-Path -LiteralPath $item.LivePath -PathType Leaf) {
            Write-Warning "Skipping $($item.Label): $($item.LivePath) already exists again. Resolve manually - both it and $($item.BackupPath) are left in place."
            $allRestored = $false
            $notRestored += $item
            continue
        }
        try {
            Move-Item -LiteralPath $item.BackupPath -Destination $item.LivePath
            Write-Output "Restored: $($item.Label) -> $($item.LivePath)"
        } catch {
            Write-Warning "Could not restore $($item.Label) ($($item.BackupPath)): $_"
            $allRestored = $false
            $notRestored += $item
        }
    }

    $finalRemaining = @($remaining) + @($notRestored)
    if ($finalRemaining.Count -eq 0) {
        Remove-Item -LiteralPath $manifestPath -Force
    } else {
        $finalRemaining | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8
        if (-not $allRestored) {
            Write-Warning "Some items were not restored; kept in $manifestPath so a re-run of 'ai-restore' can retry them."
        }
    }
}

if ($Restore) { Invoke-Restore -TargetProvider $Provider } else { Invoke-Backup -TargetProvider $Provider }
