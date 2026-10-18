#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [string] $RepositoryRoot = (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent),
    [string] $StateDirectory = "recovery/state",
    [Parameter(Mandatory)] [string] $OneDriveDirectory,
    [ValidateRange(30, 1800)] [int] $TimeoutSeconds = 300,
    # A ~360 MB encrypted artifact is still uploading when the backup step
    # returns, so off-machine custody gets its own, longer budget instead of
    # borrowing the local-copy one.
    [ValidateRange(60, 7200)] [int] $CloudSyncTimeoutSeconds = 1800,
    [ValidateRange(1, 120)] [int] $PollSeconds = 5
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force

function Resolve-UnderRoot {
    param([string] $Value)
    if ([IO.Path]::IsPathRooted($Value)) { return [IO.Path]::GetFullPath($Value) }
    return [IO.Path]::GetFullPath((Join-Path $RepositoryRoot $Value))
}

function Invoke-OneDriveDehydrate {
    param([Parameter(Mandatory)] [string] $Path)

    # Explorer's storage-provider verb is isolated in a child process because
    # the OneDrive shell extension can block indefinitely while reconciling.
    $command = @'
$ErrorActionPreference = "Stop"
$path = $env:MULTICA_ONEDRIVE_VERIFY_ARTIFACT
$shell = New-Object -ComObject Shell.Application
$folder = $shell.Namespace((Split-Path $path -Parent))
$item = if ($null -ne $folder) { $folder.ParseName((Split-Path $path -Leaf)) } else { $null }
$verb = if ($null -ne $item) {
    @($item.Verbs()) | Where-Object { ($_.Name -replace '&', '').Trim() -eq 'Free up space' } | Select-Object -First 1
} else { $null }
if ($null -eq $verb) { exit 2 }
$verb.DoIt()
'@
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
    $info = [Diagnostics.ProcessStartInfo]::new()
    $info.FileName = Get-PowerShell7Path
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.Environment["MULTICA_ONEDRIVE_VERIFY_ARTIFACT"] = $Path
    foreach ($argument in @("-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-EncodedCommand", $encoded)) {
        [void]$info.ArgumentList.Add($argument)
    }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $info
    try {
        if (-not $process.Start()) { return $false }
        if (-not $process.WaitForExit(10000)) {
            $process.Kill($true)
            [void]$process.WaitForExit(5000)
            return $false
        }
        return $process.ExitCode -eq 0
    }
    finally {
        $process.Dispose()
    }
}

$RepositoryRoot = [IO.Path]::GetFullPath($RepositoryRoot)
$statePath = Resolve-UnderRoot $StateDirectory
$latestBackupPath = Join-Path $statePath "backup-latest.json"
$offMachinePath = Join-Path $statePath "off-machine-latest.json"
$keyCustodyPath = Join-Path $statePath "key-custody.json"

if ([string]::IsNullOrWhiteSpace($env:OneDrive)) { throw "The real OneDrive sync root is unavailable." }
$oneDriveRoot = [IO.Path]::GetFullPath($env:OneDrive).TrimEnd('\') + '\'
$OneDriveDirectory = [IO.Path]::GetFullPath($OneDriveDirectory)
if (-not ($OneDriveDirectory.TrimEnd('\') + '\').StartsWith($oneDriveRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw "OneDrive recovery directory is outside the configured OneDrive sync root."
}
if (-not (Test-Path -LiteralPath $latestBackupPath -PathType Leaf)) { throw "Latest backup receipt is missing." }
$backup = Get-Content -LiteralPath $latestBackupPath -Raw | ConvertFrom-Json
if ($backup.status -ne "verified" -or [string]::IsNullOrWhiteSpace([string]$backup.run_id)) { throw "Latest backup is not verified." }

function Write-OffMachineFailure {
    param(
        [Parameter(Mandatory)] [string] $Status,
        [Parameter(Mandatory)] [string] $Reason,
        [string] $AvailabilityStatus = "Unknown",
        [hashtable] $Waiting
    )
    $receipt = @{
        receipt_version = 1
        kind = "off_machine_recovery_copy"
        status = $Status
        run_id = [string]$backup.run_id
        artifact_sha256 = [string]$backup.artifact.sha256
        checked_at = [DateTimeOffset]::UtcNow.ToString("o")
        availability_status = $AvailabilityStatus
        reason = $Reason
        key_custody = "pending"
    }
    if ($Waiting) { $receipt.waiting = $Waiting }
    Write-AtomicJson -Path $offMachinePath -Value $receipt
}

$artifactPath = Join-Path $OneDriveDirectory ([string]$backup.artifact.name)
$receiptPath = "$artifactPath.receipt.json"
$deadline = [DateTimeOffset]::UtcNow.AddSeconds($TimeoutSeconds)
while ((-not (Test-Path -LiteralPath $artifactPath -PathType Leaf) -or -not (Test-Path -LiteralPath $receiptPath -PathType Leaf)) -and [DateTimeOffset]::UtcNow -lt $deadline) {
    Start-Sleep -Seconds 2
}
if (-not (Test-Path -LiteralPath $artifactPath -PathType Leaf) -or -not (Test-Path -LiteralPath $receiptPath -PathType Leaf)) {
    throw "OneDrive recovery artifact or receipt did not appear before timeout."
}

# Wait for the copy to reach its recorded size and settle before hashing it.
# The hash requirement is unchanged; only the predictable first-read failure
# against a still-arriving artifact is removed.
$localCopy = Wait-ForVerifiedLocalCopy -Path $artifactPath -ExpectedBytes ([long]$backup.artifact.bytes) `
    -ExpectedSha256 ([string]$backup.artifact.sha256) -TimeoutSeconds $TimeoutSeconds -PollSeconds $PollSeconds
$localWaiting = @{
    local_copy_attempts = [int]$localCopy.Attempts
    local_copy_wait_seconds = [double]$localCopy.WaitedSeconds
    local_copy_observed_bytes = [long]$localCopy.ObservedBytes
    local_copy_expected_bytes = [long]$backup.artifact.bytes
}
if ($localCopy.Status -eq "mismatch") {
    Write-OffMachineFailure -Status "failed" -Reason ([string]$localCopy.Reason) -Waiting $localWaiting
    throw "OneDrive local copy hash does not match the backup receipt."
}
if ($localCopy.Status -ne "verified") {
    Write-OffMachineFailure -Status "sync_pending" -Reason ([string]$localCopy.Reason) -Waiting $localWaiting
    throw "OneDrive local copy did not finish arriving before timeout ($($localCopy.Reason))."
}

$oneDriveExe = Join-Path $env:LOCALAPPDATA "Microsoft/OneDrive/OneDrive.exe"
if (-not (Get-Process OneDrive -ErrorAction SilentlyContinue)) {
    if (-not (Test-Path -LiteralPath $oneDriveExe -PathType Leaf)) { throw "OneDrive sync client is unavailable." }
    Start-Process -FilePath $oneDriveExe -ArgumentList "/background" -WindowStyle Hidden
    Start-Sleep -Seconds 5
}
if (-not (Get-Process OneDrive -ErrorAction SilentlyContinue)) { throw "OneDrive sync client did not start." }

# Dehydrate only the encrypted cloud copy. Successful dehydration followed by
# rehydration and a matching hash proves provider readback rather than a second
# local-directory copy. The Offline attribute is Windows' bounded,
# machine-readable Files-on-Demand acknowledgement; unlike Explorer's metadata
# provider, this path cannot hang the scheduled verifier.
$cloudStarted = [DateTimeOffset]::UtcNow
$deadline = $cloudStarted.AddSeconds($CloudSyncTimeoutSeconds)
[void](Invoke-RecoveryProcess -FilePath "attrib.exe" -Arguments @("+U", "-P", $artifactPath))
# OneDrive refuses to free space for a file it has not finished uploading, so
# the verb is re-issued while we wait rather than attempted exactly once.
$dehydrateAttempts = 0
$becameCloudOnly = $false
$nextDehydrate = [DateTimeOffset]::MinValue
while ([DateTimeOffset]::UtcNow -lt $deadline) {
    if ([DateTimeOffset]::UtcNow -ge $nextDehydrate) {
        $dehydrateAttempts++
        [void](Invoke-OneDriveDehydrate -Path $artifactPath)
        $nextDehydrate = [DateTimeOffset]::UtcNow.AddSeconds([Math]::Max(30, $PollSeconds * 6))
    }
    $attributes = (Get-Item -LiteralPath $artifactPath -Force).Attributes
    if (($attributes -band [IO.FileAttributes]::Offline) -ne 0) {
        $becameCloudOnly = $true
        break
    }
    Start-Sleep -Seconds $PollSeconds
}
$cloudWaiting = $localWaiting + @{
    cloud_only_wait_seconds = [Math]::Round(([DateTimeOffset]::UtcNow - $cloudStarted).TotalSeconds, 3)
    dehydrate_attempts = $dehydrateAttempts
    cloud_sync_timeout_seconds = $CloudSyncTimeoutSeconds
}
if (-not $becameCloudOnly) {
    Write-OffMachineFailure -Status "sync_pending" -Reason "onedrive_cloud_only_transition_not_confirmed" -Waiting $cloudWaiting
    throw "OneDrive did not confirm off-machine custody before timeout."
}
if ((Get-Sha256 $artifactPath) -ne [string]$backup.artifact.sha256) {
    Write-OffMachineFailure -Status "failed" -Reason "provider_readback_hash_mismatch" -Waiting $cloudWaiting
    throw "OneDrive provider readback hash does not match."
}

$keyCustody = "pending"
if (Test-Path -LiteralPath $keyCustodyPath -PathType Leaf) {
    $keyReceipt = Get-Content -LiteralPath $keyCustodyPath -Raw | ConvertFrom-Json
    if ($keyReceipt.status -eq "verified") { $keyCustody = "verified" }
}
$verifiedAt = [DateTimeOffset]::UtcNow
$receipt = @{
    receipt_version = 1
    kind = "off_machine_recovery_copy"
    status = "verified"
    run_id = [string]$backup.run_id
    artifact_sha256 = [string]$backup.artifact.sha256
    verified_at = $verifiedAt.ToString("o")
    verification = "onedrive_dehydrate_rehydrate_hash_match"
    key_custody = $keyCustody
    waiting = $cloudWaiting
}
Write-AtomicJson -Path $offMachinePath -Value $receipt
Write-Host "OneDrive recovery copy verified by provider readback."
