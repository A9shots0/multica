[CmdletBinding()]
param(
    [ValidateSet('Preflight', 'Run')]
    [string] $Mode = 'Preflight'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$replayRoot = 'A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z'
$cliPath = Join-Path $replayRoot 'server\bin\multica-item14-replay-v3.exe'
$brokerPath = Join-Path $replayRoot 'server\bin\item14-integration-pilot-broker-v3.exe'
$workspacesRoot = Join-Path $replayRoot '.qualification-evidence\item14-integration-replay-20260915-v2\runtime-workspaces'
$absentProviderPath = Join-Path $replayRoot '.qualification-evidence\item14-integration-replay-20260915-v2\absent-provider.exe'

foreach ($requiredPath in @($cliPath, $brokerPath, $workspacesRoot)) {
    if (-not (Test-Path -LiteralPath $requiredPath)) {
        throw "required replay path is absent: $requiredPath"
    }
}
if (Test-Path -LiteralPath $absentProviderPath) {
    throw "provider-denial sentinel unexpectedly exists: $absentProviderPath"
}

# This stable parent-only fact is required to compare controller-bound Windows
# authority with the actual daemon host. It is deliberately not inherited by
# the tool-free provider child.
$env:MULTICA_HOST_ATTESTATION = 'alienware_windows'
$env:MULTICA_CLAUDE_PATH = $brokerPath
$env:MULTICA_CLAUDE_MODEL = 'claude-sonnet-4-6'

# Keep every non-Claude provider undiscoverable in this exact replay harness.
foreach ($name in @(
    'MULTICA_CODEX_PATH',
    'MULTICA_OPENCODE_PATH',
    'MULTICA_CODEARTS_PATH',
    'MULTICA_DEVECO_PATH',
    'MULTICA_OPENCLAW_PATH',
    'MULTICA_HERMES_PATH',
    'MULTICA_PI_PATH',
    'MULTICA_OMP_PATH',
    'MULTICA_CURSOR_PATH',
    'MULTICA_COPILOT_PATH',
    'MULTICA_KIMI_PATH',
    'MULTICA_REASONIX_PATH',
    'MULTICA_DSH_PATH',
    'MULTICA_KIRO_PATH',
    'MULTICA_CODEBUDDY_PATH',
    'MULTICA_ANTIGRAVITY_PATH',
    'MULTICA_QODER_PATH',
    'MULTICA_QODERCLICN_PATH',
    'MULTICA_TRAECLI_PATH',
    'MULTICA_GROK_PATH',
    'MULTICA_QWEN_PATH',
    'MULTICA_QWENPAW_PATH',
    'MULTICA_DIM_PATH',
    'MULTICA_MCODE_PATH',
    'MULTICA_ZEROCLAW_PATH'
)) {
    [Environment]::SetEnvironmentVariable($name, $absentProviderPath, 'Process')
}

if ($Mode -eq 'Preflight') {
    [pscustomobject]@{
        HostAttestation = $env:MULTICA_HOST_ATTESTATION
        ClaudePathExact = [string]::Equals($env:MULTICA_CLAUDE_PATH, $brokerPath, [StringComparison]::OrdinalIgnoreCase)
        OtherProviderSentinelAbsent = -not (Test-Path -LiteralPath $absentProviderPath)
        ProviderStarted = $false
    }
    exit 0
}

& $cliPath daemon start --foreground `
    --daemon-id 'item14-integration-replay-20260915-v2' `
    --device-name 'Item 14 Replay V2 Isolated' `
    --runtime-name 'Item 14 Replay V2 Claude' `
    --workspaces-root $workspacesRoot `
    --agent-timeout '20m0s' `
    --max-concurrent-tasks 1 `
    --no-auto-update `
    --no-auto-reload `
    --profile 'item14-integration-replay-v2-20260915'
exit $LASTEXITCODE
