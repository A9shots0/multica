Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:RecoveryMagic = [Text.Encoding]::ASCII.GetBytes("SMKDR001")
$script:RecoveryKdfIterations = 600000
$script:RecoveryManifestVersion = 1

function Write-AtomicJson {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string] $Path,
        [Parameter(Mandatory)] $Value
    )

    $directory = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $directory)) {
        New-Item -ItemType Directory -Path $directory -Force | Out-Null
    }
    $temporary = "$Path.tmp-$([Guid]::NewGuid().ToString('N'))"
    try {
        $Value | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $temporary -Encoding utf8NoBOM
        Move-Item -LiteralPath $temporary -Destination $Path -Force
    }
    finally {
        if (Test-Path -LiteralPath $temporary) {
            Remove-Item -LiteralPath $temporary -Force
        }
    }
}

function Get-RecoveryPassphrase {
    [CmdletBinding()]
    param([string] $PassphraseFile)

    $value = [Environment]::GetEnvironmentVariable("MULTICA_RECOVERY_PASSPHRASE")
    if ([string]::IsNullOrWhiteSpace($value) -and $PassphraseFile) {
        if (-not (Test-Path -LiteralPath $PassphraseFile -PathType Leaf)) {
            throw "Recovery passphrase file does not exist."
        }
        $value = (Get-Content -LiteralPath $PassphraseFile -Raw).TrimEnd("`r", "`n")
    }
    if ([string]::IsNullOrWhiteSpace($value) -or $value.Length -lt 20) {
        throw "Set MULTICA_RECOVERY_PASSPHRASE or provide a protected passphrase file with at least 20 characters."
    }
    return $value
}

function Get-Sha256 {
    [CmdletBinding()]
    param([Parameter(Mandatory)] [string] $Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-ComposeSha256 {
    [CmdletBinding()]
    param([Parameter(Mandatory)] [string] $Path)

    # Compose files are reviewed as UTF-8 text. Normalize line endings so a
    # Windows checkout cannot fail an identity reviewed from an LF checkout.
    $text = [IO.File]::ReadAllText($Path)
    $normalized = $text.Replace("`r`n", "`n").Replace("`r", "`n")
    $bytes = [Text.Encoding]::UTF8.GetBytes($normalized)
    $hasher = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($hasher.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $hasher.Dispose()
    }
}

$script:RecoveryPowerShell7 = $null

function Get-PowerShell7Path {
    <#
        Recovery tooling must not assume `pwsh` is on PATH. Candidates are tried
        in a fixed order and each one has to report its own major version, so
        Windows PowerShell 5.1 can never silently satisfy this.
    #>
    [CmdletBinding()]
    param()

    if ($script:RecoveryPowerShell7) { return $script:RecoveryPowerShell7 }

    # The probe runs as a bounded child process. A Microsoft Store app-execution
    # alias is a zero-length reparse point that satisfies Test-Path and then
    # opens the Store instead of answering, which would hang an unattended
    # scheduled run indefinitely.
    function Test-PowerShell7Candidate {
        param([string] $Path)
        $item = Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
        if ($null -eq $item -or $item.Length -eq 0) { return $false }
        $info = [Diagnostics.ProcessStartInfo]::new()
        $info.FileName = $Path
        $info.UseShellExecute = $false
        $info.RedirectStandardOutput = $true
        $info.RedirectStandardError = $true
        $info.CreateNoWindow = $true
        foreach ($argument in @("-NoProfile", "-NonInteractive", "-Command", '$PSVersionTable.PSVersion.Major')) {
            [void]$info.ArgumentList.Add($argument)
        }
        $process = [Diagnostics.Process]::new()
        $process.StartInfo = $info
        try {
            if (-not $process.Start()) { return $false }
            $reader = $process.StandardOutput.ReadToEndAsync()
            if (-not $process.WaitForExit(20000)) {
                $process.Kill($true)
                [void]$process.WaitForExit(5000)
                return $false
            }
            if ($process.ExitCode -ne 0) { return $false }
            $major = 0
            return [int]::TryParse($reader.GetAwaiter().GetResult().Trim(), [ref]$major) -and $major -ge 7
        }
        catch { return $false }
        finally { $process.Dispose() }
    }

    # An explicit override that does not answer is an operator mistake, not a
    # hint to look elsewhere.
    if (-not [string]::IsNullOrWhiteSpace($env:MULTICA_POWERSHELL7)) {
        if (-not (Test-PowerShell7Candidate $env:MULTICA_POWERSHELL7)) {
            throw "MULTICA_POWERSHELL7 is set to '$($env:MULTICA_POWERSHELL7)', which is not a working PowerShell 7 executable."
        }
        $script:RecoveryPowerShell7 = $env:MULTICA_POWERSHELL7
        return $script:RecoveryPowerShell7
    }

    $candidates = [Collections.Generic.List[string]]::new()
    $add = {
        param($value)
        if ([string]::IsNullOrWhiteSpace($value)) { return }
        $text = [string]$value
        if (-not $candidates.Contains($text)) { [void]$candidates.Add($text) }
    }
    & $add (Get-Command pwsh.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source)
    if ($PSHOME) { & $add (Join-Path $PSHOME "pwsh.exe") }
    foreach ($root in @($env:ProgramFiles, ${env:ProgramW6432}, ${env:ProgramFiles(x86)}, $env:LOCALAPPDATA, $env:ProgramData)) {
        if ([string]::IsNullOrWhiteSpace($root)) { continue }
        foreach ($pattern in @("PowerShell\*\pwsh.exe", "Microsoft\PowerShell\*\pwsh.exe", "Microsoft\WindowsApps\pwsh.exe", "chocolatey\bin\pwsh.exe")) {
            foreach ($match in @(Get-ChildItem -Path (Join-Path $root $pattern) -ErrorAction SilentlyContinue | Sort-Object FullName -Descending)) {
                & $add $match.FullName
            }
        }
    }
    $home7 = [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
    if (-not [string]::IsNullOrWhiteSpace($home7)) {
        foreach ($pattern in @(".cache\codex-runtimes\*\dependencies\native\powershell\pwsh.exe", ".dotnet\tools\pwsh.exe")) {
            foreach ($match in @(Get-ChildItem -Path (Join-Path $home7 $pattern) -ErrorAction SilentlyContinue | Sort-Object FullName)) {
                & $add $match.FullName
            }
        }
    }
    foreach ($candidate in $candidates) {
        if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) { continue }
        if (-not (Test-PowerShell7Candidate $candidate)) { continue }
        $script:RecoveryPowerShell7 = $candidate
        return $candidate
    }
    $searched = if ($candidates.Count -gt 0) { $candidates -join "; " } else { "no candidate path" }
    throw "PowerShell 7 is required by the recovery tooling and was not found. Searched: $searched. Set MULTICA_POWERSHELL7 to the exact pwsh.exe."
}

function Get-LocalCopyObservation {
    [CmdletBinding()]
    param([Parameter(Mandatory)] [string] $Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return @{ Exists = $false; Length = [long]-1; Stamp = [long]-1 }
    }
    $item = Get-Item -LiteralPath $Path -Force
    return @{ Exists = $true; Length = [long]$item.Length; Stamp = [long]$item.LastWriteTimeUtc.Ticks }
}

function Wait-ForVerifiedLocalCopy {
    <#
        A large encrypted artifact is still being written or reconciled when the
        backup step returns, so hashing it once is a predictable failure rather
        than evidence. This waits for the copy to reach its recorded size and
        stop changing, and only then compares the hash. The hash requirement
        itself is unchanged: a complete, settled copy that does not match is
        still a failure, and it is reported immediately.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string] $Path,
        [Parameter(Mandatory)] [long] $ExpectedBytes,
        [Parameter(Mandatory)] [string] $ExpectedSha256,
        # Settling needs two observations, so a caller cannot ask for a verdict
        # in less time than one poll interval.
        [Parameter(Mandatory)] [ValidateRange(1, 7200)] [int] $TimeoutSeconds,
        [ValidateRange(1, 600)] [int] $PollSeconds = 5,
        [scriptblock] $Observer,
        [scriptblock] $Hasher,
        [scriptblock] $Waiter,
        [scriptblock] $Clock
    )

    if (-not $Observer) { $Observer = { param($p) Get-LocalCopyObservation -Path $p } }
    if (-not $Hasher) { $Hasher = { param($p) Get-Sha256 $p } }
    if (-not $Waiter) { $Waiter = { param($seconds) Start-Sleep -Seconds $seconds } }
    if (-not $Clock) { $Clock = { [DateTimeOffset]::UtcNow } }

    $started = & $Clock
    $attempts = 0
    $reason = "local_copy_absent"
    $observedBytes = [long]-1
    $previousStamp = [long]::MinValue
    $previousLength = [long]::MinValue
    while ($true) {
        $attempts++
        $observation = & $Observer $Path
        $observedBytes = [long]$observation.Length
        $settled = $observation.Exists -and
                   [long]$observation.Length -eq $ExpectedBytes -and
                   [long]$observation.Length -eq $previousLength -and
                   [long]$observation.Stamp -eq $previousStamp
        if ($settled) {
            $actual = $null
            try { $actual = & $Hasher $Path }
            catch { $actual = $null }
            if ($null -eq $actual) {
                $reason = "local_copy_unreadable"
            }
            elseif ([string]$actual -eq $ExpectedSha256) {
                return @{
                    Status = "verified"; Reason = "local_copy_hash_match"; Attempts = $attempts
                    WaitedSeconds = [Math]::Round(((& $Clock) - $started).TotalSeconds, 3); ObservedBytes = $observedBytes
                }
            }
            else {
                # Complete, unchanged and different: a real mismatch, not sync lag.
                return @{
                    Status = "mismatch"; Reason = "local_cloud_copy_hash_mismatch"; Attempts = $attempts
                    WaitedSeconds = [Math]::Round(((& $Clock) - $started).TotalSeconds, 3); ObservedBytes = $observedBytes
                }
            }
        }
        elseif (-not $observation.Exists) { $reason = "local_copy_absent" }
        elseif ([long]$observation.Length -ne $ExpectedBytes) { $reason = "local_copy_incomplete" }
        else { $reason = "local_copy_still_changing" }
        $previousLength = [long]$observation.Length
        $previousStamp = [long]$observation.Stamp
        if (((& $Clock) - $started).TotalSeconds -ge $TimeoutSeconds) {
            return @{
                Status = "sync_pending"; Reason = $reason; Attempts = $attempts
                WaitedSeconds = [Math]::Round(((& $Clock) - $started).TotalSeconds, 3); ObservedBytes = $observedBytes
            }
        }
        & $Waiter $PollSeconds
    }
}

function Get-RecoveryKeys {
    param(
        [Parameter(Mandatory)] [string] $Passphrase,
        [Parameter(Mandatory)] [byte[]] $Salt
    )
    $derive = [Security.Cryptography.Rfc2898DeriveBytes]::new(
        $Passphrase,
        $Salt,
        $script:RecoveryKdfIterations,
        [Security.Cryptography.HashAlgorithmName]::SHA256
    )
    try {
        $bytes = $derive.GetBytes(64)
        return @{
            Encryption = $bytes[0..31]
            Authentication = $bytes[32..63]
        }
    }
    finally {
        $derive.Dispose()
    }
}

function Test-ConstantTimeEqual {
    param([byte[]] $Left, [byte[]] $Right)
    if ($null -eq $Left -or $null -eq $Right -or $Left.Length -ne $Right.Length) {
        return $false
    }
    [byte] $difference = 0
    for ($index = 0; $index -lt $Left.Length; $index++) {
        $difference = $difference -bor ($Left[$index] -bxor $Right[$index])
    }
    return $difference -eq 0
}

function Protect-RecoveryArchive {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string] $InputPath,
        [Parameter(Mandatory)] [string] $OutputPath,
        [Parameter(Mandatory)] [string] $Passphrase
    )

    $salt = [byte[]]::new(16)
    $iv = [byte[]]::new(16)
    [Security.Cryptography.RandomNumberGenerator]::Fill($salt)
    [Security.Cryptography.RandomNumberGenerator]::Fill($iv)
    $keys = Get-RecoveryKeys -Passphrase $Passphrase -Salt $salt
    $cipherPath = "$OutputPath.cipher-$([Guid]::NewGuid().ToString('N'))"
    $temporary = "$OutputPath.tmp-$([Guid]::NewGuid().ToString('N'))"

    try {
        $aes = [Security.Cryptography.Aes]::Create()
        $aes.KeySize = 256
        $aes.Mode = [Security.Cryptography.CipherMode]::CBC
        $aes.Padding = [Security.Cryptography.PaddingMode]::PKCS7
        $aes.Key = $keys.Encryption
        $aes.IV = $iv
        $input = [IO.File]::OpenRead($InputPath)
        $cipher = [IO.File]::Create($cipherPath)
        try {
            $encryptor = $aes.CreateEncryptor()
            $crypto = [Security.Cryptography.CryptoStream]::new(
                $cipher,
                $encryptor,
                [Security.Cryptography.CryptoStreamMode]::Write
            )
            try { $input.CopyTo($crypto) } finally { $crypto.Dispose(); $encryptor.Dispose() }
        }
        finally {
            $input.Dispose()
            $cipher.Dispose()
            $aes.Dispose()
        }

        $header = [byte[]]::new($script:RecoveryMagic.Length + $salt.Length + $iv.Length)
        [Array]::Copy($script:RecoveryMagic, 0, $header, 0, $script:RecoveryMagic.Length)
        [Array]::Copy($salt, 0, $header, $script:RecoveryMagic.Length, $salt.Length)
        [Array]::Copy($iv, 0, $header, $script:RecoveryMagic.Length + $salt.Length, $iv.Length)

        $hmac = [Security.Cryptography.HMACSHA256]::new($keys.Authentication)
        try {
            [void] $hmac.TransformBlock($header, 0, $header.Length, $null, 0)
            $cipherRead = [IO.File]::OpenRead($cipherPath)
            try {
                $buffer = [byte[]]::new(1048576)
                while (($read = $cipherRead.Read($buffer, 0, $buffer.Length)) -gt 0) {
                    [void] $hmac.TransformBlock($buffer, 0, $read, $null, 0)
                }
                [void] $hmac.TransformFinalBlock([byte[]]::new(0), 0, 0)
            }
            finally { $cipherRead.Dispose() }

            $output = [IO.File]::Create($temporary)
            try {
                $output.Write($header, 0, $header.Length)
                $cipherRead = [IO.File]::OpenRead($cipherPath)
                try { $cipherRead.CopyTo($output) } finally { $cipherRead.Dispose() }
                $output.Write($hmac.Hash, 0, $hmac.Hash.Length)
                $output.Flush($true)
            }
            finally { $output.Dispose() }
        }
        finally { $hmac.Dispose() }

        Move-Item -LiteralPath $temporary -Destination $OutputPath -Force
    }
    finally {
        foreach ($path in @($cipherPath, $temporary)) {
            if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Force }
        }
        [Array]::Clear($keys.Encryption, 0, $keys.Encryption.Length)
        [Array]::Clear($keys.Authentication, 0, $keys.Authentication.Length)
    }
}

function Unprotect-RecoveryArchive {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string] $InputPath,
        [Parameter(Mandatory)] [string] $OutputPath,
        [Parameter(Mandatory)] [string] $Passphrase
    )

    $headerLength = $script:RecoveryMagic.Length + 16 + 16
    $tagLength = 32
    $input = [IO.File]::OpenRead($InputPath)
    try {
        if ($input.Length -le ($headerLength + $tagLength)) { throw "Recovery artifact is truncated." }
        $header = [byte[]]::new($headerLength)
        if ($input.Read($header, 0, $header.Length) -ne $header.Length) { throw "Recovery artifact header is truncated." }
        $magic = $header[0..($script:RecoveryMagic.Length - 1)]
        if (-not (Test-ConstantTimeEqual $magic $script:RecoveryMagic)) { throw "Recovery artifact format is not supported." }
        $salt = $header[$script:RecoveryMagic.Length..($script:RecoveryMagic.Length + 15)]
        $iv = $header[($script:RecoveryMagic.Length + 16)..($headerLength - 1)]
        $cipherLength = $input.Length - $headerLength - $tagLength
        $keys = Get-RecoveryKeys -Passphrase $Passphrase -Salt $salt

        $hmac = [Security.Cryptography.HMACSHA256]::new($keys.Authentication)
        try {
            [void] $hmac.TransformBlock($header, 0, $header.Length, $null, 0)
            $buffer = [byte[]]::new(1048576)
            [long] $remaining = $cipherLength
            while ($remaining -gt 0) {
                $wanted = [Math]::Min([long]$buffer.Length, $remaining)
                $read = $input.Read($buffer, 0, [int]$wanted)
                if ($read -le 0) { throw "Recovery artifact ciphertext is truncated." }
                [void] $hmac.TransformBlock($buffer, 0, $read, $null, 0)
                $remaining -= $read
            }
            [void] $hmac.TransformFinalBlock([byte[]]::new(0), 0, 0)
            $storedTag = [byte[]]::new($tagLength)
            if ($input.Read($storedTag, 0, $tagLength) -ne $tagLength) { throw "Recovery artifact authentication tag is truncated." }
            if (-not (Test-ConstantTimeEqual $hmac.Hash $storedTag)) {
                throw "Recovery artifact authentication failed. Nothing was restored."
            }
        }
        finally { $hmac.Dispose() }
    }
    finally { $input.Dispose() }

    $temporary = "$OutputPath.tmp-$([Guid]::NewGuid().ToString('N'))"
    try {
        $input = [IO.File]::OpenRead($InputPath)
        [void] $input.Seek($headerLength, [IO.SeekOrigin]::Begin)
        $limited = [RecoveryLimitedReadStream]::new($input, $cipherLength)
        $aes = [Security.Cryptography.Aes]::Create()
        $aes.KeySize = 256
        $aes.Mode = [Security.Cryptography.CipherMode]::CBC
        $aes.Padding = [Security.Cryptography.PaddingMode]::PKCS7
        $aes.Key = $keys.Encryption
        $aes.IV = $iv
        $output = [IO.File]::Create($temporary)
        try {
            $decryptor = $aes.CreateDecryptor()
            $crypto = [Security.Cryptography.CryptoStream]::new($limited, $decryptor, [Security.Cryptography.CryptoStreamMode]::Read)
            try { $crypto.CopyTo($output); $output.Flush($true) } finally { $crypto.Dispose(); $decryptor.Dispose() }
        }
        finally { $output.Dispose(); $aes.Dispose(); $limited.Dispose(); $input.Dispose() }
        Move-Item -LiteralPath $temporary -Destination $OutputPath -Force
    }
    finally {
        if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Force }
        if ($keys) {
            [Array]::Clear($keys.Encryption, 0, $keys.Encryption.Length)
            [Array]::Clear($keys.Authentication, 0, $keys.Authentication.Length)
        }
    }
}

function Enter-RecoveryLock {
    [CmdletBinding()]
    param([Parameter(Mandatory)] [string] $Path)
    $directory = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $directory)) { New-Item -ItemType Directory -Path $directory -Force | Out-Null }
    for ($attempt = 0; $attempt -lt 2; $attempt++) {
        try {
            $stream = [IO.File]::Open($Path, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
            $bytes = [Text.Encoding]::UTF8.GetBytes("pid=$PID`nstarted=$([DateTimeOffset]::UtcNow.ToString('o'))`n")
            $stream.Write($bytes, 0, $bytes.Length)
            $stream.Flush($true)
            return $stream
        }
        catch [IO.IOException] {
            if ($attempt -gt 0) { break }
            try {
                $stale = [IO.File]::Open($Path, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
                $stale.Dispose()
                [IO.File]::Delete($Path)
            }
            catch [IO.IOException] {
                throw "Another backup or restore holds the recovery lock."
            }
        }
    }
    throw "Another backup or restore holds the recovery lock."
}

function Exit-RecoveryLock {
    param($Stream, [string] $Path)
    if ($Stream) { $Stream.Dispose() }
    if (Test-Path -LiteralPath $Path) { Remove-Item -LiteralPath $Path -Force }
}

function Invoke-RecoveryProcess {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string] $FilePath,
        [Parameter(Mandatory)] [string[]] $Arguments,
        [string] $WorkingDirectory,
        [string] $StandardOutputPath
    )
    $info = [Diagnostics.ProcessStartInfo]::new()
    $info.FileName = $FilePath
    $info.UseShellExecute = $false
    $info.RedirectStandardError = $true
    $info.RedirectStandardOutput = $true
    $info.CreateNoWindow = $true
    if ($WorkingDirectory) { $info.WorkingDirectory = $WorkingDirectory }
    foreach ($argument in $Arguments) { [void] $info.ArgumentList.Add($argument) }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $info
    if (-not $process.Start()) { throw "Could not start required recovery command." }
    $errorTask = $process.StandardError.ReadToEndAsync()
    if ($StandardOutputPath) {
        $output = [IO.File]::Create($StandardOutputPath)
        try { $process.StandardOutput.BaseStream.CopyTo($output); $output.Flush($true) } finally { $output.Dispose() }
    }
    else { $stdout = $process.StandardOutput.ReadToEnd() }
    $process.WaitForExit()
    $stderr = $errorTask.GetAwaiter().GetResult()
    if ($process.ExitCode -ne 0) {
        $safeError = if ($stderr.Length -gt 1000) { $stderr.Substring(0, 1000) } else { $stderr }
        throw "Required recovery command failed ($($process.ExitCode)): $safeError"
    }
    if (-not $StandardOutputPath) { return $stdout }
}

function Get-RecoveryManifestVersion { return $script:RecoveryManifestVersion }

class RecoveryLimitedReadStream : IO.Stream {
    [IO.Stream] $Inner
    [long] $Remaining
    RecoveryLimitedReadStream([IO.Stream] $inner, [long] $length) { $this.Inner = $inner; $this.Remaining = $length }
    [bool] get_CanRead() { return $true }
    [bool] get_CanSeek() { return $false }
    [bool] get_CanWrite() { return $false }
    [long] get_Length() { return $this.Remaining }
    [long] get_Position() { return 0 }
    [void] set_Position([long] $value) { throw [NotSupportedException]::new() }
    [void] Flush() {}
    [int] Read([byte[]] $buffer, [int] $offset, [int] $count) {
        if ($this.Remaining -le 0) { return 0 }
        $wanted = [Math]::Min([long]$count, $this.Remaining)
        $read = $this.Inner.Read($buffer, $offset, [int]$wanted)
        $this.Remaining -= $read
        return $read
    }
    [long] Seek([long] $offset, [IO.SeekOrigin] $origin) { throw [NotSupportedException]::new() }
    [void] SetLength([long] $value) { throw [NotSupportedException]::new() }
    [void] Write([byte[]] $buffer, [int] $offset, [int] $count) { throw [NotSupportedException]::new() }
}

Export-ModuleMember -Function @(
    "Write-AtomicJson",
    "Get-RecoveryPassphrase",
    "Get-Sha256",
    "Get-ComposeSha256",
    "Get-PowerShell7Path",
    "Get-LocalCopyObservation",
    "Wait-ForVerifiedLocalCopy",
    "Protect-RecoveryArchive",
    "Unprotect-RecoveryArchive",
    "Enter-RecoveryLock",
    "Exit-RecoveryLock",
    "Invoke-RecoveryProcess",
    "Get-RecoveryManifestVersion"
)
