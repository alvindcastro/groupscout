#!/bin/sh
# Monitoring as code (IaC Phase 9): the Healthchecks.io checks defined in
# deploy/config/healthchecks/checks.json.
#
#   monitors.sh plan    print each check as apply would send it (no key needed)
#   monitors.sh apply   create each check, or update the one with the same name
#
# apply needs HEALTHCHECKS_API_KEY, a read-write API key of the Healthchecks.io
# project (Settings > API Access), from the environment or .env. The key is
# handed to curl in a mode-0600 header file, never on its command line. Alerts
# go to the project's integrations (email to the account owner by default).
# HEALTHCHECKS_API_URL overrides https://healthchecks.io/api/v3 for
# self-hosted instances and tests. Runbook: groupscout-site
# backend/docs/guides/MONITORING.md.
set -eu

cd "$(dirname "$0")/../.."

config=deploy/config/healthchecks/checks.json
api="${HEALTHCHECKS_API_URL:-https://healthchecks.io/api/v3}"

die() { echo "monitors: $*" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || die "jq is not installed"
[ -f "$config" ] || die "$config does not exist"

case "${1:-}" in
plan)
	jq -c '.checks[]' "$config"
	;;

apply)
	command -v curl >/dev/null 2>&1 || die "curl is not installed"
	key="${HEALTHCHECKS_API_KEY:-}"
	if [ -z "$key" ] && [ -f .env ]; then
		key=$(sed -n 's/^HEALTHCHECKS_API_KEY=//p' .env | tail -1)
	fi
	[ -n "$key" ] || die "HEALTHCHECKS_API_KEY is not set (environment or .env)"

	headers=$(mktemp)
	trap 'rm -f "$headers"' EXIT
	chmod 600 "$headers"
	printf 'X-Api-Key: %s\nContent-Type: application/json\n' "$key" >"$headers"

	jq -c '.checks[]' "$config" | while read -r check; do
		name=$(printf '%s' "$check" | jq -r .name)
		# unique=name returns the existing check of that name instead of
		# creating a second one.
		found=$(printf '%s' "$check" | jq -c '. + {unique: ["name"]}' |
			curl -fsS -X POST -H "@$headers" --data-binary @- "$api/checks/") ||
			die "$name: create request failed"
		update_url=$(printf '%s' "$found" | jq -r '.update_url // empty')
		[ -n "$update_url" ] || die "$name: no update_url in the response (is the API key read-only?)"
		# Send the full definition, which also updates a check that existed.
		updated=$(printf '%s' "$check" | curl -fsS -X POST -H "@$headers" --data-binary @- "$update_url") ||
			die "$name: update request failed"
		echo "monitors: $name: $(printf '%s' "$updated" | jq -r '"schedule \(.schedule) \(.tz), grace \(.grace)s, status \(.status)"')"
	done
	;;

*)
	echo "usage: $0 {plan|apply}" >&2
	exit 2
	;;
esac
