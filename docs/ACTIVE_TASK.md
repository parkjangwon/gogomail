# ACTIVE_TASK

## TASK-462: Delivery adaptive domain backoff boundary

### 배경

Cluster-wide farm/domain concurrency is now available through Redis-backed delivery throttling, but large
bulk deployments also need a policy boundary for adaptive destination backoff. When a recipient domain
returns temporary delivery failures, workers should be able to defer later jobs for that domain without
slowing unrelated transactional traffic or other domains. Add a small delivery-domain backoff boundary and
coverage that keeps the transport/retry path deterministic and extensible.

### 구현 대상

- `internal/delivery/handler.go`
- `internal/delivery/retry.go`
- `internal/delivery/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] recipient domain별 adaptive backoff policy/interface 경계를 추가한다.
- [x] temporary failure가 발생한 recipient domain만 backoff 대상이 되고 unrelated domains/farms는 영향을 받지 않음을 검증한다.
- [x] backoff 중인 domain job은 retry scheduling 경로로 defer되고 metrics에 관찰 가능해야 한다.
- [x] permanent failure/bounce는 adaptive backoff를 확장하지 않음을 검증한다.
- [x] `go test -count=1 ./internal/delivery -run 'Backoff|Throttle|Retry|Handler'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TBD after adaptive domain backoff.
