<#
.SYNOPSIS
Creates the WSL 2 distro that hosts GroupScout production and readies it for Ansible.

.DESCRIPTION
IaC Phase 5 (groupscout-site-6ah.6), decision D1: production runs in a
dedicated WSL 2 distro on the home PC, kept apart from the dev distro
because the Compose container names are fixed. This is the WSL stand-in for
cloud-init: it installs the distro, turns on systemd, creates the deploy user
with passwordless sudo, and starts sshd on 127.0.0.1 only, accepting one key.
Everything else is Ansible (deploy/ansible/site.yml).

WSL 2 distros share one network namespace, so the controller (any distro
holding the private key) reaches the host at 127.0.0.1:$SshPort.

Safe to run again: an existing distro is only re-checked and the key file
re-applied.

  powershell -ExecutionPolicy Bypass -File deploy\windows\new-wsl-host.ps1 `
      -PublicKeyFile \\wsl.localhost\Ubuntu\home\<you>\.ssh\groupscout_prod_ed25519.pub

Runbook: groupscout-site backend/docs/guides/WSL_HOST.md.
#>
[CmdletBinding(SupportsShouldProcess)]
param(
    [Parameter(Mandatory)]
    [string]$PublicKeyFile,
    [string]$Name = 'groupscout-prod',
    [string]$Image = 'Ubuntu-26.04',
    [string]$DeployUser = 'deploy',
    # Not 1000: WSL distros share one cgroup tree, and the Podman machine's
    # user (and usually the dev distro's) is 1000, so a second user@1000.service
    # cannot start ("Failed to spawn executor: Device or resource busy").
    [int]$DeployUid = 2000,
    [int]$SshPort = 2222
)

$ErrorActionPreference = 'Continue'
# wsl.exe writes UTF-16 unless told otherwise.
$env:WSL_UTF8 = '1'

$key = (Get-Content -Raw -ErrorAction Stop $PublicKeyFile).Trim()
if ($key -notmatch '^(ssh-ed25519|ssh-rsa|ecdsa-sha2-\S+) [A-Za-z0-9+/=]+( .*)?$') {
    throw "$PublicKeyFile does not look like an SSH public key"
}

$distros = @(wsl.exe --list --quiet | ForEach-Object { $_.Trim() } | Where-Object { $_ })
if ($distros -notcontains $Name) {
    if ($PSCmdlet.ShouldProcess($Name, "wsl --install $Image")) {
        wsl.exe --install $Image --name $Name --no-launch --web-download
        if ($LASTEXITCODE -ne 0) { throw "wsl --install failed ($LASTEXITCODE)" }
    }
} else {
    Write-Output "distro $Name exists"
}

# Runs as root inside the distro. The public key is not a secret.
#
# WSL interop: every WSL 2 distro shares one binfmt_misc table, and
# systemd >= 256 (Ubuntu 26.04 ships 259) flushes it at shutdown whenever
# binfmt_misc is mounted. That deletes the WSLInterop entry, so cmd.exe and
# wsl.exe stop working in every other distro until it is re-registered. This
# distro needs no interop, so the bootstrap turns interop off, masks the
# binfmt units, installs a unit that unmounts binfmt_misc at every boot, and
# unmounts it before its own first restart.
$bootstrap = @'
set -eu
export DEBIAN_FRONTEND=noninteractive
user="__USER__"; uid="__UID__"; port="__PORT__"; key="__KEY__"

printf '%s\n' '[boot]' 'systemd=true' '' '[user]' "default=$user" '' \
	'# No Windows interop here; see wsl-binfmt-detach.service.' \
	'[interop]' 'enabled=false' 'appendWindowsPath=false' > /etc/wsl.conf

for unit in systemd-binfmt.service proc-sys-fs-binfmt_misc.automount proc-sys-fs-binfmt_misc.mount; do
	ln -sf /dev/null "/etc/systemd/system/$unit"
done
printf '%s\n' \
	'# Every WSL 2 distro shares one binfmt_misc table; systemd-shutdown flushes' \
	'# it when it is mounted, deleting WSLInterop for all distros. Unmount it here.' \
	'[Unit]' \
	'Description=Detach the shared binfmt_misc so shutdown cannot flush WSL interop' \
	'DefaultDependencies=no' 'After=local-fs.target' 'Before=sysinit.target' \
	'ConditionPathIsMountPoint=/proc/sys/fs/binfmt_misc' \
	'[Service]' 'Type=oneshot' 'RemainAfterExit=yes' 'ExecStart=/bin/umount -l /proc/sys/fs/binfmt_misc' \
	'[Install]' 'WantedBy=sysinit.target' > /etc/systemd/system/wsl-binfmt-detach.service
mkdir -p /etc/systemd/system/sysinit.target.wants
ln -sf /etc/systemd/system/wsl-binfmt-detach.service /etc/systemd/system/sysinit.target.wants/wsl-binfmt-detach.service

# WSL distros share one cgroup tree: the deploy user needs a uid no other
# distro's systemd user manager uses, and its own subordinate id range.
if ! id "$user" >/dev/null 2>&1; then
	useradd --create-home --shell /bin/bash --uid "$uid" --user-group "$user"
elif [ "$(id -u "$user")" != "$uid" ]; then
	loginctl terminate-user "$user" 2>/dev/null || true
	pkill -KILL -u "$user" 2>/dev/null || true
	sleep 1
	usermod -u "$uid" "$user"
	groupmod -g "$uid" "$user"
	chown -R "$user:$user" "/home/$user"
fi
sed -i "/^$user:/d" /etc/subuid /etc/subgid
echo "$user:300000:65536" >> /etc/subuid
echo "$user:300000:65536" >> /etc/subgid
echo "$user ALL=(ALL) NOPASSWD:ALL" > "/etc/sudoers.d/90-$user"
chmod 440 "/etc/sudoers.d/90-$user"
install -d -m 700 -o "$user" -g "$user" "/home/$user/.ssh"
printf '%s\n' "$key" > "/home/$user/.ssh/authorized_keys"
chown "$user:$user" "/home/$user/.ssh/authorized_keys"
chmod 600 "/home/$user/.ssh/authorized_keys"

if ! command -v sshd >/dev/null 2>&1 || ! command -v python3 >/dev/null 2>&1; then
	apt-get update -q
	apt-get install -yq openssh-server python3
fi
mkdir -p /etc/ssh/sshd_config.d
printf '%s\n' "# Bootstrap only; the Ansible hardening role owns sshd afterwards." \
	"Port $port" 'ListenAddress 127.0.0.1' 'PasswordAuthentication no' \
	'KbdInteractiveAuthentication no' 'PermitRootLogin no' > /etc/ssh/sshd_config.d/10-groupscout-bootstrap.conf
sshd -t
ln -sf /usr/lib/systemd/system/ssh.service /etc/systemd/system/multi-user.target.wants/ssh.service

# This boot may have binfmt_misc mounted; detach it before the restart below.
umount -l /proc/sys/fs/binfmt_misc 2>/dev/null || true
# end of bootstrap
'@
$bootstrap = $bootstrap.Replace('__USER__', $DeployUser).Replace('__UID__', "$DeployUid").Replace('__PORT__', "$SshPort").Replace('__KEY__', $key)

if ($PSCmdlet.ShouldProcess($Name, 'Bootstrap: systemd, deploy user, sshd on 127.0.0.1')) {
    # Feed the script on stdin with LF line endings.
    ($bootstrap -replace "`r`n", "`n") | wsl.exe -d $Name -u root -- bash -s
    if ($LASTEXITCODE -ne 0) { throw "bootstrap failed ($LASTEXITCODE)" }

    # Restart the distro so systemd runs as PID 1 with the new configuration.
    wsl.exe --terminate $Name | Out-Null
    wsl.exe -d $Name -u root --exec systemctl is-system-running --wait | Out-Null
    $listen = wsl.exe -d $Name -u root --exec ss -ltnH "sport = :$SshPort"
    $binfmt = wsl.exe -d $Name -u root --exec findmnt -n /proc/sys/fs/binfmt_misc
    if (-not $listen) { throw "sshd is not listening on 127.0.0.1:$SshPort" }
    if ($binfmt) { throw "binfmt_misc is still mounted in $Name; stopping it would break WSL interop" }
    Write-Output "$Name ready: ssh -i <key> -p $SshPort $DeployUser@127.0.0.1"
    Write-Output "WSL stops an idle distro after ~15 s; keep a session open (or the autostart keep-alive) while Ansible runs."
}
