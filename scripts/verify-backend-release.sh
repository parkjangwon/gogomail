#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

echo "==> go test ./..."
go test ./...

echo "==> go mod tidy -diff"
go mod tidy -diff

if [ "${GOGOMAIL_TEST_DATABASE_URL:-}" != "" ]; then
	echo "==> PostgreSQL integration tests"
	GOGOMAIL_TEST_DATABASE_URL="${GOGOMAIL_TEST_DATABASE_URL}" go test ./internal/maildb ./internal/outbox
else
	echo "==> skipping PostgreSQL integration tests: GOGOMAIL_TEST_DATABASE_URL is not set"
fi

echo "==> git status --short"
git status --short
