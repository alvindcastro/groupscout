<#
.SYNOPSIS
Creates or updates the scheduled task that starts Podman and GroupScout at logon.

.DESCRIPTION
IaC Phase 10 (groupscout-site-6ah.11). Copies podman-autostart.ps1 to
-InstallDir (default %LOCALAPPDATA%\GroupScout) and registers the task
"PodmanMachineAutostart" to run that copy at logon of the current user. Safe
to run again: an existing task is replaced with the same definition, which is
how script updates are rolled out. Needs no administrator rights.

  powershell -ExecutionPolicy Bypass -File deploy\windows\register-autostart.ps1 [-RunNow]
  powershell -ExecutionPolicy Bypass -File deploy\windows\register-autostart.ps1 -Unregister

-WhatIf shows what would change. Runbook: groupscout-site
backend/docs/guides/PODMAN_MIGRATION.md.
#>
[CmdletBinding(SupportsShouldProcess)]
param(
    [string]$TaskName = 'PodmanMachineAutostart',
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'GroupScout'),
    # Run the task once after registering and report its result.
    [switch]$RunNow,
    # Remove the task (the installed script copy is left in place).
    [switch]$Unregister
)

$ErrorActionPreference = 'Stop'

$existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue

if ($Unregister) {
    if ($existing -and $PSCmdlet.ShouldProcess($TaskName, 'Unregister scheduled task')) {
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
        Write-Output "Removed scheduled task $TaskName."
    } elseif (-not $existing) {
        Write-Output "Scheduled task $TaskName does not exist."
    }
    return
}

$source = Join-Path $PSScriptRoot 'podman-autostart.ps1'
$target = Join-Path $InstallDir 'podman-autostart.ps1'
if ($PSCmdlet.ShouldProcess($target, "Install $source")) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Force -Path $source -Destination $target
}

$user = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
$action = New-ScheduledTaskAction -Execute 'powershell.exe' `
    -Argument ('-NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "{0}"' -f $target)
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $user
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Minutes 10) -StartWhenAvailable `
    -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -MultipleInstances IgnoreNew
$principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited
$verb = if ($existing) { 'Update' } else { 'Create' }

if ($PSCmdlet.ShouldProcess($TaskName, "$verb scheduled task (at logon of $user)")) {
    Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings `
        -Principal $principal -Force `
        -Description 'Starts the Podman machine and the GroupScout containers at logon (groupscout deploy/windows, IaC Phase 10).' |
        Out-Null
    Write-Output ("{0}d scheduled task {1}: runs {2} at logon of {3}." -f $verb, $TaskName, $target, $user)
}

if ($RunNow -and $PSCmdlet.ShouldProcess($TaskName, 'Run now')) {
    Start-ScheduledTask -TaskName $TaskName
    $deadline = (Get-Date).AddMinutes(10)
    do {
        Start-Sleep -Seconds 2
        $state = (Get-ScheduledTask -TaskName $TaskName).State
    } while ($state -eq 'Running' -and (Get-Date) -lt $deadline)
    $info = Get-ScheduledTaskInfo -TaskName $TaskName
    Write-Output ("Ran {0}: result {1} (0 ok, 1 Podman not ready, 2 a container failed). Log: {2}" -f
        $TaskName, $info.LastTaskResult, (Join-Path $env:LOCALAPPDATA 'GroupScout\podman-autostart.log'))
    if ($info.LastTaskResult -ne 0) { exit 1 }
}
