#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

next_generated_files="
apps/webmail/next-env.d.ts
apps/webmail/tsconfig.json
apps/console/next-env.d.ts
apps/console/tsconfig.json
apps/console/tsconfig.tsbuildinfo
"

next_generated_snapshot_dir="$(mktemp -d)"

snapshot_next_generated_files() {
	for file in $next_generated_files; do
		if [ -f "$file" ]; then
			mkdir -p "$next_generated_snapshot_dir/$(dirname "$file")"
			cp "$file" "$next_generated_snapshot_dir/$file"
		fi
	done
}

restore_next_generated_files() {
	for file in $next_generated_files; do
		if [ -f "$next_generated_snapshot_dir/$file" ]; then
			cp "$next_generated_snapshot_dir/$file" "$file"
		fi
	done
	rm -rf "$next_generated_snapshot_dir"
}

snapshot_next_generated_files
trap restore_next_generated_files EXIT

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
