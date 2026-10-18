#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [string] $RepositoryRoot = (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent),
    [string] $StateDirectory = "recovery/state",
    [Parameter(Mandatory)] [switch] $ConfirmedInPhonePasswordManager
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force

if (-not $ConfirmedInPhonePasswordManager) { throw "Owner confirmation is required; key custody cannot be inferred." }
$statePath = if ([IO.Path]::IsPathRooted($StateDirectory)) {
    [IO.Path]::GetFullPath($StateDirectory)
} else {
    [IO.Path]::GetFullPath((Join-Path $RepositoryRoot $StateDirectory))
}
$receipt = @{
    receipt_version = 1
    kind = "recovery_key_custody"
    status = "verified"
    verified_at = [DateTimeOffset]::UtcNow.ToString("o")
    method = "owner_phone_password_manager"
}
Write-AtomicJson -Path (Join-Path $statePath "key-custody.json") -Value $receipt
Write-Host "Separate recovery-key custody recorded."
