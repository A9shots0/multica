#!/usr/bin/env pwsh
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$module = Join-Path $PSScriptRoot "Recovery.Common.psm1"
Import-Module $module -Force

$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ("smk-dr-test-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $temporaryRoot | Out-Null

function Assert-True {
    param([bool] $Condition, [string] $Message)
    if (-not $Condition) { throw "FAIL: $Message" }
}

try {
    $plain = Join-Path $temporaryRoot "plain.bin"
    $encrypted = Join-Path $temporaryRoot "backup.smkdr"
    $restored = Join-Path $temporaryRoot "restored.bin"
    $passphrase = "test-only-recovery-passphrase-0001"
    [IO.File]::WriteAllBytes($plain, [byte[]](0..255))

    Protect-RecoveryArchive -InputPath $plain -OutputPath $encrypted -Passphrase $passphrase
    Unprotect-RecoveryArchive -InputPath $encrypted -OutputPath $restored -Passphrase $passphrase
    Assert-True ((Get-Sha256 $plain) -eq (Get-Sha256 $restored)) "encrypted archive did not round-trip"
    Write-Host "PASS: encrypted archive round-trip"

    $bytes = [IO.File]::ReadAllBytes($encrypted)
    $bytes[50] = $bytes[50] -bxor 1
    [IO.File]::WriteAllBytes($encrypted, $bytes)
    $rejected = $false
    try {
        Unprotect-RecoveryArchive -InputPath $encrypted -OutputPath $restored -Passphrase $passphrase
    }
    catch {
        $rejected = $_.Exception.Message -match "authentication failed"
    }
    Assert-True $rejected "tampered archive was not rejected before restore"
    Write-Host "PASS: tampered archive fails closed"

    $receipt = Join-Path $temporaryRoot "state/receipt.json"
    Write-AtomicJson -Path $receipt -Value @{ status = "verified"; secret = $null }
    Assert-True (Test-Path -LiteralPath $receipt) "atomic receipt was not published"
    Assert-True (-not (Test-Path -LiteralPath "$receipt.tmp")) "temporary receipt remained"
    Write-Host "PASS: atomic JSON receipt"

    $lockPath = Join-Path $temporaryRoot "state/recovery.lock"
    $lock = Enter-RecoveryLock -Path $lockPath
    $secondRejected = $false
    try { [void](Enter-RecoveryLock -Path $lockPath) } catch { $secondRejected = $true }
    Assert-True $secondRejected "overlapping recovery lock was accepted"
    Exit-RecoveryLock -Stream $lock -Path $lockPath
    Write-Host "PASS: overlapping recovery run rejected"

    [IO.File]::WriteAllText($lockPath, "pid=999999`nstarted=stale`n")
    $recoveredLock = Enter-RecoveryLock -Path $lockPath
    Assert-True ($null -ne $recoveredLock) "unowned stale lock was not reclaimed"
    Exit-RecoveryLock -Stream $recoveredLock -Path $lockPath
    Write-Host "PASS: stale recovery lock reclaimed"
}
finally {
    if (Test-Path -LiteralPath $temporaryRoot) {
        [IO.Directory]::Delete($temporaryRoot, $true)
    }
}
