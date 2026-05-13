# ACTIVE_TASK

## TASK-442: CalDAV Basic auth parity audit

### 배경

CalDAV and CardDAV share the same Basic auth resolver shape, but CardDAV had explicit coverage for
the RFC-required `WWW-Authenticate` challenge and trusted-proxy `X-Forwarded-Proto` normalization
that CalDAV lacked. Add parity coverage so CalDAV auth keeps returning the Basic challenge on 401
paths and accepts uppercase/whitespace HTTPS forwarded-proto values only from trusted proxies.

### 구현 대상

- `internal/caldavgw/auth_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV Basic auth 실패가 `WWWAuthenticate()` challenge interface와 `Basic realm="CalDAV"` 값을 반환하는지 검증한다.
- [x] CalDAV가 trusted proxy에서 온 `" HTTPS "` `X-Forwarded-Proto` 값을 HTTPS로 인정하는지 검증한다.
- [x] `go test -count=1 ./internal/caldavgw -run 'TestBasicAuthResolver(ReturnsUnauthorizedChallenge|AllowsUppercaseForwardedProtoWithWhitespace)'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-443: CardDAV/CalDAV auth repository active policy audit
