# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-073
- **제목**: External RDBMS Sync UI
- **배경**: Phase 8-C. Implement RDBMS sync configuration and monitoring UI.
  - RDBMS sync scheduling & execution
  - Connection test and validation UI
  - Field mapping configuration interface
  - Sync progress tracking
  - Error logging and recovery

- **구현 대상**:
  1. `internal/admin/rdbms_service.go` — RDBMS sync service
  2. `internal/admin/rdbms_service_test.go` — Unit tests
  3. Sync job management
  4. Connection pool handling
  5. Query result transformation

- **완료 조건**:
  - [ ] `go test ./...` 통과 (새 테스트 포함)
  - [ ] RDBMSService with sync scheduling
  - [ ] Connection pool management
  - [ ] Sync progress tracking
  - [ ] Error handling & recovery
  - [ ] git status: clean

- **이전 태스크**: TASK-072 ✅ (External RDBMS Config & Sync) — COMPLETE

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
