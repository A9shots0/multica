#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [string] $RepositoryRoot = (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent),
    [string] $ComposeFile = "docker-compose.selfhost.yml",
    [string] $ComposeOverrideFile,
    [string] $EnvironmentFile,
    [string] $SourceRoot,
    [string[]] $AdditionalSourcePath = @(),
    [string] $ImageArchivePath,
    [string] $OutputDirectory = "recovery/backups",
    [string] $StateDirectory = "recovery/state",
    [string] $SecondaryDirectory,
    [string] $PassphraseFile,
    [ValidateRange(2, 365)] [int] $RetainVerified = 14
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force

function Resolve-UnderRoot {
    param([string] $Value)
    if ([IO.Path]::IsPathRooted($Value)) { return [IO.Path]::GetFullPath($Value) }
    return [IO.Path]::GetFullPath((Join-Path $RepositoryRoot $Value))
}

function Invoke-Docker {
    param([string[]] $Arguments, [string] $OutputPath)
    Invoke-RecoveryProcess -FilePath "docker" -Arguments $Arguments -WorkingDirectory $RepositoryRoot -StandardOutputPath $OutputPath
}

function Get-ComposeVolumeName {
    param([string] $LogicalName)
    $lines = Invoke-RecoveryProcess -FilePath "docker" -WorkingDirectory $RepositoryRoot -Arguments @(
        "volume", "ls", "--quiet",
        "--filter", "label=com.docker.compose.project=multica",
        "--filter", "label=com.docker.compose.volume=$LogicalName"
    )
    $names = @($lines -split "`r?`n" | Where-Object { $_ })
    if ($names.Count -ne 1) { throw "Expected exactly one compose volume for '$LogicalName'; found $($names.Count)." }
    return $names[0]
}

function Get-TrackedSourceFiles {
    $raw = Invoke-RecoveryProcess -FilePath "git" -WorkingDirectory $SourceRoot -Arguments @(
        "ls-files", "--cached", "--others", "--exclude-standard"
    )
    $files = @($raw -split "`r?`n" | Where-Object {
        $_ -and
        $_ -notmatch '(^|/)[.]env($|[.])' -and
        $_ -notmatch '(^|/)recovery/' -and
        (Test-Path -LiteralPath (Join-Path $SourceRoot $_) -PathType Leaf)
    })
    $sourcePrefix = $SourceRoot.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    foreach ($additional in $AdditionalSourcePath) {
        $path = [IO.Path]::GetFullPath((Join-Path $SourceRoot $additional))
        if (-not $path.StartsWith($sourcePrefix, [StringComparison]::OrdinalIgnoreCase)) { throw "Additional source path escapes the source root." }
        $item = Get-Item -LiteralPath $path -ErrorAction Stop
        if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Additional source links are not supported." }
        $entries = if ($item.PSIsContainer) { @(Get-ChildItem -LiteralPath $path -Recurse -File) } else { @($item) }
        foreach ($entry in $entries) {
            if (($entry.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Additional source links are not supported." }
            $relative = [IO.Path]::GetRelativePath($SourceRoot, $entry.FullName).Replace("\", "/")
            if ($relative -match '(^|/)[.]env($|[.])|(^|/)recovery/|[.]key$|[.]credentials') { throw "Additional source path contains excluded secret/configuration material." }
            $files += $relative
        }
    }
    return @($files | Sort-Object -Unique)
}

function Remove-ExpiredVerifiedArtifacts {
    param([string] $Directory, [int] $Keep)
    if (-not (Test-Path -LiteralPath $Directory)) { return }
    $artifacts = @(Get-ChildItem -LiteralPath $Directory -File -Filter "multica-recovery-*.smkdr" |
        Sort-Object LastWriteTimeUtc -Descending)
    if ($artifacts.Count -le $Keep) { return }
    foreach ($artifact in $artifacts[$Keep..($artifacts.Count - 1)]) {
        $receipt = "$($artifact.FullName).receipt.json"
        if (-not (Test-Path -LiteralPath $receipt -PathType Leaf)) { continue }
        $record = Get-Content -LiteralPath $receipt -Raw | ConvertFrom-Json
        if ($record.status -ne "verified") { continue }
        [IO.File]::Delete($artifact.FullName)
        [IO.File]::Delete($receipt)
    }
}

$RepositoryRoot = [IO.Path]::GetFullPath($RepositoryRoot)
$SourceRoot = if ($SourceRoot) { [IO.Path]::GetFullPath($SourceRoot) } else { $RepositoryRoot }
$composePath = Resolve-UnderRoot $ComposeFile
$environmentPath = if ($EnvironmentFile) { Resolve-UnderRoot $EnvironmentFile } else { Join-Path $RepositoryRoot ".env" }
$composeArguments = @("compose", "--env-file", $environmentPath, "--project-name", "multica", "-f", $composePath)
$overridePath = if ($ComposeOverrideFile) { Resolve-UnderRoot $ComposeOverrideFile } else { $null }
if ($overridePath) { $composeArguments += @("-f", $overridePath) }
$outputPath = Resolve-UnderRoot $OutputDirectory
$statePath = Resolve-UnderRoot $StateDirectory
$secondaryPath = if ($SecondaryDirectory) { Resolve-UnderRoot $SecondaryDirectory } else { $null }
$lockPath = Join-Path $statePath "recovery.lock"
$latestReceiptPath = Join-Path $statePath "backup-latest.json"
$runId = [DateTimeOffset]::UtcNow.ToString("yyyyMMddTHHmmssZ") + "-" + [Guid]::NewGuid().ToString("N").Substring(0, 8)
$startedAt = [DateTimeOffset]::UtcNow
$lock = $null
$stageRoot = Join-Path ([IO.Path]::GetTempPath()) "multica-recovery-$runId"
$payloadPath = Join-Path ([IO.Path]::GetTempPath()) "multica-recovery-$runId.tar.gz"
$artifactName = "multica-recovery-$runId.smkdr"
$artifactPath = Join-Path $outputPath $artifactName
$published = $false

try {
    New-Item -ItemType Directory -Path $outputPath, $statePath -Force | Out-Null
    $lock = Enter-RecoveryLock -Path $lockPath
    $passphrase = Get-RecoveryPassphrase -PassphraseFile $PassphraseFile
    if (-not (Test-Path -LiteralPath $composePath -PathType Leaf)) { throw "Compose file is missing." }

    [void](Invoke-RecoveryProcess -FilePath "docker" -WorkingDirectory $RepositoryRoot -Arguments @("info", "--format", "{{.ServerVersion}}"))
    $services = Invoke-RecoveryProcess -FilePath "docker" -WorkingDirectory $RepositoryRoot -Arguments ($composeArguments + @("ps", "--services", "--filter", "status=running"))
    foreach ($required in @("postgres", "backend")) {
        if (@($services -split "`r?`n") -notcontains $required) { throw "Required compose service '$required' is not running." }
    }

    $content = Join-Path $stageRoot "content"
    $config = Join-Path $content "config"
    $evidence = Join-Path $content "evidence"
    New-Item -ItemType Directory -Path $content, $config, $evidence -Force | Out-Null

    $databaseDump = Join-Path $content "database.dump"
    $databaseVerification = Join-Path $evidence "database-verification.txt"
    Invoke-Docker -OutputPath $databaseVerification -Arguments ($composeArguments + @(
        "exec", "-T", "postgres", "sh", "-lc",
        'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "select coalesce(max(version), current_database()) from schema_migrations; select count(*) from \"user\"; select count(*) from workspace; select count(*) from issue; select count(*) from comment; select count(*) from inbox_item;"'
    ))
    if ([string]::IsNullOrWhiteSpace((Get-Content -LiteralPath $databaseVerification -Raw))) {
        throw "Database verification evidence is empty."
    }
    Invoke-Docker -OutputPath $databaseDump -Arguments ($composeArguments + @(
        "exec", "-T", "postgres", "sh", "-lc",
        'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc'
    ))
    if ((Get-Item -LiteralPath $databaseDump).Length -le 0) { throw "Database dump is empty." }

    $uploadsVolume = Get-ComposeVolumeName -LogicalName "backend_uploads"
    $stageMount = $stageRoot.Replace("\", "/")
    Invoke-Docker -Arguments @(
        "run", "--rm", "-v", "${uploadsVolume}:/source:ro", "-v", "${stageMount}:/backup",
        "pgvector/pgvector:pg17", "sh", "-lc", "tar -czf /backup/content/uploads.tar.gz -C /source ."
    )
    $uploadsArchive = Join-Path $content "uploads.tar.gz"
    if (-not (Test-Path -LiteralPath $uploadsArchive -PathType Leaf)) { throw "Uploads archive was not created." }
    $uploadsVerification = Join-Path $evidence "uploads.sha256"
    Invoke-Docker -OutputPath $uploadsVerification -Arguments @(
        "run", "--rm", "-v", "${uploadsVolume}:/source:ro", "pgvector/pgvector:pg17", "sh", "-lc",
        'find /source -type f -print0 | sort -z | xargs -0 -r sha256sum | sed "s#  /source/#  #"'
    )

    $sourceFiles = Get-TrackedSourceFiles
    if ($sourceFiles.Count -eq 0) { throw "Source inventory is empty." }
    $sourceList = Join-Path $stageRoot "source-files.txt"
    $sourceFiles | Set-Content -LiteralPath $sourceList -Encoding utf8NoBOM
    $sourceArchive = Join-Path $content "source.tar.gz"
    [void](Invoke-RecoveryProcess -FilePath "tar.exe" -WorkingDirectory $SourceRoot -Arguments @(
        "-czf", $sourceArchive, "-T", $sourceList
    ))

    Copy-Item -LiteralPath $composePath -Destination (Join-Path $config "docker-compose.selfhost.yml")
    if (-not (Test-Path -LiteralPath $environmentPath -PathType Leaf)) { throw "Required .env configuration is missing." }
    Copy-Item -LiteralPath $environmentPath -Destination (Join-Path $config ".env")
    if ($overridePath) { Copy-Item -LiteralPath $overridePath -Destination (Join-Path $config "compose.override.yml") }
    Invoke-Docker -OutputPath (Join-Path $config "compose.resolved.yml") -Arguments ($composeArguments + @("config"))
    if ($ImageArchivePath) { Copy-Item -LiteralPath (Resolve-UnderRoot $ImageArchivePath) -Destination (Join-Path $content "deployment-images.tar") }

    $gitStatus = Invoke-RecoveryProcess -FilePath "git" -WorkingDirectory $SourceRoot -Arguments @("status", "--short", "--branch")
    $gitHead = (Invoke-RecoveryProcess -FilePath "git" -WorkingDirectory $SourceRoot -Arguments @("rev-parse", "HEAD")).Trim()
    $images = Invoke-RecoveryProcess -FilePath "docker" -WorkingDirectory $RepositoryRoot -Arguments ($composeArguments + @("images", "--format", "json"))
    $databaseEvidence = Invoke-RecoveryProcess -FilePath "docker" -WorkingDirectory $RepositoryRoot -Arguments ($composeArguments + @(
        "exec", "-T", "postgres", "sh", "-lc",
        'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" -c "select current_database(), pg_database_size(current_database()), (select count(*) from pg_stat_user_tables);"'
    ))
    $gitStatus | Set-Content -LiteralPath (Join-Path $evidence "git-status.txt") -Encoding utf8NoBOM
    $images | Set-Content -LiteralPath (Join-Path $evidence "compose-images.jsonl") -Encoding utf8NoBOM
    $databaseEvidence | Set-Content -LiteralPath (Join-Path $evidence "database.txt") -Encoding utf8NoBOM

    $components = @{}
    foreach ($entry in @(
        @{ Name = "database"; Path = $databaseDump },
        @{ Name = "database_verification"; Path = $databaseVerification },
        @{ Name = "uploads"; Path = $uploadsArchive },
        @{ Name = "uploads_verification"; Path = $uploadsVerification },
        @{ Name = "source"; Path = $sourceArchive },
        @{ Name = "environment"; Path = (Join-Path $config ".env") },
        @{ Name = "compose"; Path = (Join-Path $config "docker-compose.selfhost.yml") },
        @{ Name = "resolved_compose"; Path = (Join-Path $config "compose.resolved.yml") }
    )) {
        $file = Get-Item -LiteralPath $entry.Path
        $relative = [IO.Path]::GetRelativePath($content, $file.FullName).Replace("\", "/")
        $components[$entry.Name] = @{
            path = $relative
            bytes = $file.Length
            sha256 = Get-Sha256 $file.FullName
        }
    }
    foreach ($extra in @(
        @{ Name = "compose_override"; Path = (Join-Path $config "compose.override.yml"); Required = [bool]$overridePath },
        @{ Name = "deployment_images"; Path = (Join-Path $content "deployment-images.tar"); Required = [bool]$ImageArchivePath }
    )) {
        if ($extra.Required) {
            $file = Get-Item -LiteralPath $extra.Path -ErrorAction Stop
            $components[$extra.Name] = @{ path = [IO.Path]::GetRelativePath($content, $file.FullName).Replace("\", "/"); bytes = $file.Length; sha256 = Get-Sha256 $file.FullName }
        }
    }
    $manifest = @{
        manifest_version = Get-RecoveryManifestVersion
        run_id = $runId
        created_at = [DateTimeOffset]::UtcNow.ToString("o")
        source = @{ git_head = $gitHead; included_file_count = $sourceFiles.Count }
        components = $components
    }
    Write-AtomicJson -Path (Join-Path $content "manifest.json") -Value $manifest

    [void](Invoke-RecoveryProcess -FilePath "tar.exe" -WorkingDirectory $content -Arguments @("-czf", $payloadPath, "."))
    Protect-RecoveryArchive -InputPath $payloadPath -OutputPath $artifactPath -Passphrase $passphrase
    $artifactHash = Get-Sha256 $artifactPath
    $artifactBytes = (Get-Item -LiteralPath $artifactPath).Length

    $replication = "not_configured"
    if ($secondaryPath) {
        New-Item -ItemType Directory -Path $secondaryPath -Force | Out-Null
        $secondaryArtifact = Join-Path $secondaryPath $artifactName
        Copy-Item -LiteralPath $artifactPath -Destination $secondaryArtifact
        if ((Get-Sha256 $secondaryArtifact) -ne $artifactHash) { throw "Secondary backup verification failed." }
        # A matching copy proves copy integrity only. Off-machine custody and
        # independent recovery-key custody require a separate attestation.
        $replication = "copy_verified"
    }

    $completedAt = [DateTimeOffset]::UtcNow
    $receipt = @{
        receipt_version = 1
        run_id = $runId
        kind = "backup"
        status = "verified"
        started_at = $startedAt.ToString("o")
        completed_at = $completedAt.ToString("o")
        elapsed_seconds = [Math]::Round(($completedAt - $startedAt).TotalSeconds, 3)
        artifact = @{ name = $artifactName; bytes = $artifactBytes; sha256 = $artifactHash }
        components = @($components.Keys | Sort-Object)
        replication = $replication
    }
    Write-AtomicJson -Path "$artifactPath.receipt.json" -Value $receipt
    Write-AtomicJson -Path $latestReceiptPath -Value $receipt
    if ($secondaryPath) {
        $secondaryReceipt = Join-Path $secondaryPath "$artifactName.receipt.json"
        Copy-Item -LiteralPath "$artifactPath.receipt.json" -Destination $secondaryReceipt -Force
        if ((Get-Sha256 $secondaryReceipt) -ne (Get-Sha256 "$artifactPath.receipt.json")) {
            throw "Secondary receipt verification failed."
        }
    }
    $published = $true
    Remove-ExpiredVerifiedArtifacts -Directory $outputPath -Keep $RetainVerified
    if ($secondaryPath) { Remove-ExpiredVerifiedArtifacts -Directory $secondaryPath -Keep $RetainVerified }
    Write-Host "Backup verified: $artifactName"
}
catch {
    if (-not $published -and (Test-Path -LiteralPath $artifactPath)) { [IO.File]::Delete($artifactPath) }
    $failedAt = [DateTimeOffset]::UtcNow
    $message = $_.Exception.Message
    if ($message.Length -gt 500) { $message = $message.Substring(0, 500) }
    $failure = @{
        receipt_version = 1
        run_id = $runId
        kind = "backup"
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
    Exit-RecoveryLock -Stream $lock -Path $lockPath
    foreach ($path in @($payloadPath, $stageRoot)) {
        if (Test-Path -LiteralPath $path) {
            if ((Get-Item -LiteralPath $path).PSIsContainer) { [IO.Directory]::Delete($path, $true) }
            else { [IO.File]::Delete($path) }
        }
    }
}
