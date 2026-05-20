#!/usr/bin/env bash
# backup.sh — PostgreSQL backup for gogomail
#
# Required environment variables:
#   GOGOMAIL_DATABASE_URL     — PostgreSQL connection URL
#
# Optional environment variables:
#   GOGOMAIL_BACKUP_DIR           — Directory for backup files (default: ./backups)
#   GOGOMAIL_BACKUP_RETENTION_DAYS — Days of backups to retain (default: 7)
#   GOGOMAIL_BACKUP_S3_BUCKET     — S3 bucket name; if set, upload the backup with aws s3 cp
#   GOGOMAIL_BACKUP_S3_PREFIX     — S3 key prefix (default: backups/)
set -euo pipefail

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 127
  fi
}

require_cmd pg_dump
require_cmd gzip

DATABASE_URL="${GOGOMAIL_DATABASE_URL:-}"
if [[ -z "$DATABASE_URL" ]]; then
  echo "GOGOMAIL_DATABASE_URL is required" >&2
  exit 2
fi

BACKUP_DIR="${GOGOMAIL_BACKUP_DIR:-./backups}"
RETENTION_DAYS="${GOGOMAIL_BACKUP_RETENTION_DAYS:-7}"
S3_BUCKET="${GOGOMAIL_BACKUP_S3_BUCKET:-}"
S3_PREFIX="${GOGOMAIL_BACKUP_S3_PREFIX:-backups/}"

mkdir -p "$BACKUP_DIR"

TIMESTAMP="$(date -u '+%Y-%m-%d-%H')"
FILENAME="backup-${TIMESTAMP}.sql.gz"
FILEPATH="${BACKUP_DIR}/${FILENAME}"

echo "starting backup: ${FILEPATH}"
pg_dump --format=plain --no-owner --no-privileges "$DATABASE_URL" | gzip > "$FILEPATH"
echo "backup written: ${FILEPATH} ($(du -sh "$FILEPATH" | cut -f1))"

# Remove backups older than RETENTION_DAYS
if [[ "$RETENTION_DAYS" -gt 0 ]]; then
  echo "pruning backups older than ${RETENTION_DAYS} days"
  find "$BACKUP_DIR" -maxdepth 1 -name 'backup-*.sql.gz' -mtime "+${RETENTION_DAYS}" -delete
fi

# Optional S3 upload
if [[ -n "$S3_BUCKET" ]]; then
  require_cmd aws
  S3_KEY="${S3_PREFIX}${FILENAME}"
  echo "uploading to s3://${S3_BUCKET}/${S3_KEY}"
  aws s3 cp "$FILEPATH" "s3://${S3_BUCKET}/${S3_KEY}"
  echo "s3 upload complete"
fi

echo "backup done"
