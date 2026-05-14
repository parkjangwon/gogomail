# ACTIVE_TASK

## TASK-465: Delivery farm-aware domain backoff isolation

### 배경

Redis-backed adaptive domain backoff now shares provider tempfail state across delivery workers, but a
domain-only key can make bulk tempfail pressure slow unrelated transactional/general mail to the same
recipient domain. Large deployments need a farm-aware backoff mode so operators can isolate bulk ramps
from ordinary user-facing delivery while still using shared Redis coordination.

### 구현 대상

- `internal/delivery/backoff.go`
- `internal/delivery/*_test.go`
- `internal/config/config.go`
- `internal/config/validate.go`
- `internal/config/config_file.go`
- `internal/app/run.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] domain backoff supports explicit scope modes: `domain` and `farm_domain`.
- [x] `farm_domain` scope defers only the failed farm/domain pair, so bulk tempfail does not defer transactional/general mail to the same domain.
- [x] Redis and local backoff implementations use the same scope semantics.
- [x] runtime config exposes `GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_SCOPE=domain|farm_domain` and YAML equivalent with validation.
- [x] `runDeliveryWorker` passes the configured scope into local/Redis backoff policy.
- [x] `go test -count=1 ./internal/delivery -run 'Backoff'` 통과.
- [x] `go test -count=1 ./internal/config ./internal/app -run 'Backoff|Delivery'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TBD after farm-aware domain backoff isolation.
