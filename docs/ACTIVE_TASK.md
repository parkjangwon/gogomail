# ACTIVE_TASK

## TASK-464: Delivery Redis adaptive domain backoff

### 배경

Adaptive domain backoff can now be enabled in delivery workers, but the current policy is process-local.
Large server-farm deployments need the same recipient-domain backoff state to be shared across worker
processes, especially during provider tempfail storms from bulk delivery. Add a Redis-backed
`DomainBackoff` implementation and runtime backend selection while preserving the local default.

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

- [x] Redis domain backoff stores per-domain state shared by worker processes.
- [x] Redis backoff `Check` defers only active domains and allows unrelated domains.
- [x] Redis temporary failure observation extends/caps the per-domain delay deterministically.
- [x] runtime config exposes `GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_BACKEND=local|redis` and YAML equivalent.
- [x] `runDeliveryWorker` wires Redis backoff through the existing Redis client when selected.
- [x] `go test -count=1 ./internal/delivery -run 'Backoff'` 통과.
- [x] `go test -count=1 ./internal/config ./internal/app -run 'Backoff|Delivery'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TBD after Redis adaptive domain backoff.
