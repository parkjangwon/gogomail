# ACTIVE_TASK

## Current Task

**Task 10: Split internal/app/admin_service.go into domain files** — COMPLETE

## Previous Task Status

**Task 9: Split internal/httpapi/admin.go into route-group files** — COMPLETE

## Current Task Details

**File:** `internal/app/admin_service.go` (was 1,759 lines, now 93 lines)
**Changes:**
- Extracted delivery methods → `admin_service_delivery.go` (backpressure, DAV sync retention, API usage export, mail flow stats)
- Extracted storage methods → `admin_service_storage.go` (attachment cleanup, drive upload sessions, drive nodes)
- Extracted directory methods → `admin_service_directory.go` (delegation, group membership, alias methods)
- Extracted user methods → `admin_service_user.go` (user/domain CRUD, MFA, LDAP/RDBMS sync, IdP config)
- Extracted config methods → `admin_service_config.go` (config store, domain settings, alerts, API keys)
- `admin_service.go` now contains only the `adminService` struct + interfaces (93 lines)
- `go build ./internal/app/...` exits 0
- `go test ./internal/app/... -count=1` → 169 passed

## Next Steps After Current Task

→ All 10 tasks in the codebase improvement plan are complete
