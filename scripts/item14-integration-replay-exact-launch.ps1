[CmdletBinding()]
param(
    [ValidateSet('Provision', 'Preflight', 'Start', 'Cleanup')]
    [string] $Mode = 'Preflight'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$replayRoot = 'A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z'
$contractPath = Join-Path $replayRoot '.qualification-evidence\item14-integration-replay-20260915-v3\EXACT-LAUNCH-CONTRACT.json'
$contractItem = Get-Item -LiteralPath $contractPath -Force -ErrorAction Stop
if ($contractItem.PSIsContainer -or ($contractItem.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw 'exact launch contract must be a non-symlink regular file'
}
$contract = Get-Content -LiteralPath $contractPath -Raw | ConvertFrom-Json -DateKind String
$expectedProperties = @(
    'agent_timeout', 'auto_reload_enabled', 'auto_update_enabled', 'backend_port',
    'broker_path', 'broker_sha256', 'cli_path', 'cli_profile', 'cli_sha256',
    'controller_container', 'controller_image_id', 'daemon_id', 'database_container',
    'database_port', 'daemon_auth_expires_at', 'daemon_auth_mode', 'device_name',
    'frontend_container', 'guarded_package_required', 'host_attestation',
    'max_concurrent_tasks', 'max_turns', 'model', 'non_claude_provider_sentinel',
    'provider_call_limit', 'provider_profile_path', 'provider_retry_limit',
    'reasoning_effort', 'report_path', 'runtime_name', 'runtime_provider',
    'server_url', 'task_max_attempts', 'tool_policy', 'version', 'web_port',
    'workflow_controller_id', 'workflow_provider_task_ceiling', 'workspace_id',
    'workspaces_root', 'cli_profile_path'
)
$actualProperties = @($contract.PSObject.Properties.Name | Sort-Object)
if (($actualProperties -join "`n") -cne (($expectedProperties | Sort-Object) -join "`n")) {
    throw 'exact launch contract properties drifted'
}

$expectedImageID = 'sha256:53ce0736da07fbc00c0fc7c7be8c2b8414e5afea63100aa921fee881b5f37080'
if ($contract.version -cne 'item14.integration-replay-exact-launch.v2' -or
    $contract.controller_image_id -cne $expectedImageID -or
    $contract.server_url -cne 'http://127.0.0.1:18083' -or
    $contract.workspace_id -cne 'a9fbfbf1-a688-42ba-9b1c-5d8f42043be4' -or
    $contract.workflow_controller_id -cne '00000000-0000-4000-8000-000000001402' -or
    $contract.daemon_id -cne 'item14-integration-replay-20260915-v3' -or
    $contract.daemon_auth_mode -cne 'workspace_scoped_daemon_token_v1' -or
    $contract.daemon_auth_expires_at -cne '2026-09-16T14:00:00Z' -or
    $contract.host_attestation -cne 'alienware_windows' -or
    $contract.runtime_provider -cne 'claude' -or
    $contract.model -cne 'claude-sonnet-4-6' -or
    $contract.reasoning_effort -cne 'medium' -or
    $contract.max_concurrent_tasks -ne 1 -or $contract.agent_timeout -cne '20m0s' -or
    $contract.auto_update_enabled -or $contract.auto_reload_enabled -or
    $contract.provider_call_limit -ne 1 -or $contract.provider_retry_limit -ne 0 -or
    $contract.max_turns -ne 1 -or $contract.tool_policy -cne 'tool_free_v1' -or
    $contract.workflow_provider_task_ceiling -ne 5 -or $contract.task_max_attempts -ne 1 -or
    -not $contract.guarded_package_required) {
    throw 'exact launch contract value drifted'
}

$expectedCLIProfilePath = 'C:\Users\a9sho\.multica\profiles\item14-integration-replay-v3-daemon-20260915'
if ([IO.Path]::GetFullPath([string]$contract.cli_profile_path) -cne [IO.Path]::GetFullPath($expectedCLIProfilePath) -or
    $contract.cli_profile -cne 'item14-integration-replay-v3-daemon-20260915') {
    throw 'isolated Multica daemon profile drifted'
}
$daemonAuthExpiry = [DateTimeOffset]::ParseExact(
    [string]$contract.daemon_auth_expires_at,
    'yyyy-MM-ddTHH:mm:ssZ',
    [Globalization.CultureInfo]::InvariantCulture,
    [Globalization.DateTimeStyles]::AssumeUniversal
)
if ($daemonAuthExpiry -le [DateTimeOffset]::UtcNow) {
    throw 'task-local daemon identity expiry is not in the future'
}

function Get-TokenHash([string] $Token) {
    $bytes = [Text.Encoding]::UTF8.GetBytes($Token)
    return [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($bytes)).ToLowerInvariant()
}

function Invoke-ReplayDatabaseScalar([string] $Sql) {
    $output = & docker exec ([string]$contract.database_container) psql -U multica -d multica -Atq -c $Sql
    if ($LASTEXITCODE -ne 0) {
        throw 'replay database assertion failed'
    }
    $last = $output | Select-Object -Last 1
    if ($null -eq $last) {
        return ''
    }
    return ([string]$last).Trim()
}

$cliProfileDir = [string]$contract.cli_profile_path
$cliConfigPath = Join-Path $cliProfileDir 'config.json'

if ($Mode -eq 'Provision') {
    if (Test-Path -LiteralPath $cliConfigPath) {
        throw 'task-local daemon credential already exists'
    }
    $existing = Invoke-ReplayDatabaseScalar "SELECT count(*) FROM daemon_token WHERE workspace_id = '$($contract.workspace_id)' AND daemon_id = '$($contract.daemon_id)'"
    if ($existing -cne '0') {
        throw 'task-local daemon identity already exists'
    }
    [IO.Directory]::CreateDirectory($cliProfileDir) | Out-Null
    $profileDirItem = Get-Item -LiteralPath $cliProfileDir -Force
    if ($profileDirItem.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw 'task-local daemon profile must not be a reparse point'
    }
    $tokenBytes = [byte[]]::new(20)
    [Security.Cryptography.RandomNumberGenerator]::Fill($tokenBytes)
    $daemonToken = 'mdt_' + [Convert]::ToHexString($tokenBytes).ToLowerInvariant()
    $tokenHash = Get-TokenHash $daemonToken
    $tempConfigPath = Join-Path $cliProfileDir ('.config-' + [Guid]::NewGuid().ToString('N') + '.json.tmp')
    $inserted = $false
    try {
        $configJSON = [pscustomobject]@{
            server_url = [string]$contract.server_url
            workspace_id = [string]$contract.workspace_id
            token = $daemonToken
        } | ConvertTo-Json
        [IO.File]::WriteAllText($tempConfigPath, $configJSON + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
        $insertSQL = "INSERT INTO daemon_token (id, token_hash, workspace_id, daemon_id, expires_at) VALUES (gen_random_uuid(), '$tokenHash', '$($contract.workspace_id)', '$($contract.daemon_id)', '$($contract.daemon_auth_expires_at)'::timestamptz)"
        [void](Invoke-ReplayDatabaseScalar $insertSQL)
        $inserted = $true
        Move-Item -LiteralPath $tempConfigPath -Destination $cliConfigPath -ErrorAction Stop
    } catch {
        if ($inserted) {
            [void](Invoke-ReplayDatabaseScalar "DELETE FROM daemon_token WHERE token_hash = '$tokenHash' AND workspace_id = '$($contract.workspace_id)' AND daemon_id = '$($contract.daemon_id)'")
        }
        [IO.File]::Delete($tempConfigPath)
        [IO.File]::Delete($cliConfigPath)
        throw
    } finally {
        $daemonToken = $null
        $tokenBytes = $null
    }
}

foreach ($binding in @(
    @{ Path = [string]$contract.cli_path; SHA256 = [string]$contract.cli_sha256 },
    @{ Path = [string]$contract.broker_path; SHA256 = [string]$contract.broker_sha256 }
)) {
    $item = Get-Item -LiteralPath $binding.Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw 'launch executable must be a non-symlink regular file'
    }
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $binding.Path).Hash.ToLowerInvariant() -cne $binding.SHA256) {
        throw 'launch executable digest drifted'
    }
}

$containerBindings = @(
    @{ Name = [string]$contract.database_container; Port = [string]$contract.database_port; ContainerPort = '5432/tcp' },
    @{ Name = [string]$contract.controller_container; Port = [string]$contract.backend_port; ContainerPort = '8080/tcp' },
    @{ Name = [string]$contract.frontend_container; Port = [string]$contract.web_port; ContainerPort = '3000/tcp' }
)
foreach ($binding in $containerBindings) {
    $inspection = @(& docker inspect $binding.Name 2>$null | ConvertFrom-Json)[0]
    $portRows = @($inspection.NetworkSettings.Ports.($binding.ContainerPort))
    $hostPort = if ($portRows.Count -eq 1) { [string]$portRows[0].HostPort } else { '' }
    if (-not [bool]$inspection.State.Running -or $hostPort -cne $binding.Port) {
        throw "replay container binding drifted: $($binding.Name)"
    }
}
$controllerInspection = @(& docker inspect ([string]$contract.controller_container) | ConvertFrom-Json)[0]
$actualControllerImage = [string]$controllerInspection.Image
if ($actualControllerImage -cne [string]$contract.controller_image_id) {
    throw 'running controller image drifted'
}
$controllerEnv = @($controllerInspection.Config.Env)
if (@($controllerEnv | Where-Object { $_ -ceq ('MULTICA_WORKFLOW_CONTROLLER_ID=' + [string]$contract.workflow_controller_id) }).Count -ne 1) {
    throw 'running workflow controller identity drifted'
}

if ($Mode -eq 'Cleanup') {
    if (Test-Path -LiteralPath $cliConfigPath) {
        $configItem = Get-Item -LiteralPath $cliConfigPath -Force
        if ($configItem.PSIsContainer -or ($configItem.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw 'task-local daemon config is not a non-symlink regular file'
        }
        $cleanupConfig = Get-Content -LiteralPath $cliConfigPath -Raw | ConvertFrom-Json
        $cleanupToken = [string]$cleanupConfig.token
        if ($cleanupToken -cnotmatch '^mdt_[0-9a-f]{40}$') {
            throw 'task-local daemon credential shape drifted before cleanup'
        }
        $cleanupHash = Get-TokenHash $cleanupToken
        [void](Invoke-ReplayDatabaseScalar "DELETE FROM daemon_token WHERE token_hash = '$cleanupHash' AND workspace_id = '$($contract.workspace_id)' AND daemon_id = '$($contract.daemon_id)'")
        $cleanupToken = $null
        [IO.File]::Delete($cliConfigPath)
    }
    $remaining = Invoke-ReplayDatabaseScalar "SELECT count(*) FROM daemon_token WHERE workspace_id = '$($contract.workspace_id)' AND daemon_id = '$($contract.daemon_id)'"
    if ($remaining -cne '0' -or (Test-Path -LiteralPath $cliConfigPath)) {
        throw 'task-local daemon credential cleanup did not reach verified absence'
    }
    [pscustomobject]@{
        DaemonCredentialRevoked = $true
        DaemonCredentialFileAbsent = $true
        ProviderStarted = $false
    }
    return
}

$credentialPath = Join-Path (Join-Path ([string]$contract.provider_profile_path) '.claude') '.credentials.json'
$credentialItem = Get-Item -LiteralPath $credentialPath -Force -ErrorAction Stop
if ($credentialItem.PSIsContainer -or ($credentialItem.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw 'task-local credential shape is unavailable'
}
if (Test-Path -LiteralPath ([string]$contract.report_path)) {
    throw 'additive replay report path must be absent before launch'
}
if (Test-Path -LiteralPath ([string]$contract.non_claude_provider_sentinel)) {
    throw 'non-Claude provider sentinel unexpectedly exists'
}

$cliConfigItem = Get-Item -LiteralPath $cliConfigPath -Force -ErrorAction Stop
if ($cliConfigItem.PSIsContainer -or ($cliConfigItem.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw 'task-local daemon config is not a non-symlink regular file'
}
$cliConfig = Get-Content -LiteralPath $cliConfigPath -Raw | ConvertFrom-Json
$cliConfigProperties = @($cliConfig.PSObject.Properties.Name | Sort-Object)
if (($cliConfigProperties -join "`n") -cne ((@('server_url', 'token', 'workspace_id') | Sort-Object) -join "`n") -or
    $cliConfig.server_url -cne [string]$contract.server_url -or
    $cliConfig.workspace_id -cne [string]$contract.workspace_id -or
    ([string]$cliConfig.token) -cnotmatch '^mdt_[0-9a-f]{40}$') {
    throw 'task-local daemon configuration drifted'
}
$daemonTokenHash = Get-TokenHash ([string]$cliConfig.token)
$daemonIdentityRows = Invoke-ReplayDatabaseScalar "SELECT count(*) FROM daemon_token WHERE token_hash = '$daemonTokenHash' AND workspace_id = '$($contract.workspace_id)' AND daemon_id = '$($contract.daemon_id)' AND expires_at = '$($contract.daemon_auth_expires_at)'::timestamptz AND expires_at > now()"
$daemonIdentityTotal = Invoke-ReplayDatabaseScalar "SELECT count(*) FROM daemon_token WHERE workspace_id = '$($contract.workspace_id)' AND daemon_id = '$($contract.daemon_id)'"
if ($daemonIdentityRows -cne '1' -or $daemonIdentityTotal -cne '1') {
    throw 'task-local daemon identity is not exactly authenticated'
}
$daemonTokenHash = $null

$env:MULTICA_HOST_ATTESTATION = [string]$contract.host_attestation
$env:MULTICA_CLAUDE_PATH = [string]$contract.broker_path
$env:MULTICA_CLAUDE_MODEL = [string]$contract.model
foreach ($name in @(
    'MULTICA_CODEX_PATH', 'MULTICA_OPENCODE_PATH', 'MULTICA_CODEARTS_PATH',
    'MULTICA_DEVECO_PATH', 'MULTICA_OPENCLAW_PATH', 'MULTICA_HERMES_PATH',
    'MULTICA_PI_PATH', 'MULTICA_OMP_PATH', 'MULTICA_CURSOR_PATH',
    'MULTICA_COPILOT_PATH', 'MULTICA_KIMI_PATH', 'MULTICA_REASONIX_PATH',
    'MULTICA_DSH_PATH', 'MULTICA_KIRO_PATH', 'MULTICA_CODEBUDDY_PATH',
    'MULTICA_ANTIGRAVITY_PATH', 'MULTICA_QODER_PATH',
    'MULTICA_QODERCLICN_PATH', 'MULTICA_TRAECLI_PATH', 'MULTICA_GROK_PATH',
    'MULTICA_QWEN_PATH', 'MULTICA_QWENPAW_PATH', 'MULTICA_DIM_PATH',
    'MULTICA_MCODE_PATH', 'MULTICA_ZEROCLAW_PATH'
)) {
    [Environment]::SetEnvironmentVariable($name, [string]$contract.non_claude_provider_sentinel, 'Process')
}

if ($Mode -eq 'Preflight' -or $Mode -eq 'Provision') {
    [pscustomobject]@{
        Version = [string]$contract.version
        ControllerImageExact = $true
        ContainerBindingsExact = $true
        WorkflowControllerExact = $true
        DaemonIdentityExact = $true
        HostAttestation = $env:MULTICA_HOST_ATTESTATION
        ExecutableDigestsExact = $true
        CredentialShapeReady = $true
        GuardedPackageRequired = [bool]$contract.guarded_package_required
        ProviderStarted = $false
    }
    return
}

& ([string]$contract.cli_path) daemon start --foreground `
    --daemon-id ([string]$contract.daemon_id) `
    --device-name ([string]$contract.device_name) `
    --runtime-name ([string]$contract.runtime_name) `
    --workspaces-root ([string]$contract.workspaces_root) `
    --agent-timeout ([string]$contract.agent_timeout) `
    --max-concurrent-tasks ([int]$contract.max_concurrent_tasks) `
    --no-auto-update `
    --no-auto-reload `
    --profile ([string]$contract.cli_profile)
if ($LASTEXITCODE -ne 0) {
    throw "replay daemon exited with code $LASTEXITCODE"
}
