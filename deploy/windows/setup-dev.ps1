<#
.SYNOPSIS
Takes a Windows workstation with the tools installed to a running GroupScout stack.

.DESCRIPTION
IaC Phase 10 (groupscout-site-6ah.11). The second of two commands:

  winget configure -f deploy\windows\configuration.dsc.yaml --accept-configuration-agreements
  powershell -ExecutionPolicy Bypass -File deploy\windows\setup-dev.ps1

Each step checks first and changes only what is missing, so it is safe to run
again:
  1. Create the Podman machine if there is none, and start it.
  2. Check that 'podman compose' finds a Compose provider.
  3. Create .env from .env.example if there is no .env. Real values come from
     the SOPS-encrypted deploy\secrets\prod.sops.env ('make secrets-render'
     in WSL; see SECRETS.md). This script never writes secrets.
  4. Register the logon autostart task (register-autostart.ps1).
  5. Build and start the stack with 'podman compose up -d --build' and wait
     for /health.

-WhatIf shows the plan. Runbook: groupscout-site
backend/docs/guides/PODMAN_MIGRATION.md.
#>
[CmdletBinding(SupportsShouldProcess)]
param(
    # Compose profiles to start: llm = Ollama, obs = Prometheus/Grafana/Loki/Alloy.
    [string]$Profiles = 'llm,obs',
    # Podman machine size for a new machine; 0 keeps Podman's default.
    [int]$Cpus = 0,
    [int]$MemoryMB = 0,
    [switch]$SkipAutostart,
    # Prepare everything but do not build or start containers.
    [switch]$SkipStack,
    [int]$HealthTimeoutSeconds = 300
)

# Native commands are checked through $LASTEXITCODE. 'Stop' would turn any
# stderr line from podman (it logs progress there) into a terminating error
# under Windows PowerShell 5.1.
$ErrorActionPreference = 'Continue'
$repo = (Resolve-Path -ErrorAction Stop (Join-Path $PSScriptRoot '..\..')).Path

function Step([string]$Message) { Write-Output "== $Message" }

# winget installs do not update the PATH of an already-open session.
$env:Path = @(
    [Environment]::GetEnvironmentVariable('Path', 'Machine'),
    [Environment]::GetEnvironmentVariable('Path', 'User'),
    $env:Path
) -join ';'

$podman = (Get-Command podman.exe -ErrorAction SilentlyContinue).Source
if (-not $podman) {
    $podman = Join-Path $env:ProgramFiles 'RedHat\Podman\podman.exe'
}
if (-not (Test-Path $podman)) {
    throw "Podman is not installed. Run first: winget configure -f deploy\windows\configuration.dsc.yaml --accept-configuration-agreements"
}

Step 'Podman machine'
$machines = @(& $podman machine list --format '{{.Name}}' | Where-Object { $_ })
if ($machines.Count -eq 0) {
    $initArgs = @('machine', 'init')
    if ($Cpus -gt 0) { $initArgs += @('--cpus', "$Cpus") }
    if ($MemoryMB -gt 0) { $initArgs += @('--memory', "$MemoryMB") }
    if ($PSCmdlet.ShouldProcess('Podman', ($initArgs -join ' '))) {
        & $podman @initArgs
        if ($LASTEXITCODE -ne 0) { throw "podman machine init failed ($LASTEXITCODE)" }
    }
} else {
    Write-Output ("machine exists: {0}" -f ($machines -join ', '))
}
$state = (& $podman machine inspect --format '{{.State}}' 2>$null | Select-Object -First 1)
if ($state -ne 'running') {
    if ($PSCmdlet.ShouldProcess('Podman machine', 'start')) {
        & $podman machine start
        if ($LASTEXITCODE -ne 0) { throw "podman machine start failed ($LASTEXITCODE)" }
    }
} else {
    Write-Output 'machine running'
}

Step 'Compose provider'
$composeOutput = & $podman compose version 2>&1 | ForEach-Object { "$_" }
if ($LASTEXITCODE -ne 0) {
    throw "'podman compose' has no provider. Install Docker.DockerCompose (configuration.dsc.yaml) and open a new terminal."
}
# Keep the version line; drop podman's provider banner and color codes.
$composeOutput | Where-Object { $_ -match 'version' -and $_ -notmatch '^>>>>' } |
    ForEach-Object { $_ -replace '\x1b\[[0-9;]*m', '' } | Select-Object -First 1

Step '.env'
$envFile = Join-Path $repo '.env'
if (Test-Path $envFile) {
    Write-Output '.env exists; left unchanged'
} elseif ($PSCmdlet.ShouldProcess($envFile, 'Create from .env.example')) {
    Copy-Item -ErrorAction Stop (Join-Path $repo '.env.example') $envFile
    Write-Output '.env created from .env.example with placeholder values.'
    Write-Output 'For the real values: make secrets-render in WSL (backend/docs/guides/SECRETS.md).'
}

if (-not $SkipAutostart) {
    Step 'Logon autostart task'
    & (Join-Path $PSScriptRoot 'register-autostart.ps1') -WhatIf:$WhatIfPreference
}

if ($SkipStack) {
    Write-Output 'Skipping the stack (-SkipStack).'
    return
}

Step "Stack (profiles: $Profiles)"
if ($PSCmdlet.ShouldProcess($repo, "podman compose up -d --build (COMPOSE_PROFILES=$Profiles)")) {
    $env:COMPOSE_PROFILES = $Profiles
    $git = Get-Command git.exe -ErrorAction SilentlyContinue
    if ($git) { $env:GROUPSCOUT_VERSION = (& git -C $repo rev-parse --short HEAD) }
    Push-Location $repo
    try {
        & $podman compose up -d --build
        if ($LASTEXITCODE -ne 0) { throw "podman compose up failed ($LASTEXITCODE)" }
    } finally {
        Pop-Location
    }

    $deadline = (Get-Date).AddSeconds($HealthTimeoutSeconds)
    $health = $null
    do {
        try { $health = Invoke-RestMethod -TimeoutSec 5 -Uri 'http://localhost:8080/health' } catch { Start-Sleep -Seconds 3 }
    } while (-not $health -and (Get-Date) -lt $deadline)
    if (-not $health) { throw "http://localhost:8080/health did not answer within $HealthTimeoutSeconds s" }
    Write-Output ("API healthy: database {0}, version {1}" -f $health.database, $health.version)
    Write-Output 'API http://localhost:8080   n8n http://localhost:5678   Grafana http://localhost:3000'
}
