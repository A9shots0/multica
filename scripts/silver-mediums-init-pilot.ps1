$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$examplePath = Join-Path $repoRoot ".env.example"
$envPath = Join-Path $repoRoot ".env"

if (-not (Test-Path -LiteralPath $examplePath)) {
    throw "Missing .env.example"
}

if (Test-Path -LiteralPath $envPath) {
    Write-Host "Pilot .env already exists; preserving it unchanged."
    exit 0
}

function New-RandomHex([int]$byteCount) {
    $bytes = New-Object byte[] $byteCount
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    return -join ($bytes | ForEach-Object { $_.ToString("x2") })
}

function New-RandomBase64([int]$byteCount) {
    $bytes = New-Object byte[] $byteCount
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    return [Convert]::ToBase64String($bytes)
}

$jwt = New-RandomHex 32
$postgresPassword = New-RandomHex 24
$vcsKey = New-RandomBase64 32
$content = Get-Content -LiteralPath $examplePath
$content = $content -replace '^JWT_SECRET=.*', "JWT_SECRET=$jwt"
$content = $content -replace '^POSTGRES_PASSWORD=.*', "POSTGRES_PASSWORD=$postgresPassword"
$content = $content -replace '^(DATABASE_URL=postgres://[^:]+:)[^@]*(@.*)', "`${1}$postgresPassword`${2}"
$content = $content -replace '^MULTICA_VCS_SECRET_KEY=.*', "MULTICA_VCS_SECRET_KEY=$vcsKey"
$content = $content -replace '^MULTICA_IMAGE_TAG=.*', 'MULTICA_IMAGE_TAG=v0.4.42'

# The pilot is loopback-only. Signup remains available only long enough for
# Drew to create the first local account; it is locked in the next phase.
$content = $content -replace '^ALLOW_SIGNUP=.*', 'ALLOW_SIGNUP=true'

$content | Set-Content -LiteralPath $envPath -Encoding utf8NoBOM
Write-Host "Created ignored pilot .env with generated secrets and pinned image v0.4.42."
