# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-034
- **제목**: Batch Worker — Quota Alert Check (quota-alert-check 잡 구현, Phase 2-C)
- **배경**: Phase 2-C 배치 잡. `quota-alert-check` stub이 존재하나 log only.
  `quota_alert_thresholds` + `quota_alerts` 테이블 인프라 완비.
  users/domains/companies에서 할당량 초과 엔티티를 스캔하여 알림을 기록해야 한다.
- **구현 대상**:
  - `internal/maildb/quota_alert_scan.go` (신규): `ScanAndRecordQuotaAlerts(ctx, defaultWarning, defaultCritical float64) (int, error)` — 단일 INSERT...SELECT CTE로 구현. 사용률이 임계치를 초과하고 24시간 내 동일 유형 알림이 없는 엔티티에 quota_alerts 행 삽입. 삽입 행 수 반환.
  - `internal/maildb/quota_alert_scan_test.go` (신규): nil DB 및 잘못된 임계값 테스트
  - `internal/app/run.go`: `quota-alert-check` stub → `ScanAndRecordQuotaAlerts(ctx, 0.80, 0.95)` 실제 호출
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] `ScanAndRecordQuotaAlerts` 테스트 통과 (nil DB, invalid ratio 검증)
  - [x] `run.go`의 `quota-alert-check` 잡이 실제 스캔 수행
- **다음 태스크**: TASK-035 (백로그에서 선택)

---

## 완료됨

- **TASK-033**: Batch Worker — Token Cleanup (token-cleanup 잡, 만료 공유 링크 삭제) ✅ (2026-05-09)
  - `internal/maildb/token_cleanup.go`: PruneExpiredAttachmentShareLinks + PruneExpiredDriveShareLinks
  - `internal/maildb/token_cleanup_test.go`: nil DB + zero cutoff 검증 4개 테스트
  - `internal/app/run.go`: token-cleanup stub → 실제 호출
  - `go test ./...` 5191개 통과

- **TASK-032**: Batch Worker — TOTP Used-Code Pruning ✅ (2026-05-09)
  - `internal/maildb/mfa.go`: PruneExpiredTOTPCodes
  - `internal/maildb/mfa_test.go`: 2개 테스트
  - `internal/app/run.go`: used-code-cleanup stub → 실제 호출

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
