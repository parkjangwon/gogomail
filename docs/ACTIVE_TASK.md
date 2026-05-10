# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-075
- **제목**: Statistics & Dashboard Cache
- **배경**: Phase 8-D. Implement admin dashboard statistics and caching.
  - Real-time user/domain/mailbox statistics
  - Cache computed metrics for performance
  - Time-series data collection
  - Dashboard data aggregation
  - Cache invalidation strategy

- **구현 대상**:
  1. `internal/admin/statistics_service.go` — Statistics service
  2. `internal/admin/statistics_service_test.go` — Unit tests
  3. Metric computation
  4. Cache management
  5. Time-series storage

- **완료 조건**:
  - [ ] `go test ./...` 통과 (새 테스트 포함)
  - [ ] StatisticsService with metric collection
  - [ ] Caching layer implementation
  - [ ] Dashboard data aggregation
  - [ ] Cache invalidation
  - [ ] git status: clean

- **이전 태스크**: TASK-074 ✅ (Mail Log Queries & UI) — COMPLETE

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
