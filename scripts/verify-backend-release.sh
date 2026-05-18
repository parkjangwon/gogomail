#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

echo "==> go test ./..."
if [ "${GOGOMAIL_TEST_OPENSEARCH_URL:-}" != "" ]; then
	echo "==> OpenSearch integration tests enabled by GOGOMAIL_TEST_OPENSEARCH_URL"
else
	echo "==> OpenSearch integration tests skipped unless GOGOMAIL_TEST_OPENSEARCH_URL is set"
fi
go test ./...

echo "==> go mod tidy -diff"
go mod tidy -diff

if [ "${GOGOMAIL_TEST_DATABASE_URL:-}" != "" ]; then
	echo "==> PostgreSQL integration tests"
	GOGOMAIL_TEST_DATABASE_URL="${GOGOMAIL_TEST_DATABASE_URL}" go test ./internal/maildb ./internal/outbox
else
	echo "==> skipping PostgreSQL integration tests: GOGOMAIL_TEST_DATABASE_URL is not set"
fi

if [ "${GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL:-}" != "" ]; then
	echo "==> backup restore rehearsal"
	GOGOMAIL_DATABASE_URL="${GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL}" ./scripts/backup-restore-rehearsal.sh
else
	echo "==> skipping backup restore rehearsal: GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL is not set"
fi

if [ "${GOGOMAIL_SECURITY_VERIFY:-}" = "1" ]; then
	echo "==> go vet ./..."
	go vet ./...
	if command -v govulncheck >/dev/null 2>&1; then
		echo "==> govulncheck ./..."
		govulncheck ./...
	else
		echo "security verification failed: govulncheck is not installed" >&2
		exit 1
	fi
else
	echo "==> skipping security verification: GOGOMAIL_SECURITY_VERIFY=1 is not set"
fi

echo "==> git status --short"
status="$(git status --short)"
if [ "$status" != "" ]; then
	printf '%s\n' "$status"
	echo "release verification failed: working tree is not clean" >&2
	exit 1
fi
echo "working tree clean"
