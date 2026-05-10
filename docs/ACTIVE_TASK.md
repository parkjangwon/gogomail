# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-071
- **제목**: LDAP Sync UI & Logs
- **배경**: Phase 8-C. Implement LDAP sync configuration and audit logging.
  - LDAP sync scheduling & manual trigger
  - Sync progress tracking and logs
  - User audit trail for sync operations
  - Sync error handling and retry logic
  - Real-time sync status dashboard

- **구현 대상**:
  1. `internal/admin/ldap_service.go` — LDAP sync service
  2. `internal/admin/ldap_service_test.go` — Unit tests
  3. Sync job scheduling
  4. Audit logging for LDAP operations
  5. Error handling and recovery

- **완료 조건**:
  - [ ] `go test ./...` 통과 (새 테스트 포함)
  - [ ] LDAPService with sync scheduling
  - [ ] Sync progress tracking
  - [ ] Audit log integration
  - [ ] Error handling & retry
  - [ ] git status: clean

- **이전 태스크**: TASK-070 ✅ (LDAP Identity Config) — COMPLETE

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
