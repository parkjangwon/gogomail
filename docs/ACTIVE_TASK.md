# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-074
- **제목**: Mail Log Queries & UI
- **배경**: Phase 8-D. Implement mail operation audit logging and query interface.
  - Log all mail operations (send, receive, delete, move)
  - Query mail logs by date/user/action
  - Performance optimization with indexes
  - Retention policy enforcement
  - Audit trail compliance

- **구현 대상**:
  1. `internal/admin/mail_log_service.go` — Mail log service
  2. `internal/admin/mail_log_service_test.go` — Unit tests
  3. Mail operation logging
  4. Log query and filtering
  5. Retention policy management

- **완료 조건**:
  - [ ] `go test ./...` 통과 (새 테스트 포함)
  - [ ] MailLogService with log recording
  - [ ] Query by user/date/action
  - [ ] Pagination support
  - [ ] Retention policy execution
  - [ ] git status: clean

- **이전 태스크**: TASK-073 ✅ (External RDBMS Sync UI) — COMPLETE

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기 ✓
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```
