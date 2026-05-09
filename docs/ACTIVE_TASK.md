# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-031
- **제목**: Delta Sync FanOut — mail.stored 이벤트 → deltasync.FanOut 연동 (Phase 7-B item 2)
- **배경**: Phase 7-B "IMAP IDLE fans out real-time mailbox events to connected clients".
  `deltasync.FanOut`이 정의되어 있으나 `imapnotify.MailStoredHandler`와 연결되지 않음.
  mail.stored 이벤트 처리 시 `MailboxEventBroker`(IMAP IDLE)와 동시에 `FanOut`(델타 싱크)에도 알림이 가야 함.
- **구현 대상**:
  - `internal/imapnotify/handler.go`: `DeltaSyncNotifier` 인터페이스 + `WithDeltaSync` 추가; `HandleEvent`에서 FanOut 알림 호출
  - `internal/app/run.go`: `deltasync.FanOut` 생성, `fanOutAdapter` 구현, `newIMAPMailboxEventRouter`에 연결
- **완료 조건**:
  - [x] `go test ./...` 통과 (5185개)
  - [x] `TestMailStoredHandlerNotifiesDeltaSync` 통과 — mail.stored 처리 시 DeltaSyncNotifier 호출 확인
  - [x] `TestMailStoredHandlerSkipsDeltaSyncWhenNil` 통과 — nil notifier 안전 처리
  - [x] `run.go` FanOut 연결 확인
- **다음 태스크**: TASK-032 (백로그에서 선택)

---

## 완료됨

- **TASK-030**: Delta Sync Cursor — Postgres 영속 스토어 ✅ (2026-05-09)
  - `migrations/0076_device_sync_cursors.sql`: `device_sync_cursors` 테이블 + 인덱스
  - `internal/deltasync/postgres.go`: `PostgresCursorStore` — Save/Get/ListByMailbox/Delete
  - `internal/deltasync/postgres_integration_test.go`: 5개 통합 테스트 (DB 없으면 skip)
  - `go test ./...` 5182개 통과

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
