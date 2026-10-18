#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [string] $Path = (Join-Path $env:USERPROFILE ".multica-recovery/recovery.key"),
    [switch] $Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Path = [IO.Path]::GetFullPath($Path)
if ((Test-Path -LiteralPath $Path) -and -not $Force) {
    throw "Recovery key already exists. Refusing to replace it because existing backups depend on it."
}
$directory = Split-Path -Parent $Path
New-Item -ItemType Directory -Path $directory -Force | Out-Null
$bytes = [byte[]]::new(48)
[Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
$secret = [Convert]::ToBase64String($bytes)
$temporary = "$Path.tmp-$([Guid]::NewGuid().ToString('N'))"
try {
    [IO.File]::WriteAllText($temporary, $secret, [Text.UTF8Encoding]::new($false))
    if ($IsWindows) {
        $identity = [Security.Principal.WindowsIdentity]::GetCurrent().User
        $acl = [Security.AccessControl.FileSecurity]::new()
        $acl.SetOwner($identity)
        $acl.SetAccessRuleProtection($true, $false)
        $rule = [Security.AccessControl.FileSystemAccessRule]::new(
            $identity,
            [Security.AccessControl.FileSystemRights]::FullControl,
            [Security.AccessControl.AccessControlType]::Allow
        )
        $acl.AddAccessRule($rule)
        Set-Acl -LiteralPath $temporary -AclObject $acl
    }
    Move-Item -LiteralPath $temporary -Destination $Path -Force
    Write-Host "Recovery key initialized. Store an offline copy separately before relying on it for workstation-loss recovery."
}
finally {
    [Array]::Clear($bytes, 0, $bytes.Length)
    $secret = $null
    if (Test-Path -LiteralPath $temporary) { [IO.File]::Delete($temporary) }
}
