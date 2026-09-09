param(
    [switch]$Restore
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot

# Where AI Gauge itself looks for each provider - kept in sync with
# internal/providers/claude.go (findClaudeCredentials), codex.go
# (findCodexCredentials), and antigravity_windows.go
# (antigravityFallbackPath). Renaming these to *.bak is what turns this
# machine into the "no CLI installed, no local sign-in" state a clean
# certification device starts from, without needing a second Windows
# account or a VM: internal_providers reads none of them, so every
# provider reports not_installed exactly as it would on that clean device.
#
# Get-Command must run on every call rather than once, because after a
# backup the live file is gone and Get-Command can no longer see it -
# which is also why backup records what it found in $manifestPath instead
# of relying on Restore to look the paths up again from scratch.
function Get-AiBackupTargets {
    $targets = @(
        @{ Label = "Claude credentials"; Path = (Join-Path $env:USERPROFILE ".claude\.credentials.json") }
        @{ Label = "Codex credentials"; Path = (Join-Path $env:USERPROFILE ".codex\auth.json") }
    )

    foreach ($entry in @(
            @{ Label = "claude executable"; Name = "claude" }
            @{ Label = "codex executable"; Name = "codex" }
            @{ Label = "agy executable"; Name = "agy" }
        )) {
        $command = Get-Command $entry.Name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($command) {
            $targets += @{ Label = $entry.Label; Path = $command.Source }
        }
    }

    # agy's fallback install location (used when it isn't on PATH) is
    # separate from what Get-Command above would have found.
    $agyFallback = Join-Path $env:LOCALAPPDATA "agy\bin\agy.exe"
    if ((Test-Path -LiteralPath $agyFallback -PathType Leaf) -and
        -not ($targets | Where-Object { $_.Path -eq $agyFallback })) {
        $targets += @{ Label = "agy executable (fallback path)"; Path = $agyFallback }
    }

    return $targets
}

# Local-only bookkeeping (gitignored) of exactly what a backup moved, so
# restore does not have to re-derive paths that backup can no longer see.
$manifestPath = Join-Path $repo ".ai-credentials-backup.local.json"

function Invoke-Backup {
    $targets = Get-AiBackupTargets
    if (Test-Path -LiteralPath $manifestPath -PathType Leaf) {
        Write-Warning "A backup manifest already exists at $manifestPath. Run '.\build.ps1 ai-restore' first before backing up again."
        exit 1
    }

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
            $moved += @{ Label = $target.Label; LivePath = $livePath; BackupPath = $backupPath }
        } catch {
            Write-Warning "Could not back up $($target.Label) ($livePath): $_. Still in use by a running process?"
        }
    }

    if ($moved.Count -eq 0) {
        Write-Output "Nothing to back up - no live credential or executable files were found."
        return
    }

    $moved | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Write-Output ""
    Write-Output "Backed up $($moved.Count) item(s). Run '.\build.ps1 ai-restore' when done testing."
}

function Invoke-Restore {
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        Write-Output "Nothing to restore - no backup manifest found at $manifestPath."
        return
    }

    $raw = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    $moved = @($raw | ForEach-Object { $_ })
    $allRestored = $true
    foreach ($item in $moved) {
        if (-not (Test-Path -LiteralPath $item.BackupPath -PathType Leaf)) {
            Write-Warning "Skipping $($item.Label): backup file $($item.BackupPath) is gone."
            continue
        }
        if (Test-Path -LiteralPath $item.LivePath -PathType Leaf) {
            # Something (e.g. a reinstall while backed up) recreated the live
            # file. Leave both alone rather than guessing which one is right.
            Write-Warning "Skipping $($item.Label): $($item.LivePath) already exists again. Resolve manually - both it and $($item.BackupPath) are left in place."
            $allRestored = $false
            continue
        }
        try {
            Move-Item -LiteralPath $item.BackupPath -Destination $item.LivePath
            Write-Output "Restored: $($item.Label) -> $($item.LivePath)"
        } catch {
            Write-Warning "Could not restore $($item.Label) ($($item.BackupPath)): $_"
            $allRestored = $false
        }
    }

    if ($allRestored) {
        Remove-Item -LiteralPath $manifestPath -Force
    } else {
        Write-Warning "Some items were not restored; kept $manifestPath so a re-run of 'ai-restore' can retry them."
    }
}

if ($Restore) { Invoke-Restore } else { Invoke-Backup }
