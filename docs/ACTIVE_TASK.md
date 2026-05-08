# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-004
- **제목**: Phase 2-C — Batch Worker & Distributed Job Lock
- **배경**: pg_try_advisory_lock 기반 분산 잡 락을 구현하고, 배치 워커 모드를 추가한다.
  여러 인스턴스가 동시에 실행필 때 하나만 잡을 실행하도록 보장한다.
- **구현 대상**: internal/batchlock, batch-worker 모드, job registry, graceful shutdown

### 완료 조건

- [x] `internal/batchlock` 패키지: `PostgresJobLock` (`pg_try_advisory_lock`)
- [x] `--mode=batch-worker` wiring: job registry + ticker loop + graceful shutdown
- [x] 초기 등록 잡 5개 구현 (ScheduledMailFlusher, QuotaAlertCheck, MFAGracePeriod, TokenCleanup 등)
- [x] 테스트: 동시 2 인스턴스 → 하나만 실행 검증
- [x] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-005 — Phase 2-D 실시간 설정 전파 (SSE) + 스코프 보안**

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + docs), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```
