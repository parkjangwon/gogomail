# ACTIVE_TASK

## TASK-463: Delivery adaptive domain backoff runtime wiring

### 배경

TASK-462 added the delivery `DomainBackoff` boundary and in-memory adaptive policy, but production workers
cannot enable it through runtime configuration yet. Wire adaptive domain backoff into `delivery-worker`
behind explicit config so operators can protect ordinary users from provider tempfail storms during bulk
delivery ramps without changing code.

### 구현 대상

- `internal/config/config.go`
- `internal/config/validate.go`
- `internal/config/config_file.go`
- `internal/app/run.go`
- `internal/app/*_test.go`
- `internal/config/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] runtime config exposes `GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_ENABLED`, base delay, and max delay with YAML equivalents.
- [x] startup validation rejects nonpositive/invalid adaptive backoff durations when enabled.
- [x] `runDeliveryWorker` wires `InMemoryDomainBackoff` when adaptive backoff is enabled.
- [x] config/app tests cover env loading, YAML loading where appropriate, validation, and worker wiring helper behavior.
- [x] `go test -count=1 ./internal/config ./internal/app -run 'Backoff|Delivery'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TBD after adaptive domain backoff runtime wiring.
