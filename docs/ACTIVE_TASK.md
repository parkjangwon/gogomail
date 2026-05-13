# ACTIVE_TASK

## TASK-368: POP3 auth success test helper cleanup

### 배경

POP3 인증 성공 회귀 테스트들이 transaction CAPA에서 auth-only capability 제거와
`STAT` 성공을 반복적으로 검증하고 있다. 성공 상태 검증을 공통 헬퍼로 정리해
다음 인증 메커니즘 테스트를 더 안전하게 추가한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] transaction CAPA와 `STAT` 성공을 함께 확인하는 인증 성공 헬퍼를 추가한다.
- [x] `AUTH PLAIN` initial/challenge 성공 테스트가 공통 헬퍼를 사용하도록 정리한다.
- [x] `AUTH LOGIN` 성공 테스트가 공통 헬퍼를 사용하도록 정리한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-369: POP3 USER PASS transaction capability audit
