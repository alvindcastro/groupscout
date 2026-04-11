#!/bin/bash

# GroupScout Volume Backup Script
# This script backs up the named Docker volumes to a compressed tarball.

BACKUP_DIR="./backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
mkdir -p "$BACKUP_DIR"

VOLUMES=("groupscout_ollama_data" "groupscout_pgdata" "groupscout_n8n_data" "groupscout_prometheus_data" "groupscout_grafana_data")

echo "Starting backup of GroupScout volumes..."

for VOLUME in "${VOLUMES[@]}"; do
  if docker volume inspect "$VOLUME" > /dev/null 2>&1; then
    echo "Backing up $VOLUME..."
    FILENAME="$BACKUP_DIR/${VOLUME}_$TIMESTAMP.tar.gz"
    
    # Use a temporary container to access the volume and stream it to a tarball
    docker run --rm -v "$VOLUME:/data:ro" -v "$(pwd)/$BACKUP_DIR:/backup" alpine \
      tar -czf "/backup/$(basename "$FILENAME")" -C /data .
    
    if [ $? -eq 0 ]; then
      echo "✅ Successfully backed up $VOLUME to $FILENAME"
    else
      echo "❌ Failed to back up $VOLUME"
    fi
  else
    echo "⚠️ Volume $VOLUME not found, skipping."
  fi
done

echo "Backup process complete."
