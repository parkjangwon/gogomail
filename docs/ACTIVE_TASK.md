# ACTIVE_TASK

## TASK-359: POP3 auth capability assertion helper cleanup

### 배경

POP3 인증 오류/취소 회귀 테스트가 authorization capability 보존을 반복적으로
검증하면서 동일한 CAPA assertion이 여러 곳에 중복되었다. 다음 인증 경로를 더
안전하게 추가할 수 있도록 공통 헬퍼로 정리한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CAPA에 `USER`와 `SASL PLAIN LOGIN`이 남는지 확인하는 공통 헬퍼를 추가한다.
- [x] `AUTH PLAIN` 오류/취소 테스트가 공통 헬퍼를 사용하도록 정리한다.
- [x] `AUTH LOGIN` 오류/취소 테스트가 공통 헬퍼를 사용하도록 정리한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-360: POP3 AUTH PLAIN challenge invalid base64 capability audit
