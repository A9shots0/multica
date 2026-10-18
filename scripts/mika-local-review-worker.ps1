param([Parameter(Mandatory=$true)][string]$CodexPath)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
$profileName = 'mika-chat-review-20260924'
$profilePath = Join-Path $env:USERPROFILE ".multica\profiles\$profileName\config.json"
$profile = Get-Content -LiteralPath $profilePath -Raw | ConvertFrom-Json
if ($profile.server_url -ne 'http://127.0.0.1:3220' -or $profile.workspaces_root -ne 'C:/Dev/_work/mika-chat-worker-20260924') {
    throw 'Use only the disposable Mika review profile.'
}
if (-not (Test-Path -LiteralPath $CodexPath -PathType Leaf)) { throw 'Provide the installed Codex executable.' }
$workerBin = Join-Path $profile.workspaces_root 'bin'
New-Item -ItemType Directory -Force -Path $workerBin | Out-Null
$cliPath = Join-Path $workerBin 'multica.exe'
$sourceCli = Join-Path $repoRoot 'server\bin\mika-review-multica.exe'
$buildVersion = & $sourceCli --version
if ($LASTEXITCODE -ne 0 -or $buildVersion -match '^multica dev(?:\s|$)') {
    throw 'Build the review CLI with the Makefile version/commit ldflags before launch; an unstamped dev binary falsely warns that project context is unsupported.'
}
# Stop this named review daemon before rebuilding/replacing its executable.
Copy-Item -LiteralPath $sourceCli -Destination $cliPath
# The worker must resolve the same CLI as this checkout, not an older global install.
$env:PATH = "$workerBin;$env:PATH"
$probeSource = Get-Content -LiteralPath (Join-Path $repoRoot 'server\internal\daemon\agents_probe.go') -Raw
[regex]::Matches($probeSource, 'MULTICA_[A-Z0-9_]+_PATH') | ForEach-Object { $_.Value } | Sort-Object -Unique | ForEach-Object {
    [Environment]::SetEnvironmentVariable($_, (Join-Path $profile.workspaces_root 'disabled-provider.exe'), 'Process')
}
$env:MULTICA_CODEX_PATH = (Resolve-Path -LiteralPath $CodexPath).Path
$env:MULTICA_GC_ENABLED = 'false'
& $cliPath --profile $profileName daemon start --foreground --daemon-id mika-chat-review-isolated-20260924 --max-concurrent-tasks 1 --agent-timeout 5m --no-auto-update --no-auto-reload
exit $LASTEXITCODE
