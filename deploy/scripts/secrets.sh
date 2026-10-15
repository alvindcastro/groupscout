#!/bin/sh
# Secrets as code (IaC Phase 3): SOPS + age helpers behind the make targets.
#
#   secrets.sh edit   [env]  open deploy/secrets/<env>.sops.env in $EDITOR
#   secrets.sh render [env]  decrypt it to .env (mode 0600). Refuses to replace
#                            a .env that differs unless FORCE=1.
#   secrets.sh diff   [env]  name the keys that differ between .env and the
#                            encrypted file (never prints values)
#   secrets.sh updatekeys    re-encrypt every deploy/secrets file to the
#                            recipients currently listed in .sops.yaml
#   secrets.sh scan          gitleaks over the git history, plus a check that
#                            no plaintext env or key file is tracked
#
# env defaults to $SECRETS_ENV, else "prod". SECRETS_OUT overrides the .env
# path. sops reads the age private key from $SOPS_AGE_KEY_FILE or its default
# (~/.config/sops/age/keys.txt on Linux, %AppData%\sops\age\keys.txt on
# Windows). Runbook: groupscout-site backend/docs/guides/SECRETS.md.
set -eu

cd "$(dirname "$0")/../.."

cmd="${1:-}"
env_name="${2:-${SECRETS_ENV:-prod}}"
secrets_file="deploy/secrets/$env_name.sops.env"
out="${SECRETS_OUT:-.env}"

die() { echo "secrets: $*" >&2; exit 1; }

need() {
	command -v "$1" >/dev/null 2>&1 || die "$1 is not installed (see SECRETS.md)"
}

need_file() {
	[ -f "$secrets_file" ] || die "$secrets_file does not exist"
}

# keydiff ENCRYPTED_PLAINTEXT LOCAL: one line per key that is missing on
# either side or has a different value. Comments and blank lines are ignored.
keydiff() {
	awk -v a="$secrets_file" -v b="$out" '
		/^[[:space:]]*(#|$)/ { next }
		{ k = $0; sub(/=.*/, "", k); v = substr($0, index($0, "=") + 1) }
		NR == FNR { enc[k] = v; next }
		{ loc[k] = v }
		END {
			for (k in enc) {
				if (!(k in loc)) print "  only in " a ": " k
				else if (enc[k] != loc[k]) print "  differs: " k
			}
			for (k in loc) if (!(k in enc)) print "  only in " b ": " k
		}
	' "$1" "$2" | sort
}

decrypt_to() {
	(umask 077 && sops decrypt --input-type dotenv --output-type dotenv "$secrets_file" >"$1")
}

case "$cmd" in
edit)
	need sops
	exec sops edit --input-type dotenv --output-type dotenv "$secrets_file"
	;;

render)
	need sops
	need_file
	tmp="$out.tmp.$$"
	trap 'rm -f "$tmp"' EXIT
	decrypt_to "$tmp"
	if [ -f "$out" ] && cmp -s "$tmp" "$out"; then
		echo "secrets: $out already matches $secrets_file"
		exit 0
	fi
	if [ -f "$out" ] && [ "${FORCE:-0}" != 1 ]; then
		echo "secrets: $out differs from $secrets_file:" >&2
		keydiff "$tmp" "$out" >&2
		die "keep local changes with 'make secrets-edit', or overwrite $out with FORCE=1"
	fi
	chmod 600 "$tmp"
	mv "$tmp" "$out"
	echo "secrets: wrote $out from $secrets_file"
	;;

diff)
	need sops
	need_file
	[ -f "$out" ] || die "$out does not exist; run 'make secrets-render'"
	tmp="$out.tmp.$$"
	trap 'rm -f "$tmp"' EXIT
	decrypt_to "$tmp"
	changes=$(keydiff "$tmp" "$out")
	if [ -z "$changes" ]; then
		echo "secrets: $out has the same keys and values as $secrets_file"
		exit 0
	fi
	echo "secrets: $out vs $secrets_file:"
	echo "$changes"
	exit 1
	;;

updatekeys)
	need sops
	for f in deploy/secrets/*.sops.env; do
		[ -e "$f" ] || continue
		sops updatekeys --yes --input-type dotenv "$f"
	done
	;;

scan)
	need gitleaks
	# --redact: this repository and its Actions logs are public.
	gitleaks git --redact --no-banner --exit-code 1 .

	status=0
	# Plaintext env files and private keys must never be tracked. Encrypted
	# *.sops.env files and the .env.example template are the exceptions.
	tracked=$(git ls-files | grep -E '(^|/)([^/]*\.env|\.env\.[^/]*)$|\.(pem|key|p12|pfx|jks|tfstate|tfstate\.backup)$|(^|/)(id_rsa|id_ecdsa|id_ed25519|keys\.txt)$' |
		grep -vE '\.sops\.env$|(^|/)\.env\.example$' || true)
	if [ -n "$tracked" ]; then
		echo "secrets: plaintext secret files are tracked:" >&2
		echo "$tracked" | sed 's/^/  /' >&2
		status=1
	fi
	# A *.sops.env file must actually be SOPS-encrypted.
	for f in $(git ls-files '*.sops.env'); do
		if ! grep -q '^sops_mac=ENC\[' "$f" || grep -vE '^(sops_[a-z_0-9]+=|#ENC\[|[A-Za-z_][A-Za-z0-9_]*=(ENC\[.*)?$|$)' "$f" >/dev/null; then
			echo "secrets: $f is not fully SOPS-encrypted" >&2
			status=1
		fi
	done
	[ "$status" = 0 ] && echo "secrets: scan passed"
	exit "$status"
	;;

*)
	echo "usage: $0 {edit|render|diff} [env] | updatekeys | scan" >&2
	exit 2
	;;
esac
