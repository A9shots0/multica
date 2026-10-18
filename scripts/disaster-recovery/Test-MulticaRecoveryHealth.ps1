#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [string] $RepositoryRoot = (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent),
    [string] $StateDirectory = "recovery/state",
    [string] $HealthUri = "http://127.0.0.1:8080/health",
    [ValidateRange(1, 20)] [int] $FailureThreshold = 3,
    [ValidateRange(1, 720)] [int] $MaxBackupAgeHours = 24,
    [ValidateSet("off", "dry_run", "live")] [string] $NotificationMode = "dry_run"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force

function Resolve-UnderRoot {
    param([string] $Value)
    if ([IO.Path]::IsPathRooted($Value)) { return [IO.Path]::GetFullPath($Value) }
    return [IO.Path]::GetFullPath((Join-Path $RepositoryRoot $Value))
}

function Read-JsonOrNull {
    param([string] $Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
    try { return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json } catch { return $null }
}

function Send-RecoverySms {
    param([string] $Body)
    if ($NotificationMode -eq "off") { return @{ state = "disabled" } }
    if ($NotificationMode -eq "dry_run") { return @{ state = "dry_run" } }
    if ($env:MULTICA_RECOVERY_SMS_LIVE_ENABLED -ne "true") { throw "Live recovery SMS requires MULTICA_RECOVERY_SMS_LIVE_ENABLED=true." }
    if ($env:MULTICA_OWNER_SMS_DESTINATION_VERIFIED -ne "true") { throw "Live recovery SMS destination is not verified." }
    foreach ($key in @("TELNYX_API_KEY", "TELNYX_PHONE_NUMBER", "MULTICA_OWNER_SMS_DESTINATION")) {
        if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($key))) { throw "Live recovery SMS configuration is incomplete." }
    }
    $headers = @{ Authorization = "Bearer $($env:TELNYX_API_KEY)"; "Content-Type" = "application/json" }
    $payload = @{ from = $env:TELNYX_PHONE_NUMBER; to = $env:MULTICA_OWNER_SMS_DESTINATION; text = $Body } | ConvertTo-Json -Compress
    $response = Invoke-RestMethod -Method Post -Uri "https://api.telnyx.com/v2/messages" -Headers $headers -Body $payload -TimeoutSec 20
    if (-not $response.data.id) { throw "Recovery SMS provider did not return an acceptance receipt." }
    return @{ state = "provider_accepted" }
}

$RepositoryRoot = [IO.Path]::GetFullPath($RepositoryRoot)
$statePath = Resolve-UnderRoot $StateDirectory
New-Item -ItemType Directory -Path $statePath -Force | Out-Null
$watchdogPath = Join-Path $statePath "watchdog-state.json"
$incidentPath = Join-Path $statePath "recovery-incident-latest.json"
$backupPath = Join-Path $statePath "backup-latest.json"
$restorePath = Join-Path $statePath "restore-latest.json"
$now = [DateTimeOffset]::UtcNow
$previous = Read-JsonOrNull $watchdogPath
$failures = if ($previous -and $previous.consecutive_health_failures) { [int]$previous.consecutive_health_failures } else { 0 }

$platformHealthy = $false
try {
    $response = Invoke-WebRequest -Uri $HealthUri -Method Get -TimeoutSec 8 -UseBasicParsing
    $platformHealthy = $response.StatusCode -ge 200 -and $response.StatusCode -lt 300
}
catch { $platformHealthy = $false }
$failures = if ($platformHealthy) { 0 } else { $failures + 1 }

$backup = Read-JsonOrNull $backupPath
$restore = Read-JsonOrNull $restorePath
$backupAgeHours = $null
$backupState = "missing"
if ($backup) {
    if ($backup.status -eq "verified") {
        $backupAgeHours = [Math]::Round(($now - [DateTimeOffset]::Parse($backup.completed_at)).TotalHours, 3)
        $backupState = if ($backupAgeHours -le $MaxBackupAgeHours) { "current" } else { "stale" }
    }
    else { $backupState = "failed" }
}
$restoreState = if (
    $restore -and $restore.status -eq "verified" -and $backup -and
    -not [string]::IsNullOrWhiteSpace([string]$restore.run_id) -and
    [string]$restore.run_id -eq [string]$backup.run_id
) { "verified" } elseif ($restore -and $restore.status -ne "verified") { "failed" } else { "unproven" }

$conditions = @()
if ($failures -ge $FailureThreshold) { $conditions += "platform_unavailable" }
if ($backupState -eq "missing" -or $backupState -eq "stale") { $conditions += "backup_due" }
if ($backupState -eq "failed") { $conditions += "backup_failed" }
if ($restoreState -eq "failed") { $conditions += "restore_failed" }
elseif ($restoreState -eq "unproven") { $conditions += "recovery_unproven" }
$incidentKey = if ($conditions.Count) { ($conditions | Sort-Object) -join "+" } else { "healthy" }
$previousKey = if ($previous -and $previous.incident_key) { [string]$previous.incident_key } else { $null }
$transition = if ($previousKey -eq $incidentKey) { "unchanged" } elseif ($incidentKey -eq "healthy") { "resolved" } else { "opened" }
$notification = @{ state = "not_needed" }

function New-WatchdogState {
    param([string] $NotificationState)
    return @{
        receipt_version = 1
        checked_at = $now.ToString("o")
        platform = if ($platformHealthy) { "available" } else { "unavailable" }
        consecutive_health_failures = $failures
        backup = $backupState
        backup_age_hours = $backupAgeHours
        restore = $restoreState
        incident_key = $incidentKey
        transition = $transition
        notification_mode = $NotificationMode
        notification = $NotificationState
    }
}

function New-IncidentState {
    param([string] $NotificationState)
    return @{
        receipt_version = 1
        kind = "recovery_health"
        status = if ($incidentKey -eq "healthy") { "resolved" } else { "active" }
        semantic_key = $incidentKey
        changed_at = $now.ToString("o")
        conditions = $conditions
        notification_mode = $NotificationMode
        notification = $NotificationState
    }
}

if ($transition -eq "unchanged" -and $previous -and $previous.notification -eq "admitted") {
    # The prior process may have crashed before or after provider acceptance.
    # Never auto-resend an ambiguous alert; require provider reconciliation.
    $notification = @{ state = "ambiguous" }
}

if ($transition -eq "opened") {
    $plain = switch -Regex ($incidentKey) {
        "platform_unavailable" { "Local Multica is unavailable after repeated checks."; break }
        "backup_failed" { "The latest Multica backup failed."; break }
        "restore_failed" { "The latest Multica recovery rehearsal failed."; break }
        "backup_due" { "Multica does not have a current verified backup."; break }
        default { "Multica backups exist, but recovery has not been proven." }
    }
    if ($NotificationMode -eq "live") {
        # Persist admission before the irreversible provider request. A crash
        # after this point is treated as ambiguous and is never auto-retried.
        Write-AtomicJson -Path $watchdogPath -Value (New-WatchdogState -NotificationState "admitted")
        Write-AtomicJson -Path $incidentPath -Value (New-IncidentState -NotificationState "admitted")
        try {
            $notification = Send-RecoverySms -Body "Silver Mediums recovery alert: $plain Open the local Recovery status for evidence and the automatic next action."
        }
        catch {
            $notification = @{ state = "ambiguous" }
            Write-AtomicJson -Path $watchdogPath -Value (New-WatchdogState -NotificationState $notification.state)
            Write-AtomicJson -Path $incidentPath -Value (New-IncidentState -NotificationState $notification.state)
            throw
        }
    }
    else {
        $notification = Send-RecoverySms -Body "Silver Mediums recovery alert: $plain Open the local Recovery status for evidence and the automatic next action."
    }
}

$state = New-WatchdogState -NotificationState $notification.state
Write-AtomicJson -Path $watchdogPath -Value $state

if ($transition -ne "unchanged" -or $notification.state -eq "ambiguous") {
    $incident = New-IncidentState -NotificationState $notification.state
    Write-AtomicJson -Path $incidentPath -Value $incident
}

Write-Host "Recovery health: $incidentKey ($transition)"
