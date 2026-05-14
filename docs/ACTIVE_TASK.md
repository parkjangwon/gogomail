# ACTIVE_TASK

## TASK-067: Audit Logs (Level 1 + 2)

### 배경

Mail lifecycle events (`mail.stored`, `mail.delivered`, `mail.bounced`, `mail.delivery_failed`) are already
emitted from SMTP receive/delivery pipelines, but audit log persistence and operator visibility are missing.

The backend-roadmap requires:
1. Audit logs persist mail events with envelope, timestamp, and status metadata
2. Admin API exposes audit log queries (by sender, recipient, domain, date range, status)
3. Audit logs include SMTP Authentication-Results headers for spam/dkim signals
4. Event stream routing handles audit fan-out without hard-coding into SMTP engines

Level 1: PostgreSQL audit log schema, event consumer, and basic persistence  
Level 2: Admin API list/filter endpoints with pagination

### 구현 대상

- `internal/auditlog/schema.go` — audit log domain model (sender, recipient, domain, status, timestamp, event type)
- `internal/auditlog/repository.go` — PostgreSQL persistence (CRUD, list with filters)
- `internal/audit/consumer.go` — event stream consumer: reads `mail.*` events, writes audit logs
- `internal/app/run.go` — wire audit consumer into event worker mode
- `internal/httpapi/auditlog.go` — admin API handlers (list, filter by sender/recipient/domain/date/status)
- `internal/httpapi/*auditlog*_test.go` — integration tests for API
- Database migration for `audit_logs` table
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`
- `docs/openapi.yaml` (if new admin API endpoints)

### 완료 조건

- [x] `audit_logs` PostgreSQL table exists with company_id, domain_id, user_id, actor_id, category, action, target_type, target_id, result, detail columns
- [x] `audit.PostgresRepository` implements Insert, ListWithFilters, and GetByID methods
- [x] audit event handlers (MailStoredHandler, DeliveryStatusHandler, DAVChangeHandler) read mail/dav events from event stream
- [x] handlers write audit log entries with category, action, target_type, target_id, and result
- [x] admin API endpoints: `GET /admin/v1/audit-logs` (list), `GET /admin/v1/audit-logs/{id}` (detail)
- [x] list endpoint supports filtering: `?company_id=`, `?domain_id=`, `?user_id=`, `?category=`, `?action=`, `?target_type=`, `?from_date=`, `?to_date=`
- [x] list endpoint supports pagination: `?limit=50&offset=0` (default limit 50, max 1000)
- [x] audit API endpoints are protected by existing admin auth token path
- [x] `go test -count=1 ./internal/audit ./internal/httpapi -run 'AuditLog'` 통과
- [x] `go test ./...` 통과
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-068: Identity Provider Abstraction
