#!/bin/sh
# Imports the tracked n8n workflows, then hands over to the image entrypoint.
#
# Runs inside the n8n container (as the node user) right before n8n starts, so
# an import can never race a running n8n for the SQLite database. A workflow is
# (re)imported only when it is missing from the database or its JSON changed
# since the last import; the sha256 of each imported file is kept in the n8n
# volume. Edits made in the n8n UI therefore survive restarts until the tracked
# file changes, and an empty volume (the 2026-07 Podman incident) gets every
# workflow back, active, with no manual steps.
#
# Any extra arguments are passed to n8n, e.g. `list:workflow --active=true`
# runs the import and then the listing instead of starting the server.
set -eu

WORKFLOW_DIR="${GROUPSCOUT_N8N_WORKFLOW_DIR:-/opt/groupscout/n8n/workflows}"
STATE_FILE="${GROUPSCOUT_N8N_STATE_FILE:-/home/node/.n8n/groupscout-workflows.sha256}"

log() { echo "[groupscout-n8n-bootstrap] $*"; }

workflow_field() {
	node -e 'const w = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")); process.stdout.write(String(w[process.argv[2]] ?? ""))' "$1" "$2"
}

if [ -d "$WORKFLOW_DIR" ]; then
	touch "$STATE_FILE"
	existing_ids=$(n8n list:workflow --onlyId 2>/dev/null || true)

	for file in "$WORKFLOW_DIR"/*.json; do
		[ -e "$file" ] || continue
		name=$(basename "$file")
		id=$(workflow_field "$file" id)
		if [ -z "$id" ]; then
			log "ERROR: $name has no top-level \"id\"; refusing to import an unstable workflow"
			exit 1
		fi

		sum=$(sha256sum "$file" | cut -d' ' -f1)
		recorded=$(grep " $name\$" "$STATE_FILE" | cut -d' ' -f1 || true)
		exists=false
		if printf '%s\n' "$existing_ids" | grep -qx "$id"; then
			exists=true
		fi
		if [ "$exists" = true ] && [ "$sum" = "$recorded" ]; then
			log "unchanged: $name ($id)"
			continue
		fi

		# Re-importing an active workflow makes the import deactivate it, which
		# the CLI user is not permitted to do (n8n 2.43), so unpublish it first.
		if [ "$exists" = true ]; then
			log "unpublishing before re-import: $id"
			n8n unpublish:workflow --id="$id"
		fi

		log "importing: $name ($id)"
		# Import always deactivates; --activeState=fromJson needs queue/multi-main
		# mode, so publish explicitly when the tracked JSON says "active": true.
		n8n import:workflow --input="$file"
		if [ "$(workflow_field "$file" active)" = "true" ]; then
			log "publishing: $id"
			n8n publish:workflow --id="$id"
		fi

		{ grep -v " $name\$" "$STATE_FILE" || true; echo "$sum $name"; } > "$STATE_FILE.tmp"
		mv "$STATE_FILE.tmp" "$STATE_FILE"
	done
else
	log "no workflow directory at $WORKFLOW_DIR; skipping import"
fi

exec /docker-entrypoint.sh "$@"
