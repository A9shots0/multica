#!/usr/bin/env pwsh
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ("smk-dr-health-" + [Guid]::NewGuid().ToString("N"))
$statePath = Join-Path $temporaryRoot "recovery/state"
New-Item -ItemType Directory -Path $statePath -Force | Out-Null
$healthScript = Join-Path $PSScriptRoot "Test-MulticaRecoveryHealth.ps1"
$module = Join-Path $PSScriptRoot "Recovery.Common.psm1"
Import-Module $module -Force

function Assert-True {
    param([bool] $Condition, [string] $Message)
    if (-not $Condition) { throw "FAIL: $Message" }
}

function Run-Health {
    & $healthScript -RepositoryRoot $temporaryRoot -HealthUri "http://127.0.0.1:9/health" -FailureThreshold 2 -NotificationMode off | Out-Null
}

try {
    $now = [DateTimeOffset]::UtcNow.ToString("o")
    Write-AtomicJson -Path (Join-Path $statePath "backup-latest.json") -Value @{
        run_id = "test-run"; status = "verified"; completed_at = $now
    }
    Write-AtomicJson -Path (Join-Path $statePath "restore-latest.json") -Value @{
        run_id = "test-run"; status = "verified"; completed_at = $now
    }

    Run-Health
    $first = Get-Content (Join-Path $statePath "watchdog-state.json") -Raw | ConvertFrom-Json
    Assert-True ($first.incident_key -eq "healthy") "one transient health failure opened an incident"

    Run-Health
    $second = Get-Content (Join-Path $statePath "watchdog-state.json") -Raw | ConvertFrom-Json
    Assert-True ($second.incident_key -eq "platform_unavailable") "failure threshold did not open platform incident"
    Assert-True ($second.transition -eq "opened") "platform incident did not record its opening transition"

    Run-Health
    $third = Get-Content (Join-Path $statePath "watchdog-state.json") -Raw | ConvertFrom-Json
    Assert-True ($third.transition -eq "unchanged") "unchanged failure created duplicate incident transition"
    Assert-True ($third.notification -eq "not_needed") "unchanged incident attempted another notification"
    Write-Host "PASS: thresholded outage detection and semantic dedupe"

    $ambiguous = @{
        receipt_version = 1
        checked_at = [DateTimeOffset]::UtcNow.ToString("o")
        platform = "unavailable"
        consecutive_health_failures = 2
        backup = "current"
        restore = "verified"
        incident_key = "platform_unavailable"
        transition = "opened"
        notification_mode = "live"
        notification = "admitted"
    }
    Write-AtomicJson -Path (Join-Path $statePath "watchdog-state.json") -Value $ambiguous
    Run-Health
    $afterAmbiguous = Get-Content (Join-Path $statePath "watchdog-state.json") -Raw | ConvertFrom-Json
    Assert-True ($afterAmbiguous.notification -eq "ambiguous") "admitted alert was not made fail-closed after restart"
    Assert-True ($afterAmbiguous.transition -eq "unchanged") "ambiguous alert opened a duplicate transition"
    Write-Host "PASS: ambiguous provider submission is never auto-retried"

    $source = Get-Content -LiteralPath $healthScript -Raw
    foreach ($gate in @("MULTICA_RECOVERY_SMS_LIVE_ENABLED", "MULTICA_OWNER_SMS_DESTINATION_VERIFIED", "NotificationMode")) {
        Assert-True ($source.Contains($gate)) "live SMS safety gate '$gate' is missing"
    }
    Write-Host "PASS: live SMS is explicitly gated"
}
finally {
    if (Test-Path -LiteralPath $temporaryRoot) { [IO.Directory]::Delete($temporaryRoot, $true) }
}
