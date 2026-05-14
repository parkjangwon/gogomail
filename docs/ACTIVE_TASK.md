# ACTIVE_TASK

## TASK-460: Delivery throttling distributed coordination audit

### 배경

Delivery worker throttling currently protects per-process farm/domain concurrency, but very large
bulk/server-farm deployments need explicit coverage and a path for cluster-wide coordination so multiple
workers cannot accidentally exceed the intended destination pressure. Audit the current throttler boundary
and add the smallest roadmap-compatible hardening step that improves multi-worker safety without coupling
SMTP protocol code to Redis internals.

### 구현 대상

- `internal/delivery/throttle.go`
- `internal/delivery/*_test.go`
- `internal/config/config.go`
- `internal/app/run.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 현재 farm/domain throttling이 process-local인지, worker farm 구성에서 어떤 위험을 남기는지 코드 기준으로 감사한다.
- [x] cluster-wide throttling을 위한 작은 인터페이스/구성 경계를 추가하거나, 구현 전 선행 테스트가 필요한 경우 실패 테스트를 먼저 작성한다.
- [x] bulk/general/transactional farm과 recipient domain 조합에서 throttling 결정이 deterministic하게 검증된다.
- [x] `go test -count=1 ./internal/delivery -run 'Throttle|Throttl'` 통과.
- [x] `go test -count=1 ./internal/config ./internal/app -run 'Throttl|Delivery'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TBD after distributed throttling audit.
