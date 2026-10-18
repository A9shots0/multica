param(
    [Parameter(Mandatory=$true)]
    [ValidateSet('Build','Start','Stop','Status')]
    [string]$Action
)
$ErrorActionPreference = 'Stop'
$candidateRoot = Split-Path -Parent $PSScriptRoot
$previousSourceId = $env:CANDIDATE_SOURCE_ID
Push-Location -LiteralPath $candidateRoot
try {
    $evidenceRoot = Join-Path $candidateRoot '.qualification-evidence\local-integration-20260915'
    New-Item -ItemType Directory -Path $evidenceRoot -Force | Out-Null
    $emptyEnv = Join-Path $evidenceRoot 'empty.env'
    if (-not (Test-Path -LiteralPath $emptyEnv)) {
        [IO.File]::WriteAllText($emptyEnv, '')
    }
    if ((Get-Item -LiteralPath $emptyEnv).Length -ne 0) {
        throw 'The candidate must use an empty environment file.'
    }
    if ($Action -eq 'Build') {
        $source = (& python scripts/local-candidate-source.py) | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0) { throw 'Could not fingerprint candidate source.' }
    } elseif ($Action -eq 'Start') {
        $source = (& python scripts/local-candidate-source.py --check) | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0) { throw 'Candidate source differs from the build manifest.' }
    } else {
        $source = Get-Content -LiteralPath (Join-Path $evidenceRoot 'candidate-source-manifest.json') -Raw | ConvertFrom-Json
    }
    $env:CANDIDATE_SOURCE_ID = $source.source_id
    $compose = @('compose','--env-file',$emptyEnv,'--project-name','smk-adoption-20260916',
                 '--file','docker-compose.adoption-20260916.yml','--parallel','1')
    $receiptPath = Join-Path $evidenceRoot 'candidate-build-receipt.json'
    $imageNames = @('smk-adoption-backend:20260916', 'smk-adoption-web:20260916', 'smk-adoption-gateway:20260916')
    if ($Action -eq 'Start') {
        $receipt = Get-Content -LiteralPath $receiptPath -Raw | ConvertFrom-Json
        if ($receipt.source_id -ne $source.source_id) { throw 'No successful build for this source.' }
        foreach ($imageName in $imageNames) {
            $actualImageId = & docker image inspect --format '{{.Id}}' $imageName
            if ($LASTEXITCODE -ne 0 -or $actualImageId -ne $receipt.images.$imageName) {
                throw "Built image changed or is missing: $imageName"
            }
        }
    }
    switch ($Action) {
        'Build'  {
            foreach ($service in @('backend','frontend','gateway')) {
                & docker @compose build $service
                if ($LASTEXITCODE -ne 0) { throw "Candidate $service build failed." }
            }
        }
        'Start'  { & docker @compose up -d --wait --wait-timeout 180 }
        'Stop'   { & docker @compose stop }
        'Status' { & docker @compose ps }
    }
    if ($LASTEXITCODE -ne 0) { throw "Candidate $Action failed; inspect the command output." }
    if ($Action -eq 'Build') {
        & python scripts/local-candidate-source.py --check
        if ($LASTEXITCODE -ne 0) { throw 'Source changed during the build. Rebuild before starting.' }
        $imageIds = [ordered]@{}
        foreach ($imageName in $imageNames) {
            $imageIds[$imageName] = & docker image inspect --format '{{.Id}}' $imageName
            if ($LASTEXITCODE -ne 0) { throw "Built image is missing: $imageName" }
        }
        [ordered]@{
            built_at = [DateTime]::UtcNow.ToString('o')
            source_id = $source.source_id
            images = $imageIds
        } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $receiptPath -Encoding utf8
    }
} finally {
    $env:CANDIDATE_SOURCE_ID = $previousSourceId
    Pop-Location
}
