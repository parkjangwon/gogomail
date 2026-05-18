#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  GOGOMAIL_DATABASE_URL=postgres://... scripts/backup-restore-rehearsal.sh

Optional:
  GOGOMAIL_RESTORE_REHEARSAL_DB_URL=postgres://...   Target scratch DB URL.
  GOGOMAIL_RESTORE_REHEARSAL_KEEP_DB=1              Keep scratch DB after checks.

The script dumps the configured database, restores it into a scratch database,
checks migration metadata, and drops the scratch database unless KEEP_DB=1.
USAGE
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 127
  fi
}

database_name() {
  local url="$1"
  local path="${url%%\?*}"
  path="${path%/}"
  echo "${path##*/}"
}

database_base_url() {
  local url="$1"
  local query=""
  if [[ "$url" == *\?* ]]; then
    query="?${url#*\?}"
  fi
  local path="${url%%\?*}"
  path="${path%/}"
  echo "${path%/*}/postgres${query}"
}

quote_ident() {
  printf '"%s"' "$(printf '%s' "$1" | sed 's/"/""/g')"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd pg_dump
require_cmd psql

source_url="${GOGOMAIL_DATABASE_URL:-}"
if [[ -z "$source_url" ]]; then
  echo "GOGOMAIL_DATABASE_URL is required" >&2
  usage >&2
  exit 2
fi

source_db="$(database_name "$source_url")"
default_target="${source_url%/*}/gogomail_restore_rehearsal_${source_db}_$$"
target_url="${GOGOMAIL_RESTORE_REHEARSAL_DB_URL:-$default_target}"

if [[ "$target_url" == "$source_url" ]]; then
  echo "refusing to restore into GOGOMAIL_DATABASE_URL" >&2
  exit 2
fi

target_db="$(database_name "$target_url")"
target_base_url="$(database_base_url "$target_url")"
dump_file="$(mktemp "${TMPDIR:-/tmp}/gogomail-backup-restore.XXXXXX.sql")"

cleanup() {
  rm -f "$dump_file"
  if [[ "${GOGOMAIL_RESTORE_REHEARSAL_KEEP_DB:-0}" != "1" ]]; then
    psql "$target_base_url" --quiet --command "DROP DATABASE IF EXISTS $(quote_ident "$target_db");" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "dumping source database: $source_db"
pg_dump --format=plain --no-owner --no-privileges --file "$dump_file" "$source_url"

echo "creating scratch database: $target_db"
psql "$target_base_url" --quiet --command "DROP DATABASE IF EXISTS $(quote_ident "$target_db");" >/dev/null
psql "$target_base_url" --quiet --command "CREATE DATABASE $(quote_ident "$target_db");" >/dev/null

echo "restoring dump into scratch database"
sed '/^SET transaction_timeout =/d' "$dump_file" | psql "$target_url" --quiet --set ON_ERROR_STOP=1 >/dev/null

echo "checking restored migration metadata"
psql "$target_url" --tuples-only --no-align --command \
  "select max(version_id) from goose_db_version where is_applied = true;" |
  awk 'NF { print "latest_applied_migration=" $0 }'

echo "backup restore rehearsal completed"
