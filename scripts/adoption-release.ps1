#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('Preflight', 'Backup', 'Deploy', 'Rollback', 'Status')]
    [string] $Action,
    # The reviewed release to act on. The default is the release this script was
    # written for, whose evidence lives in the checkout it was built from; a
    # newer release packaged by scripts/release-candidate.py keeps everything
    # under this checkout and is verified by build-input content, not by HEAD.
    [ValidatePattern('^(adoption-20260916|release-\d{8})$')]
    [string] $Release = 'adoption-20260916'
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$releaseRoot = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot)).TrimEnd('\', '/')
$ownerRoot = [IO.Path]::GetFullPath('C:/Dev/multica').TrimEnd('\', '/')
if (-not [String]::Equals($releaseRoot, $ownerRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Production release refused: scripts/adoption-release.ps1 must run from the owner checkout '$ownerRoot', not '$releaseRoot'."
}
$operationalRoot = [IO.Path]::GetFullPath('C:/Dev/SMK-multica-selfhost-pilot').TrimEnd('\', '/')
$stateRoot = Join-Path $operationalRoot 'recovery/state'
$legacyRelease = $Release -eq 'adoption-20260916'
$evidenceRoot = Join-Path $releaseRoot ".qualification-evidence/production-$Release"
$releaseEvidence = Join-Path $releaseRoot ".qualification-evidence/$Release"
$deployRoot = Join-Path $releaseRoot "deploy/silver-mediums/$Release"
$identity = Get-Content -LiteralPath (Join-Path $deployRoot 'release-identity.json') -Raw | ConvertFrom-Json
$environmentFile = Join-Path $operationalRoot '.env'
$baseCompose = Join-Path $releaseRoot 'docker-compose.selfhost.yml'
$overlay = Join-Path $deployRoot 'compose.yml'
$candidateReceiptPath = Join-Path $releaseEvidence 'candidate-build-receipt.json'
$keyFile = 'C:/Users/a9sho/.multica-recovery/recovery.key'
$cloudDirectory = 'C:/Users/a9sho/OneDrive/Silver Mediums/Multica Recovery'
$imageArchive = Join-Path $evidenceRoot 'deployment-images.tar'
$compose = @('compose', '--env-file', $environmentFile, '--project-name', 'multica', '-f', $baseCompose, '-f', $overlay)
if ($Action -eq 'Rollback') { $compose += @('-f', (Join-Path $deployRoot 'rollback.yml')) }
New-Item -ItemType Directory -Path $evidenceRoot -Force | Out-Null
Import-Module (Join-Path $PSScriptRoot 'disaster-recovery/Recovery.Common.psm1') -Force

function Invoke-DockerSafe {
    param([string[]] $Arguments)
    return Invoke-RecoveryProcess -FilePath 'docker' -Arguments $Arguments -WorkingDirectory $releaseRoot
}

function Read-Receipt {
    param([string] $Name)
    return Get-Content -LiteralPath (Join-Path $stateRoot $Name) -Raw | ConvertFrom-Json
}

function Assert-RecoveryGate {
    $backup = Read-Receipt 'backup-latest.json'
    $restore = Read-Receipt 'restore-latest.json'
    $cloud = Read-Receipt 'off-machine-latest.json'
    $custody = Read-Receipt 'key-custody.json'
    $watchdog = Read-Receipt 'watchdog-state.json'
    $age = [DateTimeOffset]::UtcNow - [DateTimeOffset]::Parse($backup.completed_at)
    $watchdogAge = [DateTimeOffset]::UtcNow - [DateTimeOffset]::Parse($watchdog.checked_at)
    if ($backup.status -ne 'verified' -or $age.TotalHours -lt 0 -or $age.TotalHours -gt 24) { throw 'A current verified backup is required.' }
    if ($restore.status -ne 'verified' -or $restore.run_id -ne $backup.run_id -or $restore.resources_retained) { throw 'Matching isolated restore and cleanup are required.' }
    if ($cloud.status -ne 'verified' -or $cloud.run_id -ne $backup.run_id -or $cloud.artifact_sha256 -ne $backup.artifact.sha256 -or $cloud.key_custody -ne 'verified' -or $custody.status -ne 'verified') { throw 'Matching cloud artifact and separate key custody are required.' }
    if ($watchdog.platform -ne 'available' -or $watchdogAge.TotalMinutes -lt 0 -or $watchdogAge.TotalMinutes -gt 15) { throw 'Fresh healthy external watchdog evidence is required.' }
    if ($backup.components -notcontains 'deployment_images' -or $backup.components -notcontains 'compose_override') { throw 'This recovery point does not yet cover the exact release images and configuration.' }
    $rehearsal = Get-Content -LiteralPath (Join-Path $evidenceRoot 'application-recovery-rehearsal.json') -Raw | ConvertFrom-Json
    if ($rehearsal.status -ne 'verified' -or $rehearsal.source_id -ne $identity.source_id -or -not $rehearsal.cleanup_verified) { throw 'Candidate application recovery rehearsal is required.' }
    return $backup.run_id
}

# A caller's unrelated process environment must not override the selected file.
$composeText = [IO.File]::ReadAllText($baseCompose) + [IO.File]::ReadAllText($overlay)
$variableNames = @([regex]::Matches($composeText, '\$\{([A-Z][A-Z0-9_]*)') | ForEach-Object { $_.Groups[1].Value } | Sort-Object -Unique)
$savedEnvironment = @{}
foreach ($name in $variableNames) {
    $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
    Remove-Item -LiteralPath "Env:\$name" -ErrorAction SilentlyContinue
}
Push-Location -LiteralPath $releaseRoot
try {
    if ($Action -eq 'Status') {
        Write-Host (Invoke-DockerSafe ($compose + @('ps')))
        return
    }
    if ($legacyRelease) {
        $source = (& python (Join-Path $PSScriptRoot 'local-candidate-source.py') --check) | ConvertFrom-Json
    } else {
        $source = (& python (Join-Path $PSScriptRoot 'local-candidate-source.py') --manifest (Join-Path $releaseEvidence 'candidate-source-manifest.json') --normalize-eol --check --content-only) | ConvertFrom-Json
    }
    if ($LASTEXITCODE -ne 0 -or $source.source_id -ne $identity.source_id) { throw 'Reviewed build inputs changed.' }
    if ((Get-ComposeSha256 $baseCompose) -ne $identity.base_compose_sha256 -or (Get-ComposeSha256 $overlay) -ne $identity.overlay_sha256) { throw 'Reviewed production configuration drift.' }
    $qualificationName = if ($legacyRelease) { 'adoption-qualification.json' } else { 'release-qualification.json' }
    $qualification = Get-Content -LiteralPath (Join-Path $releaseEvidence $qualificationName) -Raw | ConvertFrom-Json
    if ($qualification.status -ne 'verified' -or $qualification.source_id -ne $source.source_id) { throw 'Reviewed adoption qualification is required.' }
    $operating = Get-Content -LiteralPath (Join-Path $releaseEvidence 'operating-configuration-check.json') -Raw | ConvertFrom-Json
    if (-not $operating.decision.lifecycle_admitted -or $operating.provider_calls_made -ne 0 -or $operating.credentials_read -ne 0) { throw 'A provider-free initial operating configuration check is required.' }
    $receipt = Get-Content -LiteralPath $candidateReceiptPath -Raw | ConvertFrom-Json
    foreach ($property in $receipt.images.PSObject.Properties) {
        # The legacy receipt maps tag -> id; a packaged release maps service -> id.
        $reference = if ($legacyRelease) { $property.Name } else { $property.Value }
        $id = (Invoke-DockerSafe @('image', 'inspect', '--format', '{{.Id}}', $reference)).Trim()
        if ($id -ne $property.Value) { throw "Reviewed image changed: $($property.Name)" }
    }
    if (-not $legacyRelease -and ($receipt.images.backend -ne $identity.images.backend -or $receipt.images.frontend -ne $identity.images.frontend)) {
        throw 'Release identity does not name the reviewed candidate images.'
    }
    $config = Invoke-DockerSafe ($compose + @('config', '--format', 'json')) | ConvertFrom-Json
    $expectedImages = @{
        backend = $identity.images.backend
        frontend = $identity.images.frontend
        postgres = 'sha256:cf134a767f474095eeba57e0117be8e568e011a63f33fbf252f14c9b760f8e6f'
    }
    if ($Action -eq 'Rollback') {
        $expectedImages.backend = $identity.rollback_images.backend
        $expectedImages.frontend = $identity.rollback_images.frontend
    }
    foreach ($service in $expectedImages.Keys) {
        if ($config.services.$service.image -ne $expectedImages[$service]) { throw 'Deployment image does not match release identity.' }
        [void](Invoke-DockerSafe @('image', 'inspect', $expectedImages[$service]))
    }
    $runtimeEnv = $config.services.backend.environment
    $allowedSecrets = @('JWT_SECRET', 'DATABASE_URL')
    foreach ($property in $runtimeEnv.PSObject.Properties) {
        if ($property.Name -match 'SECRET|PASSWORD|TOKEN|API_KEY|VERIFICATION_CODE' -and $property.Name -notin $allowedSecrets -and $property.Value) { throw 'Unexpected credential or development code in production environment.' }
    }
    if ($runtimeEnv.APP_ENV -ne 'production' -or $runtimeEnv.ALLOW_SIGNUP -ne 'false' -or $runtimeEnv.MULTICA_OWNER_SMS_MODE -ne 'off') { throw 'Production access/notification policy mismatch.' }
    foreach ($service in @('backend','frontend')) {
        $port = @($config.services.$service.ports)
        $expectedPort = if ($service -eq 'backend') { '8080' } else { '3000' }
        if ($port.Count -ne 1 -or $port[0].host_ip -ne '127.0.0.1' -or [string]$port[0].published -ne $expectedPort) { throw 'Unexpected published production listener.' }
    }
    foreach ($volume in @('pgdata','backend_uploads')) {
        $expected = "multica_$volume"
        if ($config.volumes.$volume.name -ne $expected -or -not $config.volumes.$volume.external) { throw 'Production volume identity mismatch.' }
        [void](Invoke-DockerSafe @('volume','inspect',$expected))
    }
    $serverEnv = ((Invoke-DockerSafe @('inspect','multica-backend-1') | ConvertFrom-Json)[0]).Config.Env
    $existingJWT = @($serverEnv | Where-Object { $_.StartsWith('JWT_SECRET=') })
    if ($existingJWT.Count -ne 1 -or $runtimeEnv.JWT_SECRET -ne $existingJWT[0].Substring(11)) { throw 'Existing authenticated-session secret differs from selected source.' }
    $serverEnv = $null; $existingJWT = $null
    $query = "SELECT json_build_object('runnable_tasks',(SELECT count(*) FROM agent_task_queue WHERE status NOT IN ('completed','failed','cancelled')),'autopilots',(SELECT count(*) FROM autopilot),'triggers',(SELECT count(*) FROM autopilot_trigger),'webhook_deliveries',(SELECT count(*) FROM webhook_delivery),'channel_installations',(SELECT count(*) FROM channel_installation),'channel_outbound',(SELECT count(*) FROM channel_outbound_message),'online_runtimes',(SELECT count(*) FROM agent_runtime WHERE status <> 'offline'))::text;"
    $inventory = (Invoke-DockerSafe ($compose + @('exec','-T','postgres','psql','-U',$config.services.postgres.environment.POSTGRES_USER,'-d',$config.services.postgres.environment.POSTGRES_DB,'-At','-v','ON_ERROR_STOP=1','-c',$query))).Trim() | ConvertFrom-Json
    if ($Action -ne 'Backup') {
        # Work in flight is the only thing a container swap can damage: a running
        # task loses its run. This gate once required the whole inventory to be
        # zero, which described an instance nobody had started using yet; once
        # runtimes are registered and features configured those counts are
        # normal, and the gate would refuse every deploy from then on. The rest
        # of the inventory is recorded in the preflight receipt instead.
        if ([int64]$inventory.runnable_tasks -ne 0) {
            throw "Deploy blocked: $($inventory.runnable_tasks) agent task(s) are still in flight."
        }
    }
    $serve = & 'C:/Program Files/Tailscale/tailscale.exe' serve status --json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'Cannot verify current Tailscale Serve configuration.' }
    if ($serve.PSObject.Properties.Name -contains 'AllowFunnel' -and @($serve.AllowFunnel.PSObject.Properties | Where-Object Value).Count -ne 0) { throw 'Public Funnel access must not be active.' }
    if ($serve.Web.'a9drew.tailda9924.ts.net:443'.Handlers.'/'.Proxy -ne 'http://127.0.0.1:3000') { throw 'Private route differs from the reviewed destination.' }
    $snapshot = [ordered]@{
        checked_at = [DateTimeOffset]::UtcNow.ToString('o'); source_id = $source.source_id
        base_compose_sha256 = Get-ComposeSha256 $baseCompose; overlay_sha256 = Get-ComposeSha256 $overlay
        images = $expectedImages; manual_start_inventory = $inventory
        private_url = 'https://a9drew.tailda9924.ts.net'; funnel = $false
        production_authentication = 'existing_session_or_owner_local_generated_one_time_code; no email configured'
        credential_source = 'existing owner .env; JWT identity matches running backend; values omitted'
    }
    Write-AtomicJson -Path (Join-Path $evidenceRoot 'release-preflight.json') -Value $snapshot
    $config = $null; $runtimeEnv = $null
    if ($Action -eq 'Preflight') { Write-Host 'Release preflight verified; no service or route changed.'; return }
    if ($Action -eq 'Backup') {
        & python (Join-Path $PSScriptRoot 'verify-completion-images.py') $imageArchive --identity (Join-Path $deployRoot 'release-identity.json') --receipt (Join-Path $evidenceRoot 'image-archive-verification.json')
        if ($LASTEXITCODE -ne 0) { throw 'Exact release/rollback image archive verification failed.' }
        if ($legacyRelease) {
            $additionalSource = @('.qualification-evidence/adoption-20260916','docs/silver-mediums/INITIAL-OPERATING-CONFIGURATION-20260916.md','deploy/silver-mediums/adoption-20260916')
        } else {
            $additionalSource = @(".qualification-evidence/$Release", "deploy/silver-mediums/$Release", 'deploy/silver-mediums/adoption-20260916')
        }
        $pilotTerminalEvidence = ".qualification-evidence/production-$Release"
        if (Test-Path -LiteralPath (Join-Path $releaseRoot $pilotTerminalEvidence) -PathType Container) { $additionalSource += $pilotTerminalEvidence }
        & (Join-Path $PSScriptRoot 'disaster-recovery/Backup-Multica.ps1') -RepositoryRoot $ownerRoot -SourceRoot $releaseRoot -ComposeFile $baseCompose -ComposeOverrideFile $overlay -EnvironmentFile $environmentFile -PassphraseFile $keyFile -OutputDirectory 'A:/Multica-Recovery/Backups' -SecondaryDirectory $cloudDirectory -ImageArchivePath $imageArchive -AdditionalSourcePath $additionalSource
        $backup = Read-Receipt 'backup-latest.json'
        $artifact = Join-Path 'A:/Multica-Recovery/Backups' $backup.artifact.name
        & (Join-Path $PSScriptRoot 'disaster-recovery/Restore-MulticaIsolated.ps1') -RepositoryRoot $ownerRoot -ArtifactPath $artifact -PassphraseFile $keyFile
        & (Join-Path $PSScriptRoot 'disaster-recovery/Verify-OneDriveRecoveryCopy.ps1') -RepositoryRoot $ownerRoot -OneDriveDirectory $cloudDirectory -TimeoutSeconds 600 -CloudSyncTimeoutSeconds 1800
        & (Join-Path $PSScriptRoot 'disaster-recovery/Test-MulticaRecoveryHealth.ps1') -RepositoryRoot $ownerRoot -NotificationMode off
        Write-Host 'Release backup, isolated restore, cloud readback and watchdog cycle verified.'
        return
    }
    $backupRun = Assert-RecoveryGate
    $releaseTime = [DateTimeOffset]::UtcNow
    Write-Host (Invoke-DockerSafe ($compose + @('up','-d','--no-build','--pull','never','--wait','--wait-timeout','180')))
    $health = Invoke-RestMethod -Uri 'http://127.0.0.1:8080/healthz' -TimeoutSec 15
    $version = Invoke-RestMethod -Uri 'http://127.0.0.1:8080/health' -TimeoutSec 15
    if ($Action -eq 'Deploy' -and $version.commit -ne $source.source_id) { throw 'Running source identity differs from the reviewed build.' }
    $containers = Invoke-DockerSafe @('inspect','multica-backend-1','multica-frontend-1','multica-postgres-1') | ConvertFrom-Json
    foreach ($container in $containers) {
        $service = $container.Config.Labels.'com.docker.compose.service'
        if ($container.Image -ne $expectedImages[$service] -or $container.State.Health.Status -ne 'healthy') { throw 'Started service identity/health mismatch.' }
    }
    $receiptName = if ($Action -eq 'Rollback') { 'rollback-receipt.json' } else { 'deployment-receipt.json' }
    Write-AtomicJson -Path (Join-Path $evidenceRoot $receiptName) -Value @{
        deployed_at = $releaseTime.ToString('o'); verified_at = [DateTimeOffset]::UtcNow.ToString('o')
        source_id = $source.source_id; images = $expectedImages; backup_run_id = $backupRun
        base_compose_sha256 = Get-ComposeSha256 $baseCompose; overlay_sha256 = Get-ComposeSha256 $overlay
        action = $Action; health = $health; version = $version; private_route = 'https://a9drew.tailda9924.ts.net'; business_acceptance = 'pending'
    }
    Write-Host 'Reviewed production images started and health verified. Browser/business acceptance remains separate.'
} finally {
    Pop-Location
    foreach ($name in $savedEnvironment.Keys) {
        if ($null -eq $savedEnvironment[$name]) { Remove-Item -LiteralPath "Env:\$name" -ErrorAction SilentlyContinue }
        else { [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], 'Process') }
    }
}
