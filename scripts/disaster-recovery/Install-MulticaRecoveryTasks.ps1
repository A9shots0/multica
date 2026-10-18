#!/usr/bin/env pwsh
[CmdletBinding(SupportsShouldProcess)]
param(
    [string] $RepositoryRoot = (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent),
    [string] $PassphraseFile,
    [string] $OutputDirectory = "recovery/backups",
    [string] $SecondaryDirectory,
    [ValidateRange(1, 24)] [int] $BackupEveryHours = 6,
    [ValidateRange(1, 30)] [int] $WatchdogEveryMinutes = 5
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force

if (-not $IsWindows) { throw "The recovery task installer currently supports Windows Task Scheduler only." }
if ([string]::IsNullOrWhiteSpace($PassphraseFile)) { throw "A protected recovery passphrase file is required for unattended backups." }
$RepositoryRoot = [IO.Path]::GetFullPath($RepositoryRoot)
$PassphraseFile = [IO.Path]::GetFullPath($PassphraseFile)
if (-not (Test-Path -LiteralPath $PassphraseFile -PathType Leaf)) { throw "Recovery passphrase file does not exist." }

$pwsh = Get-PowerShell7Path
$backupScript = Join-Path $PSScriptRoot "Backup-Multica.ps1"
$cloudVerifyScript = Join-Path $PSScriptRoot "Verify-OneDriveRecoveryCopy.ps1"
$watchdogScript = Join-Path $PSScriptRoot "Test-MulticaRecoveryHealth.ps1"
$backupArguments = @("-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-File", $backupScript, "-RepositoryRoot", $RepositoryRoot, "-PassphraseFile", $PassphraseFile, "-OutputDirectory", [IO.Path]::GetFullPath($OutputDirectory))
if ($SecondaryDirectory) { $backupArguments += @("-SecondaryDirectory", [IO.Path]::GetFullPath($SecondaryDirectory)) }
$watchdogArguments = @("-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-File", $watchdogScript, "-RepositoryRoot", $RepositoryRoot, "-NotificationMode", "dry_run")

$backupAction = New-ScheduledTaskAction -Execute $pwsh -Argument (($backupArguments | ForEach-Object { '"' + $_.Replace('"', '\"') + '"' }) -join " ") -WorkingDirectory $RepositoryRoot
$watchdogAction = New-ScheduledTaskAction -Execute $pwsh -Argument (($watchdogArguments | ForEach-Object { '"' + $_.Replace('"', '\"') + '"' }) -join " ") -WorkingDirectory $RepositoryRoot
$backupTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(2) -RepetitionInterval (New-TimeSpan -Hours $BackupEveryHours)
$watchdogTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(1) -RepetitionInterval (New-TimeSpan -Minutes $WatchdogEveryMinutes)
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -DontStopIfGoingOnBatteries -AllowStartIfOnBatteries -ExecutionTimeLimit (New-TimeSpan -Hours 2)

 $tasks = @(
    @{ Name = "SilverMediums-Multica-Backup"; Action = $backupAction; Trigger = $backupTrigger },
    @{ Name = "SilverMediums-Multica-Recovery-Watchdog"; Action = $watchdogAction; Trigger = $watchdogTrigger }
)
if ($SecondaryDirectory) {
    $cloudVerifyArguments = @(
        "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-File", $cloudVerifyScript,
        "-RepositoryRoot", $RepositoryRoot,
        "-OneDriveDirectory", [IO.Path]::GetFullPath($SecondaryDirectory)
    )
    $cloudVerifyAction = New-ScheduledTaskAction -Execute $pwsh -Argument (($cloudVerifyArguments | ForEach-Object { '"' + $_.Replace('"', '\"') + '"' }) -join " ") -WorkingDirectory $RepositoryRoot
    $cloudVerifyTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(15) -RepetitionInterval (New-TimeSpan -Hours $BackupEveryHours)
    $tasks += @{ Name = "SilverMediums-Multica-Recovery-Cloud-Verify"; Action = $cloudVerifyAction; Trigger = $cloudVerifyTrigger }
}

foreach ($task in $tasks) {
    if ($PSCmdlet.ShouldProcess($task.Name, "Install or replace hidden recovery scheduled task")) {
        Register-ScheduledTask -TaskName $task.Name -Action $task.Action -Trigger $task.Trigger -Settings $settings -Description "Silver Mediums local Multica disaster recovery" -Force | Out-Null
    }
}

Write-Host "Recovery tasks configured in dry-run notification mode."
