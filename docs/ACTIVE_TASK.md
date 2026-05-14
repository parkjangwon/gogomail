# ACTIVE_TASK

## TASK-461: Delivery throttling Redis lease counter

### 배경

TASK-460 split delivery throttling into policy evaluation plus a `ThrottleCounter` lease boundary.
The runtime still wires the process-local counter, so a multi-worker/server-farm deployment does not yet
enforce farm/domain concurrency budgets cluster-wide. Add a Redis-backed counter implementation with
atomic multi-key acquire/release semantics and wire it behind explicit runtime configuration.

### 구현 대상

- `internal/delivery/throttle.go`
- `internal/delivery/*_test.go`
- `internal/config/config.go`
- `internal/app/run.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] Redis throttle counter가 multi-key acquire를 atomic하게 처리하고 하나라도 limit 초과면 어떤 key도 증가시키지 않는다.
- [x] release는 acquire된 key만 감소시키고 중복 release에 안전하다.
- [x] runtime config가 `local`/`redis` backend를 명시적으로 선택할 수 있고 잘못된 값은 startup validation에서 거부된다.
- [x] delivery worker는 Redis backend 선택 시 기존 Redis client를 공유해 `CoordinatedThrottler`에 연결한다.
- [x] `go test -count=1 ./internal/delivery -run 'Throttle|Throttl'` 통과.
- [x] `go test -count=1 ./internal/config ./internal/app -run 'Throttl|Delivery'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TBD after Redis throttle counter.
