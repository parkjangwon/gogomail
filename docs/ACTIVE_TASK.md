# ACTIVE_TASK

## Current Task

**Task 9: Split internal/httpapi/admin.go into route-group files** — COMPLETE

## Previous Task Status

**Task 8: Extract DMRoomList, DMMessageList, DMComposer from DMPanel** — COMPLETE

## Current Task Details

**File:** `internal/httpapi/admin.go` (was 8,901 lines, now 420 lines)
**Changes:**
- Extracted types/constants → `admin_types.go` (357 lines)
- Extracted middleware → `admin_middleware.go` (316 lines)
- Extracted company routes → `admin_company.go` (459 lines)
- Extracted domain routes → `admin_domain.go` (716 lines)
- Extracted user routes → `admin_user.go` (550 lines)
- Extracted operations routes → `admin_operations.go` (306 lines)
- Extracted directory routes → `admin_directory.go` (350 lines)
- Extracted storage routes → `admin_storage.go` (536 lines)
- Extracted usage routes → `admin_usage.go` (864 lines)
- Extracted mail routes → `admin_mail.go` (644 lines)
- Extracted system handlers → `admin_system.go` (3,510 lines)
- Updated `openapi_contract_test.go` to scan all new admin_*.go files
- `go build ./internal/httpapi/...` exits 0
- `go test ./internal/httpapi/... -count=1` → 1101 passed

## Next Steps After Current Task

→ Task 10: Split internal/app/admin_service.go into domain files
