#!/usr/bin/env pwsh
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
Import-Module (Join-Path $PSScriptRoot 'disaster-recovery/Recovery.Common.psm1') -Force

$base = Join-Path $root 'docker-compose.selfhost.yml'
$baseHash = Get-ComposeSha256 $base
$identityFiles = Get-ChildItem (Join-Path $root 'deploy/silver-mediums') -Recurse -Filter 'release-identity.json'
if ($identityFiles.Count -eq 0) { throw 'No release identities found.' }

foreach ($identityFile in $identityFiles) {
    $identity = Get-Content -LiteralPath $identityFile.FullName -Raw | ConvertFrom-Json
    $releaseDirectory = Split-Path -Parent $identityFile.FullName
    $overlay = Join-Path $releaseDirectory 'compose.yml'
    if ($identity.base_compose_sha256 -ne $baseHash) {
        throw "Base Compose hash mismatch in $($identityFile.FullName)."
    }
    if (-not (Test-Path -LiteralPath $overlay -PathType Leaf)) {
        throw "Release overlay is missing for $($identityFile.FullName)."
    }
    if ($identity.overlay_sha256 -ne (Get-ComposeSha256 $overlay)) {
        throw "Overlay Compose hash mismatch in $($identityFile.FullName)."
    }
}

$adoption = Get-Content -LiteralPath (Join-Path $root 'scripts/adoption-release.ps1') -Raw
if ($adoption -notmatch "C:/Dev/multica" -or $adoption -notmatch "C:/Dev/SMK-multica-selfhost-pilot") {
    throw 'Adoption release must separate the code checkout and operational roots.'
}

Write-Output "release identity fixture passed: $($identityFiles.Count) identities accept normalized Compose hashes"
