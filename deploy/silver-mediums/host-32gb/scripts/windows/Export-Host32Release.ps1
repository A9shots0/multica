[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$OutputDirectory
)

$ErrorActionPreference = "Stop"
$profileRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$repoRoot = (Resolve-Path (Join-Path $profileRoot "..\..\..")).Path
$manifestPath = Join-Path $profileRoot "release-manifest.json"
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$output = [IO.Path]::GetFullPath($OutputDirectory)
$sourceRevision = [string]$manifest.source_revision

if ($sourceRevision -notmatch '^[0-9a-f]{40}$') { throw "Release source revision is not a full Git commit." }
& git -C $repoRoot cat-file -e "$sourceRevision`^{commit}"
if ($LASTEXITCODE -ne 0) { throw "Release source revision does not exist in this repository." }
& git -C $repoRoot merge-base --is-ancestor $sourceRevision HEAD
if ($LASTEXITCODE -ne 0) { throw "Release source revision is not an ancestor of the package checkout." }
$workingStatus = & git -C $repoRoot status --porcelain=v1 --untracked-files=all
if ($LASTEXITCODE -ne 0) { throw "Could not inspect release checkout status." }
if ($workingStatus) { throw "Release checkout has uncommitted files; commit the reviewed candidate before export." }
& git -C $repoRoot diff --quiet $sourceRevision HEAD -- Dockerfile .dockerignore LICENSE NOTICE server docker
if ($LASTEXITCODE -ne 0) { throw "Backend build inputs changed after the pinned source revision." }

$unexpectedSecrets = @(Get-ChildItem -LiteralPath $profileRoot -File -Recurse -Force | Where-Object {
    ($_.Name -match '\.(pem|key|p12|pfx)$') -or
    ($_.Name -match '(^|\.)env($|\.)' -and $_.Name -notmatch '\.example$')
})
if ($unexpectedSecrets.Count -gt 0) {
    throw "Release profile contains a non-example environment or key file: $($unexpectedSecrets[0].FullName)"
}

if (Test-Path -LiteralPath $output) {
    $existing = @(Get-ChildItem -LiteralPath $output -Force)
    if ($existing.Count -gt 0) {
        throw "Output directory is not empty: $output"
    }
} else {
    New-Item -ItemType Directory -Path $output | Out-Null
}

foreach ($name in @("backend", "frontend", "postgres")) {
    $expected = [string]$manifest.images.$name
    $details = (& docker image inspect $expected --format '{{.Id}}|{{.Os}}|{{.Architecture}}').Trim()
    if ($LASTEXITCODE -ne 0) { throw "Pinned $name image is not available locally: $expected" }
    $parts = $details.Split('|')
    if ($parts.Count -ne 3 -or $parts[0] -ne $expected) {
        throw "Pinned $name image identity is ambiguous. Expected $expected; found $details."
    }
    if ($parts[1] -ne "linux" -or $parts[2] -ne "amd64") {
        throw "Pinned $name image is not linux/amd64: $details"
    }
    if ($name -eq "backend") {
        $imageMetadata = @(& docker image inspect $expected | ConvertFrom-Json)[0]
        if ($LASTEXITCODE -ne 0) { throw "Could not read pinned backend image metadata." }
        $imageRevision = [string]$imageMetadata.Config.Labels.'org.opencontainers.image.revision'
        if ($imageRevision -ne $sourceRevision) {
            throw "Pinned backend image provenance does not match release source revision."
        }
    }
}

$imageArchive = Join-Path $output "deployment-images.tar"
$imageSaveArguments = @(
    "image", "save", "--output", $imageArchive,
    $manifest.images.backend,
    $manifest.images.frontend,
    $manifest.images.postgres
)
& docker @imageSaveArguments
if ($LASTEXITCODE -ne 0) { throw "Docker image export failed." }

$packageRoot = Join-Path $output "multica-32gb-host"
New-Item -ItemType Directory -Path $packageRoot | Out-Null
Copy-Item -LiteralPath (Join-Path $repoRoot "docker-compose.selfhost.yml") -Destination $packageRoot
$deployDestination = Join-Path $packageRoot "deploy\silver-mediums"
New-Item -ItemType Directory -Path $deployDestination -Force | Out-Null
Copy-Item -LiteralPath $profileRoot -Destination (Join-Path $deployDestination "host-32gb") -Recurse

$receipt = [ordered]@{
    schema = 1
    created_at = [DateTimeOffset]::UtcNow.ToString("o")
    source_revision = $sourceRevision
    package_revision = (& git -C $repoRoot rev-parse HEAD).Trim()
    data_included = $false
    credentials_included = $false
    image_archive_sha256 = (Get-FileHash -LiteralPath $imageArchive -Algorithm SHA256).Hash.ToLowerInvariant()
}
$receiptJson = ($receipt | ConvertTo-Json) + "`n"
[IO.File]::WriteAllText(
    (Join-Path $output "export-receipt.json"),
    $receiptJson,
    [Text.UTF8Encoding]::new($false)
)

$hashes = Get-ChildItem -LiteralPath $output -File -Recurse |
    Where-Object { $_.Name -ne "SHA256SUMS" } |
    Sort-Object FullName |
    ForEach-Object {
        $relative = $_.FullName.Substring($output.TrimEnd([char[]]@('\', '/')).Length + 1).Replace('\', '/')
        $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        "$hash  $relative"
    }
$checksumText = ($hashes -join "`n") + "`n"
[IO.File]::WriteAllText(
    (Join-Path $output "SHA256SUMS"),
    $checksumText,
    [Text.UTF8Encoding]::new($false)
)

Write-Output "Release package created without database data or credentials: $output"
