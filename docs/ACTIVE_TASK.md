# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-032
- **제목**: Batch Worker — TOTP Used-Code Pruning (used-code-cleanup 잡 구현, Phase 2-C)
- **배경**: Phase 2-C "TokenCleanupJob — 만료된 TOTP used-codes, 세션 토큰 정리".
  `totp_used_codes` 테이블이 정의되어 있으나, `used-code-cleanup` 배치 잡은 stub (log only).
  TOTP 코드 재사용 방지를 위해 테이블이 무한히 늘어나는 것을 막아야 한다.
- **구현 대상**:
  - `internal/maildb/mfa.go` (신규): `PruneExpiredTOTPCodes(ctx, cutoff time.Time) (int, error)` — `totp_used_codes` WHERE `used_at < cutoff` 삭제, 삭제 행 수 반환
  - `internal/maildb/mfa_test.go` (신규): 단위 테스트 (fakeDB or integration skip)
  - `internal/app/run.go`: `used-code-cleanup` 잡 stub → `PruneExpiredTOTPCodes` 실제 호출 (cutoff = now - 5분, config 오버라이드 가능)
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] `PruneExpiredTOTPCodes` 테스트 통과 (nil DB, zero cutoff 검증)
  - [x] `run.go`의 `used-code-cleanup` 잡이 실제 pruning 수행
- **다음 태스크**: TASK-033 (백로그에서 선택)

---

## 완료됨

- **TASK-031**: Delta Sync FanOut — mail.stored → deltasync.FanOut 연동 ✅ (2026-05-09)
  - `internal/imapnotify/handler.go`: DeltaSyncNotifier 인터페이스 + WithDeltaSync
  - `internal/app/run.go`: fanOutAdapter + FanOut 생성 및 연결
  - `go test ./...` 5185개 통과

- **TASK-030**: Delta Sync Cursor — Postgres 영속 스토어 ✅ (2026-05-09)

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
