param(
    [Parameter(Mandatory = $true)][string]$BinaryPath,
    [Parameter(Mandatory = $true)][string]$Profile,
    [Parameter(Mandatory = $true)][string]$DaemonId,
    [Parameter(Mandatory = $true)][string]$LogDirectory
)

$ErrorActionPreference = 'Stop'
# Scheduled tasks can inherit an older PATH than the interactive shell.
# Resolve the installed Node and npm CLI shims for this child process only.
$env:PATH = @((Join-Path $env:ProgramFiles 'nodejs'), (Join-Path $env:APPDATA 'npm'), $env:PATH) -join ';'
New-Item -ItemType Directory -Path $LogDirectory -Force | Out-Null
$launchStamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ')
$launchBase = Join-Path $LogDirectory "daemon-launch-$launchStamp"
try {
    if (-not (Test-Path -LiteralPath $BinaryPath -PathType Leaf)) {
        throw 'The configured Multica executable is missing.'
    }
    $daemonProcess = Start-Process -FilePath $BinaryPath -ArgumentList @(
        '--profile', $Profile, 'daemon', 'start', '--foreground',
        '--daemon-id', $DaemonId, '--no-auto-update', '--no-auto-reload'
    ) -WorkingDirectory $env:USERPROFILE -WindowStyle Hidden -Wait -PassThru `
        -RedirectStandardOutput "$launchBase.stdout.log" -RedirectStandardError "$launchBase.stderr.log"
    $daemonProcess.Refresh()
    $daemonExit = $daemonProcess.ExitCode
    "ended_at=$([DateTime]::UtcNow.ToString('o')) exit_code=$daemonExit" |
        Set-Content -LiteralPath "$launchBase.result.log" -Encoding UTF8
    exit $daemonExit
} catch {
    "ended_at=$([DateTime]::UtcNow.ToString('o')) launcher_error=$($_.Exception.Message)" |
        Set-Content -LiteralPath "$launchBase.result.log" -Encoding UTF8
    exit 1
}
