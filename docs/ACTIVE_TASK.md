# ACTIVE_TASK

## ID: COMPLETE

SMTP 수신 도메인별 발신 레이트 리밋 구현 완료 (2026-05-26)

- `internal/delivery/rate_limiter.go`: `RateLimiter` 인터페이스, `DomainRateLimitPolicy`, `InMemoryDomainRateLimiter`, `RateLimitError` (신규 파일)
  - 고정 1분 윈도우 카운터, 클럭 주입 가능 (테스트용)
  - 도메인별 설정 + 기본값, 0 = 무제한
  - 멀티-수신자 잡: 모든 도메인이 한계 미만일 때만 카운터 증가
- `internal/delivery/rate_limiter_test.go`: 7개 단위 테스트 (신규 파일)
- `internal/delivery/handler.go`: `rateLimiter` 필드, `WithRateLimiter` 메서드, `HandleEvent` 통합 (백오프 확인 후, 스로틀 획득 전)
- `internal/delivery/handler_test.go`: 2개 핸들러 통합 테스트 추가
- `internal/delivery/metrics.go`: `MetricRateLimited` 상수 추가
- `internal/config/config.go`: `DeliveryRateLimitEnabled`, `DeliveryDefaultRateLimitPerMinute`, `DeliveryDomainRateLimitPerMinute` 필드
- `internal/app/run.go`: 설정에서 `InMemoryDomainRateLimiter` 생성 및 wiring
- `go test -short -count=1 ./...`: 6162 passed

**설정 환경 변수**:
- `GOGOMAIL_DELIVERY_RATE_LIMIT_ENABLED=true` — 레이트 리밋 활성화
- `GOGOMAIL_DELIVERY_DEFAULT_RATE_LIMIT_PER_MINUTE=60` — 기본 분당 메시지 수
- `GOGOMAIL_DELIVERY_DOMAIN_RATE_LIMIT_PER_MINUTE=gmail.com:100,yahoo.com:50` — 도메인별 설정

## Next Steps

`docs/NEXT_STEPS.md` 백로그에서 다음 태스크를 선택할 것.
