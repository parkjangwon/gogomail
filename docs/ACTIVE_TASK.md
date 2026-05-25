# ACTIVE_TASK

## Current Task

**Task 4: Split user-mcp tools.ts into domain modules**

## Previous Task Status

**Task 3: Harden Promtail container security** — COMPLETE
- ✓ Added `security_opt: ["no-new-privileges:true"]` to both Promtail services
- ✓ Added `read_only: true` to both Promtail services
- ✓ Added `tmpfs: [/tmp]` to both Promtail services
- ✓ Added security documentation comment explaining Docker socket risk and Loki driver alternative
- ✓ Modified: `docker/docker-compose.monitoring.yml`
- ✓ Modified: `docker/docker-compose.dev.yml`
- ✓ Validated: `docker compose -f docker/docker-compose.dev.yml config` exits 0
- ✓ Verified: `docker compose -f docker/docker-compose.dev.yml config 2>&1 | grep -c "no-new-privileges"` → `1`
- ✓ Committed and pushed

## Current Task Details

**File:** `webmail/src/user-mcp/tools.ts` (need to split into domain modules)
**Structure:** Large monolithic file with mixed domain concerns
**Changes needed:**
- Extract tools by domain
- Ensure backward compatibility via index.ts re-exports
- Document module structure

## Next Steps After Current Task

Refer to `docs/NEXT_STEPS.md` backlog (priority order):
1. Split manage-mcp tools/gogomail.ts
2. Split webmail api.ts by domain
3. TypeScript file splits (UI components)
4. Go package refactoring
5. And more...
