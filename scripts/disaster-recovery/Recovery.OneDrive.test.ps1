#!/usr/bin/env pwsh
# Off-machine readback waiting behaviour. No OneDrive, network or real backup.
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force

function Assert-True {
    param([bool] $Condition, [string] $Message)
    if (-not $Condition) { throw "FAIL: $Message" }
}

function Assert-Equal {
    param($Expected, $Actual, [string] $Message)
    if ("$Expected" -ne "$Actual") { throw "FAIL: $Message (expected '$Expected', got '$Actual')" }
}

# A scripted arrival: each entry is one observation the waiter will see.
function New-Waiter {
    param([object[]] $Observations, [string] $Hash, [string[]] $HashSequence, [switch] $KeepChanging)

    $state = [pscustomobject]@{ Index = 0; Now = [DateTimeOffset]::new(2026, 9, 16, 0, 0, 0, [TimeSpan]::Zero); Slept = 0; HashCalls = 0 }
    return @{
        State = $state
        Observer = {
            param($path)
            if ($state.Index -lt $Observations.Count) { $entry = $Observations[$state.Index] }
            else {
                $last = $Observations[$Observations.Count - 1]
                # A file OneDrive is still rewriting keeps getting a new stamp.
                $stamp = if ($KeepChanging) { [long]$last.Stamp + $state.Index } else { [long]$last.Stamp }
                $entry = @{ Exists = $last.Exists; Length = $last.Length; Stamp = $stamp }
            }
            $state.Index++
            return $entry
        }.GetNewClosure()
        Hasher = {
            param($path)
            $state.HashCalls++
            if ($HashSequence) {
                $value = if ($state.HashCalls -le $HashSequence.Count) { $HashSequence[$state.HashCalls - 1] } else { $HashSequence[$HashSequence.Count - 1] }
                if ($value -eq "throw") { throw "file is in use" }
                return $value
            }
            return $Hash
        }.GetNewClosure()
        Waiter = {
            param($seconds)
            $state.Slept += $seconds
            $state.Now = $state.Now.AddSeconds($seconds)
        }.GetNewClosure()
        Clock = { return $state.Now }.GetNewClosure()
    }
}

function Invoke-Wait {
    param($Harness, [long] $ExpectedBytes, [string] $ExpectedSha256, [int] $TimeoutSeconds = 60, [int] $PollSeconds = 5)
    return Wait-ForVerifiedLocalCopy -Path "C:/fake/artifact.smkdr" -ExpectedBytes $ExpectedBytes `
        -ExpectedSha256 $ExpectedSha256 -TimeoutSeconds $TimeoutSeconds -PollSeconds $PollSeconds `
        -Observer $Harness.Observer -Hasher $Harness.Hasher -Waiter $Harness.Waiter -Clock $Harness.Clock
}

$expected = "a" * 64
$absent = @{ Exists = $false; Length = [long]-1; Stamp = [long]-1 }

try {
    # 1. The reported defect: a large artifact is still arriving on the first read.
    $harness = New-Waiter -Observations @(
        $absent,
        @{ Exists = $true; Length = [long]120; Stamp = [long]10 },
        @{ Exists = $true; Length = [long]400; Stamp = [long]20 },
        @{ Exists = $true; Length = [long]400; Stamp = [long]20 }
    ) -Hash $expected
    $result = Invoke-Wait -Harness $harness -ExpectedBytes 400 -ExpectedSha256 $expected
    Assert-Equal "verified" $result.Status "a still-arriving artifact was not awaited"
    Assert-Equal 4 $result.Attempts "unexpected observation count while waiting"
    Assert-Equal 1 $harness.State.HashCalls "the artifact was hashed before it settled"
    Write-Host "PASS: a still-syncing artifact is awaited instead of failing its first hash read"

    # 2. The hash requirement is not weakened: a settled, complete, different copy fails.
    $harness = New-Waiter -Observations @(
        @{ Exists = $true; Length = [long]400; Stamp = [long]20 },
        @{ Exists = $true; Length = [long]400; Stamp = [long]20 }
    ) -Hash ("b" * 64)
    $result = Invoke-Wait -Harness $harness -ExpectedBytes 400 -ExpectedSha256 $expected
    Assert-Equal "mismatch" $result.Status "a settled non-matching copy was not reported as a mismatch"
    Assert-Equal "local_cloud_copy_hash_mismatch" $result.Reason "mismatch reason changed"
    Write-Host "PASS: a settled copy with a different hash still fails"

    # 3. A real mismatch is not hidden behind the whole timeout.
    Assert-True ($harness.State.Slept -le 5) "a genuine mismatch waited out the full timeout"
    Write-Host "PASS: a genuine mismatch is reported promptly, not after the timeout"

    # 4. Waiting is bounded and truthful when the copy never completes.
    $harness = New-Waiter -Observations @(@{ Exists = $true; Length = [long]399; Stamp = [long]20 }) -Hash $expected
    $result = Invoke-Wait -Harness $harness -ExpectedBytes 400 -ExpectedSha256 $expected -TimeoutSeconds 30
    Assert-Equal "sync_pending" $result.Status "an incomplete copy was not reported as pending"
    Assert-Equal "local_copy_incomplete" $result.Reason "incomplete reason changed"
    Assert-True ($result.WaitedSeconds -ge 30) "the waiter returned before its deadline"
    Assert-Equal 0 $harness.State.HashCalls "an incomplete copy was hashed"
    Write-Host "PASS: an incomplete copy times out as sync_pending without a false mismatch"

    # 5. An absent copy is pending, never a mismatch.
    $harness = New-Waiter -Observations @($absent) -Hash $expected
    $result = Invoke-Wait -Harness $harness -ExpectedBytes 400 -ExpectedSha256 $expected -TimeoutSeconds 10
    Assert-Equal "sync_pending" $result.Status "an absent copy was not reported as pending"
    Assert-Equal "local_copy_absent" $result.Reason "absent reason changed"
    Write-Host "PASS: an absent copy is pending, never a mismatch"

    # 6. A copy whose size is right but is still being rewritten keeps waiting.
    $harness = New-Waiter -Observations @(
        @{ Exists = $true; Length = [long]400; Stamp = [long]10 },
        @{ Exists = $true; Length = [long]400; Stamp = [long]11 },
        @{ Exists = $true; Length = [long]400; Stamp = [long]12 }
    ) -Hash ("b" * 64) -KeepChanging
    $result = Invoke-Wait -Harness $harness -ExpectedBytes 400 -ExpectedSha256 $expected -TimeoutSeconds 20
    Assert-Equal "sync_pending" $result.Status "a still-changing copy was called a mismatch"
    Assert-Equal "local_copy_still_changing" $result.Reason "still-changing reason changed"
    Assert-Equal 0 $harness.State.HashCalls "a still-changing copy was hashed"
    Write-Host "PASS: a complete-sized copy that is still changing keeps waiting"

    # 7. A transient unreadable file is retried, then succeeds.
    $harness = New-Waiter -Observations @(
        @{ Exists = $true; Length = [long]400; Stamp = [long]20 },
        @{ Exists = $true; Length = [long]400; Stamp = [long]20 },
        @{ Exists = $true; Length = [long]400; Stamp = [long]20 }
    ) -HashSequence @("throw", $expected)
    $result = Invoke-Wait -Harness $harness -ExpectedBytes 400 -ExpectedSha256 $expected -TimeoutSeconds 60
    Assert-Equal "verified" $result.Status "a transient read error was treated as terminal"
    Assert-Equal 2 $harness.State.HashCalls "the transient read error was not retried exactly once"
    Write-Host "PASS: a transient unreadable copy is retried rather than failed"

    # 8. A permanently unreadable copy times out honestly.
    $harness = New-Waiter -Observations @(@{ Exists = $true; Length = [long]400; Stamp = [long]20 }) -HashSequence @("throw")
    $result = Invoke-Wait -Harness $harness -ExpectedBytes 400 -ExpectedSha256 $expected -TimeoutSeconds 15
    Assert-Equal "sync_pending" $result.Status "an unreadable copy was not reported as pending"
    Assert-Equal "local_copy_unreadable" $result.Reason "unreadable reason changed"
    Write-Host "PASS: a permanently unreadable copy times out as sync_pending"

    # 9. The real observer reports real bytes from disk.
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("smk-dr-onedrive-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $temporary | Out-Null
    try {
        $file = Join-Path $temporary "artifact.bin"
        [IO.File]::WriteAllBytes($file, [byte[]](1..64))
        $observation = Get-LocalCopyObservation -Path $file
        Assert-True $observation.Exists "the real observer did not see an existing file"
        Assert-Equal 64 $observation.Length "the real observer reported the wrong size"
        $missing = Get-LocalCopyObservation -Path (Join-Path $temporary "absent.bin")
        Assert-True (-not $missing.Exists) "the real observer claimed an absent file exists"
        $real = Wait-ForVerifiedLocalCopy -Path $file -ExpectedBytes 64 -ExpectedSha256 (Get-Sha256 $file) -TimeoutSeconds 30 -PollSeconds 1
        Assert-Equal "verified" $real.Status "a real settled file did not verify"
        Write-Host "PASS: the real filesystem observer verifies a settled artifact"
    }
    finally {
        if (Test-Path -LiteralPath $temporary) { [IO.Directory]::Delete($temporary, $true) }
    }

    # 10. PowerShell 7 resolution does not depend on PATH.
    $resolved = Get-PowerShell7Path
    Assert-True (Test-Path -LiteralPath $resolved -PathType Leaf) "resolved PowerShell 7 path does not exist"
    $major = (& $resolved -NoProfile -NonInteractive -Command '$PSVersionTable.PSVersion.Major' | Select-Object -First 1)
    Assert-True ([int]$major -ge 7) "resolved interpreter is not PowerShell 7 or newer"
    Write-Host "PASS: PowerShell 7 resolves without relying on PATH"

    # 11. An explicit override that cannot answer is an operator mistake and
    # must fail loudly rather than quietly resolving something else.
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("smk-dr-pwsh-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $temporary | Out-Null
    try {
        $previousOverride = $env:MULTICA_POWERSHELL7
        try {
            $env:MULTICA_POWERSHELL7 = Join-Path $temporary "absent-pwsh.exe"
            Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force
            $rejected = $false
            try { [void](Get-PowerShell7Path) } catch { $rejected = $_.Exception.Message -match "MULTICA_POWERSHELL7 is set to" }
            Assert-True $rejected "a missing override silently fell through to another interpreter"
            Write-Host "PASS: a missing MULTICA_POWERSHELL7 override fails loudly"

            # A Microsoft Store app-execution alias is a zero-length reparse
            # point: it satisfies Test-Path and then blocks on the Store.
            $alias = Join-Path $temporary "store-alias-pwsh.exe"
            [IO.File]::WriteAllBytes($alias, [byte[]]::new(0))
            $env:MULTICA_POWERSHELL7 = $alias
            Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force
            $rejected = $false
            try { [void](Get-PowerShell7Path) } catch { $rejected = $_.Exception.Message -match "MULTICA_POWERSHELL7 is set to" }
            Assert-True $rejected "a zero-length Store alias was accepted as PowerShell 7"
            Write-Host "PASS: a zero-length Store alias is rejected instead of launched"
        }
        finally {
            if ($null -eq $previousOverride) { Remove-Item -LiteralPath "Env:\MULTICA_POWERSHELL7" -ErrorAction SilentlyContinue }
            else { $env:MULTICA_POWERSHELL7 = $previousOverride }
            Import-Module (Join-Path $PSScriptRoot "Recovery.Common.psm1") -Force
        }
    }
    finally {
        if (Test-Path -LiteralPath $temporary) { [IO.Directory]::Delete($temporary, $true) }
    }

    Write-Host "All OneDrive readback waiting checks passed."
}
catch {
    Write-Host $_.Exception.Message
    exit 1
}
