#!/usr/bin/env bash
# Nightly backup: Postgres custom-format dump + attachments tarball.
# Run from host cron as root (docker access + /mnt/backup writes):
#   30 2 * * * /opt/rp-management/deploy/backup.sh >> /var/log/rp-backup.log 2>&1
#
# Retention: dailies kept 30 days; Sunday backups kept 84 days.
set -euo pipefail

BACKUP_ROOT="${BACKUP_ROOT:-/mnt/backup/rp}"
COMPOSE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STAMP="$(date +%F)"
DEST="$BACKUP_ROOT/$STAMP"

mkdir -p "$DEST"

# 1. Database — custom format so pg_restore can do selective restores.
docker compose -f "$COMPOSE_DIR/docker-compose.yml" exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom' \
  > "$DEST/db.pgcustom"

# Refuse to call it a backup if the dump is implausibly small.
if [ "$(stat -c%s "$DEST/db.pgcustom")" -lt 1024 ]; then
  echo "ERROR: dump smaller than 1 KiB — aborting" >&2
  exit 1
fi

# 2. Attachments — tar the named volume through a throwaway container.
docker run --rm \
  -v rp_attachments:/data:ro \
  -v "$DEST":/backup \
  alpine tar -czf /backup/attachments.tar.gz -C /data .

# 3. Retention. Weekday of the dir's date decides survival past 30 days.
find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -mtime +30 | while read -r dir; do
  weekday="$(date -d "$(basename "$dir")" +%u 2>/dev/null || echo 0)"
  [ "$weekday" = "7" ] || rm -rf "$dir"
done
find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -mtime +84 -exec rm -rf {} +

echo "backup ok: $DEST ($(du -sh "$DEST" | cut -f1))"
