#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

if ! command -v pnpm >/dev/null 2>&1; then
	echo "frontend verification failed: pnpm is not installed" >&2
	exit 127
fi

echo "==> webmail type-check"
pnpm -C apps/webmail type-check

echo "==> webmail helper tests"
pnpm -C apps/webmail test:compose-helpers
pnpm -C apps/webmail test:mail-page-helpers
pnpm -C apps/webmail test:security-helpers

echo "==> console type-check"
pnpm -C apps/console type-check

if [ "${GOGOMAIL_FRONTEND_E2E:-}" = "1" ]; then
	echo "==> webmail E2E"
	pnpm -C apps/webmail test:e2e
	echo "==> console E2E"
	pnpm -C apps/console test:e2e
else
	echo "==> skipping frontend E2E: GOGOMAIL_FRONTEND_E2E=1 is not set"
fi

if [ "${GOGOMAIL_FRONTEND_BUILD:-}" = "1" ]; then
	echo "==> webmail build"
	pnpm -C apps/webmail build
	echo "==> console build"
	pnpm -C apps/console build
else
	echo "==> skipping frontend production builds: GOGOMAIL_FRONTEND_BUILD=1 is not set"
fi
