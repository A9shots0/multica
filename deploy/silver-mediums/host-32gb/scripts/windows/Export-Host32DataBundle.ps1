[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$ArtifactPath,
    [Parameter(Mandatory)] [string]$PassphraseFile,
    [Parameter(Mandatory)] [string]$OutputDirectory
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$profileRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$repoRoot = (Resolve-Path (Join-Path $profileRoot "..\..\..")).Path
$recoveryModule = Join-Path $repoRoot "scripts\disaster-recovery\Recovery.Common.psm1"
Import-Module $recoveryModule -Force

$artifact = [IO.Path]::GetFullPath($ArtifactPath)
$output = [IO.Path]::GetFullPath($OutputDirectory)
$receiptPath = "$artifact.receipt.json"
$releaseManifestPath = Join-Path $profileRoot "release-manifest.json"

if (-not (Test-Path -LiteralPath $artifact -PathType Leaf)) { throw "Encrypted backup artifact is missing." }
if (-not (Test-Path -LiteralPath $receiptPath -PathType Leaf)) { throw "Verified backup receipt is missing." }
if (-not (Test-Path -LiteralPath $PassphraseFile -PathType Leaf)) { throw "Protected passphrase file is missing." }
if (Test-Path -LiteralPath $output) {
    if (@(Get-ChildItem -LiteralPath $output -Force).Count -gt 0) { throw "Output directory is not empty: $output" }
} else {
    New-Item -ItemType Directory -Path $output | Out-Null
}

$receipt = Get-Content -LiteralPath $receiptPath -Raw | ConvertFrom-Json
if ($receipt.status -ne "verified") { throw "Backup receipt is not verified." }
$artifactHash = Get-Sha256 $artifact
if ($artifactHash -ne [string]$receipt.artifact.sha256) { throw "Encrypted backup hash does not match its receipt." }

$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) "multica-host32-export-$([Guid]::NewGuid().ToString('N'))"
$payloadPath = Join-Path $temporaryRoot "payload.tar.gz"
$contentPath = Join-Path $temporaryRoot "content"

try {
    New-Item -ItemType Directory -Path $contentPath -Force | Out-Null
    $passphrase = Get-RecoveryPassphrase -PassphraseFile $PassphraseFile
    Unprotect-RecoveryArchive -InputPath $artifact -OutputPath $payloadPath -Passphrase $passphrase
    [void](Invoke-RecoveryProcess -FilePath "tar.exe" -WorkingDirectory $contentPath -Arguments @("-xzf", $payloadPath))

    $manifestPath = Join-Path $contentPath "manifest.json"
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) { throw "Backup manifest is missing." }
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ($manifest.manifest_version -ne (Get-RecoveryManifestVersion)) { throw "Backup manifest version is unsupported." }
    if ([string]$manifest.run_id -ne [string]$receipt.run_id) { throw "Backup run identity does not match its receipt." }

    $selected = [ordered]@{
        database_dump = "database"
        database_verification = "database_verification"
        uploads_archive = "uploads"
        uploads_verification = "uploads_verification"
    }
    $components = [ordered]@{}
    foreach ($destinationName in $selected.Keys) {
        $sourceName = $selected[$destinationName]
        if ($manifest.components.PSObject.Properties.Name -notcontains $sourceName) {
            throw "Required backup component is missing: $sourceName"
        }
        $sourceComponent = $manifest.components.$sourceName
        $sourcePath = [IO.Path]::GetFullPath((Join-Path $contentPath $sourceComponent.path))
        $contentPrefix = $contentPath.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
        if (-not $sourcePath.StartsWith($contentPrefix, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Backup component escapes the extracted archive root."
        }
        if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) { throw "Backup component file is missing: $sourceName" }
        if ((Get-Item -LiteralPath $sourcePath).Length -ne [int64]$sourceComponent.bytes) { throw "Backup component size mismatch: $sourceName" }
        if ((Get-Sha256 $sourcePath) -ne [string]$sourceComponent.sha256) { throw "Backup component hash mismatch: $sourceName" }

        $extension = switch ($destinationName) {
            "database_dump" { ".dump" }
            "database_verification" { ".txt" }
            "uploads_archive" { ".tar.gz" }
            "uploads_verification" { ".sha256" }
        }
        $destinationPath = Join-Path $output "$destinationName$extension"
        Copy-Item -LiteralPath $sourcePath -Destination $destinationPath
        $file = Get-Item -LiteralPath $destinationPath
        $components[$destinationName] = [ordered]@{
            path = $file.Name
            bytes = $file.Length
            sha256 = Get-Sha256 $file.FullName
        }
    }

    $release = Get-Content -LiteralPath $releaseManifestPath -Raw | ConvertFrom-Json
    $transfer = [ordered]@{
        schema = 1
        created_at = [DateTimeOffset]::UtcNow.ToString("o")
        source_backup = [ordered]@{
            run_id = [string]$receipt.run_id
            sha256 = $artifactHash
            bytes = [int64]$receipt.artifact.bytes
        }
        application_source_revision = [string]$release.source_revision
        source_archive_git_head = [string]$manifest.source.git_head
        components = $components
        credentials_included = $false
        production_environment_included = $false
    }
    $transfer | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $output "transfer-manifest.json") -Encoding utf8NoBOM

    $hashes = Get-ChildItem -LiteralPath $output -File | Where-Object Name -ne "SHA256SUMS" | Sort-Object Name | ForEach-Object {
        "$(Get-Sha256 $_.FullName)  $($_.Name)"
    }
    [IO.File]::WriteAllText((Join-Path $output "SHA256SUMS"), (($hashes -join "`n") + "`n"), [Text.UTF8Encoding]::new($false))
    Write-Output "Bounded data bundle created without production configuration or credentials: $output"
}
finally {
    if (Test-Path -LiteralPath $temporaryRoot) { [IO.Directory]::Delete($temporaryRoot, $true) }
}
