[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$profileRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$repoRoot = (Resolve-Path (Join-Path $profileRoot "..\..\..")).Path
$manifest = Get-Content -LiteralPath (Join-Path $profileRoot "release-manifest.json") -Raw | ConvertFrom-Json
$examplePath = Join-Path $profileRoot "host.env.example"
$composePath = Join-Path $profileRoot "compose.yml"
$featureFlagsPath = Join-Path $profileRoot "feature-flags.yml"

if ($manifest.target.os -ne "darwin" -or $manifest.target.architecture -ne "x86_64") {
    throw "Release manifest is not pinned to the Intel Mac target."
}
if ($manifest.data_included -ne $false -or $manifest.credentials_included -ne $false) {
    throw "Release manifest must not claim that data or credentials are included."
}

$example = @{}
foreach ($line in Get-Content -LiteralPath $examplePath) {
    if ($line -match '^([A-Z0-9_]+)=(.*)$') { $example[$matches[1]] = $matches[2] }
}
foreach ($mapping in @(
    @{ Key = "MULTICA_BACKEND_IMAGE"; Manifest = "backend" },
    @{ Key = "MULTICA_WEB_IMAGE"; Manifest = "frontend" },
    @{ Key = "MULTICA_POSTGRES_IMAGE"; Manifest = "postgres" }
)) {
    $manifestImage = [string]$manifest.images.PSObject.Properties[$mapping.Manifest].Value
    if ($example[$mapping.Key] -ne $manifestImage) {
        throw "$($mapping.Key) does not match release-manifest.json."
    }
}

$renderArguments = @(
    "compose", "--env-file", $examplePath,
    "-f", (Join-Path $repoRoot "docker-compose.selfhost.yml"),
    "-f", $composePath,
    "config", "--format", "json"
)
# Prove stale caller-level overrides cannot escape the task-targeted rollout
# file through this production Compose profile.
$env:FF_CONTINUOUS_WORK_OBSERVATION_V1 = "true"
$env:FF_CONTINUOUS_WORK_CHECKPOINT_V1 = "true"
$rendered = & docker @renderArguments
if ($LASTEXITCODE -ne 0) { throw "Docker Compose rendering failed." }
$config = $rendered | ConvertFrom-Json

foreach ($serviceName in @("postgres", "backend", "frontend")) {
    $service = $config.services.$serviceName
    if ($service.pull_policy -ne "never") { throw "$serviceName pull policy is not fail-closed." }
    if ($service.restart -ne "unless-stopped") { throw "$serviceName restart policy is not unless-stopped." }
}
foreach ($published in @($config.services.backend.ports, $config.services.frontend.ports)) {
    foreach ($port in @($published)) {
        if ($port.host_ip -ne "127.0.0.1") { throw "A service port is not bound to loopback." }
    }
}
if ([string]$config.services.backend.environment.FF_PLUGINS_V1 -ne "false") {
    throw "The host example must keep Plugin and Outcome Receipt integrations disabled until explicitly enabled."
}
if ($config.services.backend.environment.PSObject.Properties.Name -contains "FF_CONTINUOUS_WORK_CHECKPOINT_V1") {
    throw "The host profile must not globally override the targeted checkpoint cohort."
}
if ($config.services.backend.environment.PSObject.Properties.Name -contains "FF_CONTINUOUS_WORK_OBSERVATION_V1") {
    throw "The host profile must not globally override the targeted observation cohort."
}
if ([string]$config.services.backend.environment.MULTICA_FEATURE_FLAGS_FILE -ne "/run/secrets/multica/feature-flags.yml") {
    throw "The backend must load the reviewed feature-flag file."
}
$featureFlagMount = @(@($config.services.backend.volumes) | Where-Object { $_.target -eq "/run/secrets/multica/feature-flags.yml" })
if ($featureFlagMount.Count -ne 1 -or $featureFlagMount[0].read_only -ne $true) {
    throw "The reviewed feature-flag file must be mounted exactly once and read-only."
}
$expectedFeatureFlags = @(
    "continuous_work_observation_v1:",
    "  default: false",
    "  allow_by: workspace_provider",
    "  allow:",
    '    - "4e3cdf12-463c-4834-9291-c5ff2e422663:codex"',
    "",
    "continuous_work_checkpoint_v1:",
    "  default: false"
)
$actualFeatureFlags = @(Get-Content -LiteralPath $featureFlagsPath)
if (Compare-Object -ReferenceObject $expectedFeatureFlags -DifferenceObject $actualFeatureFlags -SyncWindow 0) {
    throw "The reviewed feature-flag cohort does not match the bounded Pilot-workspace Codex rollout."
}

$gitArguments = @(
    "-C", $repoRoot, "ls-files", "--",
    "deploy/silver-mediums/host-32gb/host.env",
    "deploy/silver-mediums/host-32gb/*.pem",
    "deploy/silver-mediums/host-32gb/*.key"
)
$trackedSensitive = & git @gitArguments
if ($LASTEXITCODE -ne 0) { throw "Git tracked-file inspection failed." }
if ($trackedSensitive) { throw "A sensitive host file is tracked by Git." }

Write-Output "Host32 profile verification passed."
