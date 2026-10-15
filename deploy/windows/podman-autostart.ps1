<#
.SYNOPSIS
Starts the Podman machine and the GroupScout containers at Windows logon.

.DESCRIPTION
IaC Phase 10 (groupscout-site-6ah.11). register-autostart.ps1 installs a copy
of this script under %LOCALAPPDATA%\GroupScout and runs it from the scheduled
task "PodmanMachineAutostart", so the task never depends on a repo checkout or
branch.

The Podman machine does not start with Windows, and rootless containers with
restart=unless-stopped are not brought back inside it. That is how the July
2026 Docker-to-Podman move silently stopped the lead cadence. This script
starts the machine, waits until Podman answers, starts Postgres and waits for
it to report healthy, then starts the other containers. A container that does
not exist yet is logged and skipped. Each run appends to the log file.

Exit codes: 0 all present containers started, 1 Podman never became ready,
2 a container failed to start.

Runbook: groupscout-site backend/docs/guides/PODMAN_MIGRATION.md.
#>
[CmdletBinding()]
param(
    [string]$PodmanPath = (Join-Path $env:ProgramFiles 'RedHat\Podman\podman.exe'),
    # Started first; the rest start once these are healthy.
    [string[]]$FirstContainers = @('groupscout_postgres'),
    [string[]]$Containers = @(
        'groupscout_ollama', 'groupscout_app', 'groupscout_n8n',
        'groupscout_prometheus', 'groupscout_loki', 'groupscout_alloy', 'groupscout_grafana'
    ),
    [int]$ReadyTimeoutSeconds = 180,
    [int]$HealthyTimeoutSeconds = 120,
    [string]$LogPath = (Join-Path $env:LOCALAPPDATA 'GroupScout\podman-autostart.log')
)

$ErrorActionPreference = 'Continue'

$logDir = Split-Path -Parent $LogPath
if (-not (Test-Path $logDir)) { New-Item -ItemType Directory -Force -Path $logDir | Out-Null }
if ((Test-Path $LogPath) -and (Get-Item $LogPath).Length -gt 1MB) {
    Move-Item -Force $LogPath "$LogPath.1"
}

function Write-Log([string]$Message) {
    $line = '{0} {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $Message
    Add-Content -Path $LogPath -Value $line
    Write-Verbose $line
}

# Runs podman and returns @{ Code; Output }. stderr is folded into Output.
function Invoke-Podman {
    $output = & $PodmanPath @args 2>&1 | ForEach-Object { "$_" }
    @{ Code = $LASTEXITCODE; Output = (($output -join ' ') -replace '\s+', ' ').Trim() }
}

function Test-ContainerExists([string]$Name) {
    (Invoke-Podman container exists $Name).Code -eq 0
}

function Wait-Healthy([string]$Name) {
    $deadline = (Get-Date).AddSeconds($HealthyTimeoutSeconds)
    do {
        $status = (Invoke-Podman inspect --format '{{.State.Health.Status}}' $Name).Output
        # A container without a healthcheck reports an empty status: running is enough.
        if ($status -eq 'healthy' -or $status -eq '') { return $true }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    return $false
}

Write-Log '--- autostart begin'
if (-not (Test-Path $PodmanPath)) {
    Write-Log "podman not found at $PodmanPath"
    exit 1
}

# 1) Start the machine; "already running" is fine.
$start = Invoke-Podman machine start
Write-Log ("machine start: exit {0} {1}" -f $start.Code, $start.Output)

# 2) Wait until the client can talk to the machine.
$deadline = (Get-Date).AddSeconds($ReadyTimeoutSeconds)
$ready = $false
do {
    if ((Invoke-Podman info --format '{{.Host.Arch}}').Code -eq 0) { $ready = $true; break }
    Start-Sleep -Seconds 2
} while ((Get-Date) -lt $deadline)
if (-not $ready) {
    Write-Log "podman did not answer within $ReadyTimeoutSeconds s"
    exit 1
}
Write-Log 'podman ready'

# 3) Start the containers, dependencies first.
$failed = 0
foreach ($name in @($FirstContainers) + @($Containers)) {
    if (-not (Test-ContainerExists $name)) {
        Write-Log "skip $name (no such container; run 'podman compose up -d' once to create it)"
        continue
    }
    $result = Invoke-Podman start $name
    if ($result.Code -ne 0) {
        $failed++
        Write-Log ("start {0} failed: {1}" -f $name, $result.Output)
        continue
    }
    Write-Log "started $name"
    if ($FirstContainers -contains $name) {
        if (Wait-Healthy $name) { Write-Log "$name healthy" }
        else { Write-Log "$name not healthy after $HealthyTimeoutSeconds s; starting the rest anyway" }
    }
}

Write-Log ("--- autostart end ({0} failed)" -f $failed)
if ($failed -gt 0) { exit 2 }
exit 0
