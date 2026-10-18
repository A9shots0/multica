#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string] $ArtifactPath,
    [string] $RepositoryRoot = (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent),
    [string] $StateDirectory = "recovery/state",
    [string] $PassphraseFile,
    [string] $DrillId,
    [switch] $KeepIsolatedResources
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force

function Resolve-UnderRoot {
    param([string] $Value)
    if ([IO.Path]::IsPathRooted($Value)) { return [IO.Path]::GetFullPath($Value) }
    return [IO.Path]::GetFullPath((Join-Path $RepositoryRoot $Value))
}

function Assert-IsolatedName {
    param([string] $Name)
    if ($Name -notmatch '^multica_item13_restore_[a-z0-9]{6,24}(_db|_uploads)?$') {
        throw "Refusing a recovery resource name outside the isolated Item 13 namespace."
    }
}

function Invoke-DockerText {
    param([string[]] $Arguments)
    return Invoke-RecoveryProcess -FilePath "docker" -Arguments $Arguments -WorkingDirectory $RepositoryRoot
}

$RepositoryRoot = [IO.Path]::GetFullPath($RepositoryRoot)
$ArtifactPath = Resolve-UnderRoot $ArtifactPath
$statePath = Resolve-UnderRoot $StateDirectory
$lockPath = Join-Path $statePath "recovery.lock"
$latestReceiptPath = Join-Path $statePath "restore-latest.json"
$startedAt = [DateTimeOffset]::UtcNow
$id = if ($DrillId) { $DrillId.ToLowerInvariant() } else { [Guid]::NewGuid().ToString("N").Substring(0, 10) }
if ($id -notmatch '^[a-z0-9]{6,24}$') { throw "DrillId must contain 6-24 lowercase letters or digits." }
$containerName = "multica_item13_restore_$id"
$databaseVolume = "${containerName}_db"
$uploadsVolume = "${containerName}_uploads"
Assert-IsolatedName $containerName
Assert-IsolatedName $databaseVolume
Assert-IsolatedName $uploadsVolume

$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) $containerName
$payloadPath = Join-Path ([IO.Path]::GetTempPath()) "$containerName.tar.gz"
$contentPath = Join-Path $temporaryRoot "content"
$lock = $null
$createdContainer = $false
$createdDatabaseVolume = $false
$createdUploadsVolume = $false
$completed = $false

try {
    New-Item -ItemType Directory -Path $statePath, $contentPath -Force | Out-Null
    $lock = Enter-RecoveryLock -Path $lockPath
    if (-not (Test-Path -LiteralPath $ArtifactPath -PathType Leaf)) { throw "Recovery artifact does not exist." }
    $passphrase = Get-RecoveryPassphrase -PassphraseFile $PassphraseFile
    [void](Invoke-DockerText @("info", "--format", "{{.ServerVersion}}"))

    $siblingReceipt = "$ArtifactPath.receipt.json"
    if (Test-Path -LiteralPath $siblingReceipt -PathType Leaf) {
        $artifactReceipt = Get-Content -LiteralPath $siblingReceipt -Raw | ConvertFrom-Json
        if ($artifactReceipt.status -ne "verified") { throw "Artifact receipt is not verified." }
        if ((Get-Sha256 $ArtifactPath) -ne $artifactReceipt.artifact.sha256) { throw "Artifact checksum does not match its verified receipt." }
    }

    Unprotect-RecoveryArchive -InputPath $ArtifactPath -OutputPath $payloadPath -Passphrase $passphrase
    [void](Invoke-RecoveryProcess -FilePath "tar.exe" -WorkingDirectory $contentPath -Arguments @("-xzf", $payloadPath))
    $manifestPath = Join-Path $contentPath "manifest.json"
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) { throw "Recovery manifest is missing." }
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ($manifest.manifest_version -ne (Get-RecoveryManifestVersion)) { throw "Recovery manifest version is not supported." }

    $required = @(
        "database", "database_verification", "uploads", "uploads_verification",
        "source", "environment", "compose", "resolved_compose"
    )
    foreach ($name in $required) {
        if ($manifest.components.PSObject.Properties.Name -notcontains $name) { throw "Recovery component '$name' is missing from the manifest." }
    }
    foreach ($name in $manifest.components.PSObject.Properties.Name) {
        $component = $manifest.components.$name
        $candidate = [IO.Path]::GetFullPath((Join-Path $contentPath $component.path))
        if (-not $candidate.StartsWith($contentPath.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw "Recovery component path escapes the archive root." }
        if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) { throw "Recovery component '$name' is missing." }
        if ((Get-Item -LiteralPath $candidate).Length -ne $component.bytes) { throw "Recovery component '$name' has the wrong size." }
        if ((Get-Sha256 $candidate) -ne $component.sha256) { throw "Recovery component '$name' failed checksum verification." }
    }

    foreach ($resource in @($containerName, $databaseVolume, $uploadsVolume)) {
        $existing = if ($resource -eq $containerName) {
            Invoke-DockerText @("container", "ls", "-a", "--filter", "name=^/$resource$", "--format", "{{.Names}}")
        } else {
            Invoke-DockerText @("volume", "ls", "--filter", "name=^$resource$", "--format", "{{.Name}}")
        }
        if (-not [string]::IsNullOrWhiteSpace($existing)) { throw "Isolated recovery resource already exists; choose a new DrillId." }
    }

    [void](Invoke-DockerText @("volume", "create", $databaseVolume)); $createdDatabaseVolume = $true
    [void](Invoke-DockerText @("volume", "create", $uploadsVolume)); $createdUploadsVolume = $true
    $restorePassword = [Guid]::NewGuid().ToString("N") + [Guid]::NewGuid().ToString("N")
    [void](Invoke-DockerText @(
        "run", "-d", "--name", $containerName,
        "-e", "POSTGRES_USER=multica_restore",
        "-e", "POSTGRES_DB=multica_restore",
        "-e", "POSTGRES_PASSWORD=$restorePassword",
        "-v", "${databaseVolume}:/var/lib/postgresql/data",
        "pgvector/pgvector:pg17"
    )); $createdContainer = $true

    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        try {
            [void](Invoke-DockerText @("exec", $containerName, "pg_isready", "-U", "multica_restore", "-d", "multica_restore"))
            $ready = $true
            break
        }
        catch { Start-Sleep -Seconds 1 }
    }
    if (-not $ready) { throw "Isolated PostgreSQL did not become ready." }

    $databaseDump = Join-Path $contentPath $manifest.components.database.path
    [void](Invoke-DockerText @("cp", $databaseDump, "${containerName}:/tmp/database.dump"))
    [void](Invoke-DockerText @(
        "exec", $containerName, "pg_restore", "-U", "multica_restore", "-d", "multica_restore",
        "--no-owner", "--no-privileges", "/tmp/database.dump"
    ))
    [void](Invoke-DockerText @("exec", $containerName, "rm", "-f", "/tmp/database.dump"))

    $mount = $contentPath.Replace("\", "/")
    [void](Invoke-DockerText @(
        "run", "--rm", "-v", "${uploadsVolume}:/restore", "-v", "${mount}:/backup:ro",
        "pgvector/pgvector:pg17", "sh", "-lc", "tar -xzf /backup/uploads.tar.gz -C /restore"
    ))

    $databaseResult = (Invoke-DockerText @(
        "exec", $containerName, "psql", "-U", "multica_restore", "-d", "multica_restore", "-At", "-F", "|",
        "-c", "select current_database(), pg_database_size(current_database()), (select count(*) from pg_stat_user_tables);"
    )).Trim()
    $databaseParts = $databaseResult -split '[|]'
    if ($databaseParts.Count -ne 3 -or $databaseParts[0] -ne "multica_restore" -or [int64]$databaseParts[1] -le 0 -or [int]$databaseParts[2] -le 0) {
        throw "Isolated database verification failed."
    }
    $expectedDatabaseVerification = [IO.File]::ReadAllText((Join-Path $contentPath $manifest.components.database_verification.path)).Trim()
    $actualDatabaseVerification = (Invoke-DockerText @(
        "exec", $containerName, "psql", "-U", "multica_restore", "-d", "multica_restore", "-At", "-c",
        'select coalesce(max(version), current_database()) from schema_migrations; select count(*) from "user"; select count(*) from workspace; select count(*) from issue; select count(*) from comment; select count(*) from inbox_item;'
    )).Trim()
    if ($actualDatabaseVerification -ne $expectedDatabaseVerification) {
        throw "Recovered migration identity or representative database counts do not match the backup evidence."
    }
    $uploadCount = [int](Invoke-DockerText @(
        "run", "--rm", "-v", "${uploadsVolume}:/restore:ro", "pgvector/pgvector:pg17", "sh", "-lc",
        "find /restore -type f | wc -l"
    )).Trim()
    $expectedUploadsVerification = [IO.File]::ReadAllText((Join-Path $contentPath $manifest.components.uploads_verification.path)).Trim()
    $actualUploadsVerification = (Invoke-DockerText @(
        "run", "--rm", "-v", "${uploadsVolume}:/source:ro", "pgvector/pgvector:pg17", "sh", "-lc",
        'find /source -type f -print0 | sort -z | xargs -0 -r sha256sum | sed "s#  /source/#  #"'
    )).Trim()
    if ($actualUploadsVerification -ne $expectedUploadsVerification) {
        throw "Recovered upload paths or checksums do not match the backup evidence."
    }
    $sourceArchive = Join-Path $contentPath $manifest.components.source.path
    $sourceListing = Invoke-RecoveryProcess -FilePath "tar.exe" -WorkingDirectory $temporaryRoot -Arguments @("-tzf", $sourceArchive)
    if ([string]::IsNullOrWhiteSpace($sourceListing)) { throw "Recovered source archive is empty." }
    $sourceCount = @($sourceListing -split "`r?`n" | Where-Object { $_ }).Count
    if ($sourceCount -ne [int]$manifest.source.included_file_count) {
        throw "Recovered source inventory count does not match the backup manifest."
    }

    # A successful receipt is published only after disposable plaintext
    # resources are either explicitly retained or strictly removed and proven absent.
    if (-not $KeepIsolatedResources) {
        Assert-IsolatedName $containerName
        [void](Invoke-DockerText @("rm", "-f", $containerName))
        $createdContainer = $false
        foreach ($volume in @($uploadsVolume, $databaseVolume)) {
            Assert-IsolatedName $volume
            [void](Invoke-DockerText @("volume", "rm", $volume))
            if ($volume -eq $uploadsVolume) { $createdUploadsVolume = $false }
            else { $createdDatabaseVolume = $false }
        }
        $leftoverContainer = Invoke-DockerText @("container", "ls", "-a", "--filter", "name=^/$containerName$", "--format", "{{.Names}}")
        $leftoverVolumes = Invoke-DockerText @("volume", "ls", "--filter", "name=^${containerName}_", "--format", "{{.Name}}")
        if (-not [string]::IsNullOrWhiteSpace($leftoverContainer) -or -not [string]::IsNullOrWhiteSpace($leftoverVolumes)) {
            throw "Disposable recovery resources remain after cleanup."
        }
    }

    $completedAt = [DateTimeOffset]::UtcNow
    $recoveryPoint = [DateTimeOffset]::Parse($manifest.created_at)
    $receipt = @{
        receipt_version = 1
        run_id = $manifest.run_id
        drill_id = $id
        kind = "isolated_restore"
        status = "verified"
        started_at = $startedAt.ToString("o")
        completed_at = $completedAt.ToString("o")
        elapsed_seconds = [Math]::Round(($completedAt - $startedAt).TotalSeconds, 3)
        recovery_point_age_seconds = [Math]::Round(($startedAt - $recoveryPoint).TotalSeconds, 3)
        database = @{
            bytes = [int64]$databaseParts[1]
            user_tables = [int]$databaseParts[2]
            migration_and_representative_counts = "matched"
        }
        uploads = @{ files = $uploadCount; path_and_checksum_manifest = "matched" }
        source = @{ included_file_count = $manifest.source.included_file_count; git_head = $manifest.source.git_head }
        resources_retained = [bool]$KeepIsolatedResources
    }
    Write-AtomicJson -Path (Join-Path $statePath "restore-$id.json") -Value $receipt
    Write-AtomicJson -Path $latestReceiptPath -Value $receipt
    $completed = $true
    Write-Host "Isolated restore verified: $id"
}
catch {
    $failedAt = [DateTimeOffset]::UtcNow
    $message = $_.Exception.Message
    if ($message.Length -gt 500) { $message = $message.Substring(0, 500) }
    $failure = @{
        receipt_version = 1
        drill_id = $id
        kind = "isolated_restore"
        status = "failed"
        started_at = $startedAt.ToString("o")
        completed_at = $failedAt.ToString("o")
        elapsed_seconds = [Math]::Round(($failedAt - $startedAt).TotalSeconds, 3)
        error = $message
    }
    try { Write-AtomicJson -Path $latestReceiptPath -Value $failure } catch {}
    throw
}
finally {
    if (-not $KeepIsolatedResources) {
        if ($createdContainer) {
            Assert-IsolatedName $containerName
            try { [void](Invoke-DockerText @("rm", "-f", $containerName)) } catch { Write-Warning "Failed to remove recovery container after an unsuccessful drill." }
        }
        foreach ($volume in @(
            @{ Name = $uploadsVolume; Created = $createdUploadsVolume },
            @{ Name = $databaseVolume; Created = $createdDatabaseVolume }
        )) {
            if ($volume.Created) {
                Assert-IsolatedName $volume.Name
                try { [void](Invoke-DockerText @("volume", "rm", $volume.Name)) } catch { Write-Warning "Failed to remove a recovery volume after an unsuccessful drill." }
            }
        }
    }
    Exit-RecoveryLock -Stream $lock -Path $lockPath
    foreach ($path in @($payloadPath, $temporaryRoot)) {
        if (Test-Path -LiteralPath $path) {
            if ((Get-Item -LiteralPath $path).PSIsContainer) { [IO.Directory]::Delete($path, $true) }
            else { [IO.File]::Delete($path) }
        }
    }
    if (-not $completed) { Write-Verbose "Isolated restore did not complete." }
}
