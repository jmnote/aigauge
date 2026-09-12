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

# Local-only bookkeeping (gitignored under hack/temp/) of exactly what a backup moved, so
# restore does not have to re-derive paths that backup can no longer see.
$manifestDir = Join-Path $PSScriptRoot "temp"
$manifestPath = Join-Path $manifestDir ".ai-credentials-backup.local.json"

function Invoke-Backup {
    param([string]$TargetProvider = "all")

    if (-not (Test-Path -LiteralPath $manifestDir)) {
        New-Item -ItemType Directory -Force -Path $manifestDir | Out-Null
    }

    $existingMoved = @()
    if (Test-Path -LiteralPath $manifestPath -PathType Leaf) {
        $raw = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
        $existingMoved = @($raw | ForEach-Object { $_ })
    }

    $targets = Get-AiBackupTargets -TargetProvider $TargetProvider
    $newMoved = @()

    foreach ($target in $targets) {
        $livePath = $target.Path
        $backupPath = "$livePath.bak"

        # If already tracked in manifest, leave it as is
        $alreadyTracked = $existingMoved | Where-Object { $_.BackupPath -eq $backupPath -or $_.LivePath -eq $livePath }
        if ($alreadyTracked) {
            continue
        }

        # If live file does not exist, nothing to back up for this target
        if (-not (Test-Path -LiteralPath $livePath -PathType Leaf)) {
            continue
        }

        # If backup file already exists without being tracked, do not overwrite
        if (Test-Path -LiteralPath $backupPath -PathType Leaf) {
            Write-Warning "Skipping $($target.Label): $backupPath already exists. Not touching it."
            continue
        }

        try {
            Move-Item -LiteralPath $livePath -Destination $backupPath
            Write-Output "Backed up: $($target.Label) -> $backupPath"
            $newMoved += @{
                Provider   = $target.Provider
                Label      = $target.Label
                LivePath   = $livePath
                BackupPath = $backupPath
            }
        } catch {
            Write-Warning "Could not back up $($target.Label) ($livePath): $_. Still in use by a running process?"
        }
    }

    if ($newMoved.Count -eq 0) {
        # Check if requested provider(s) are already backed up in manifest
        $matchingExisting = $existingMoved | Where-Object {
            $prov = if ($_.Provider) { $_.Provider } else { Get-ProviderFromLabel $_.Label }
            $TargetProvider -eq "all" -or $prov -eq $TargetProvider
        }
        if ($matchingExisting.Count -gt 0) {
            $provLabel = if ($TargetProvider -eq "all") { "All requested providers" } else { "Provider '$TargetProvider'" }
            Write-Output "$provLabel are already backed up ($($matchingExisting.Count) item(s) in manifest). Run '.\build.ps1 ai-restore' when done testing."
        } else {
            Write-Output "Nothing to back up for '$TargetProvider' - no live credential or executable files were found."
        }
        return
    }

    $allMoved = @($existingMoved) + @($newMoved)
    $allMoved | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Write-Output ""
    $restoreHint = if ($TargetProvider -eq "all") { ".\build.ps1 ai-restore" } else { ".\build.ps1 ai-restore $TargetProvider" }
    Write-Output "Backed up $($newMoved.Count) new item(s) for '$TargetProvider' ($($allMoved.Count) total item(s) currently backed up). Run '$restoreHint' when done testing."
}

function Invoke-Restore {
    param([string]$TargetProvider = "all")

    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        $legacyPath = Join-Path $repo ".ai-credentials-backup.local.json"
        if (Test-Path -LiteralPath $legacyPath -PathType Leaf) {
            $manifestPath = $legacyPath
        } else {
            Write-Output "Nothing to restore - no backup manifest found at $manifestPath."
            return
        }
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
        $liveExists = Test-Path -LiteralPath $item.LivePath -PathType Leaf
        $backupExists = Test-Path -LiteralPath $item.BackupPath -PathType Leaf

        # Case 1: Already restored (live exists and backup file is already gone)
        if ($liveExists -and -not $backupExists) {
            Write-Output "Already restored: $($item.Label) -> $($item.LivePath)"
            continue
        }

        # Case 2: Backup is missing
        if (-not $backupExists) {
            Write-Warning "Skipping $($item.Label): backup file $($item.BackupPath) is gone."
            continue
        }

        # Case 3: Both exist (live was recreated while backup also exists)
        if ($liveExists -and $backupExists) {
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
        if (Test-Path -LiteralPath $manifestDir) {
            $remainingFiles = @(Get-ChildItem -LiteralPath $manifestDir -Force)
            if ($remainingFiles.Count -eq 0) {
                Remove-Item -LiteralPath $manifestDir -Force
            }
        }
    } else {
        $finalRemaining | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8
        if (-not $allRestored) {
            Write-Warning "Some items were not restored; kept in $manifestPath so a re-run of 'ai-restore' can retry them."
        }
    }
}

if ($Restore) { Invoke-Restore -TargetProvider $Provider } else { Invoke-Backup -TargetProvider $Provider }
