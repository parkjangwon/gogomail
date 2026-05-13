# ACTIVE_TASK

## TASK-246: SMTP submission sender alias authorization audit

### 배경

Authenticated Submission은 인증된 사용자의 기본 주소와 일치하는 `MAIL FROM`만
허용한다. 실제 DB에는 같은 사용자의 추가 `user_addresses`가 존재할 수 있으므로,
사용자가 소유한 별칭/추가 주소로 발신하는 정상 submission이 거절될 수 있다.
인증 결과에 허용 발신 주소 목록을 포함하고 `MAIL FROM` 검증이 이를 사용해야 한다.

### 구현 대상

- `internal/smtp/submission.go`
- `internal/smtp/submission_test.go`
- `internal/maildb/submission.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] SubmissionUser가 인증된 사용자의 허용 발신 주소 목록을 표현한다.
- [x] MAIL FROM이 기본 주소뿐 아니라 허용된 추가 주소도 통과한다.
- [x] DB authenticator가 사용자의 active 주소 목록을 인증 결과에 포함한다.
- [x] `go test ./internal/smtp ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-247: SMTP submitted object rollback audit
