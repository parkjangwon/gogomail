#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

CONTAINER="${GOGOMAIL_DEV_POSTGRES_CONTAINER:-gogomail-postgres-dev}"
DB_USER="${GOGOMAIL_DEV_POSTGRES_USER:-gogomail}"
DB_NAME="${GOGOMAIL_DEV_POSTGRES_DB:-gogomail}"
SEED_FILE="${GOGOMAIL_DEV_SEED_FILE:-scripts/seed_dev_data.sql}"

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required to seed the development database" >&2
	exit 1
fi

if [ ! -f "$SEED_FILE" ]; then
	echo "seed file not found: $SEED_FILE" >&2
	exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
	echo "PostgreSQL container is not running: $CONTAINER" >&2
	echo "Start it with: docker compose -f docker/docker-compose.dev.yml up -d postgres" >&2
	exit 1
fi

echo "==> Seeding gogomail beta development data"
echo "    container: $CONTAINER"
echo "    database:  $DB_NAME"
echo "    user:      $DB_USER"
echo "    file:      $SEED_FILE"

docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" < "$SEED_FILE"

cat <<'EOF'

Seed complete.

Useful webmail login:
  email:    pjw@parkjw.org
  password: pass1234

Additional seeded users use password:
  pass1234

EOF
