# ACTIVE_TASK

## Current Task

**Task 3: Harden Promtail container security**

## Previous Task Status

**Task 2: Add generated TypeScript client to .gitignore** — COMPLETE
- ✓ Added `clients/typescript/index.ts` to `.gitignore` with comment
- ✓ Ran `git rm --cached clients/typescript/index.ts` to untrack
- ✓ Created `clients/typescript/README.md` with regeneration instructions
- ✓ Verified: `git check-ignore -v clients/typescript/index.ts` outputs match
- ✓ Makefile `gen-ts-client` target confirmed functional
- ✓ Committed and pushed

## Current Task Details

**File:** `docs/docker-compose.yaml` (Promtail service section)
**Changes needed:**
- Add network isolation (read-only logging volumes)
- Restrict capabilities
- Add resource limits
- Document security hardening in comments

## Next Steps After Current Task

Refer to `docs/NEXT_STEPS.md` backlog (priority order):
1. TypeScript file splits
2. Go package refactoring
3. Documentation hygiene
4. And more...
