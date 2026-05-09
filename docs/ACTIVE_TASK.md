# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-044
- **제목**: Batch Worker — Scheduled Mail Flusher + OutgoingMessage.ScheduledAt (Phase 2-C item 1)
- **배경**: `run.go`의 `scheduled-mail-flusher` 잡이 no-op 스텁. `OutgoingMessage` 구조체에
  `ScheduledAt` 필드가 없어 예약 메일이 즉시 outbox에 들어간다(`available_at = now()`).
  flusher 잡이 실제 작동하려면 ① `ScheduledAt` 필드 추가 + outbox `available_at` 반영,
  ② 잡이 `mail.outbound.batch` 토픽에서 오래된 `pending` 항목을 감지해 경고를 로깅해야 한다.
- **구현 대상**:
  - `internal/maildb/outgoing.go`:
    - `OutgoingMessage`에 `ScheduledAt time.Time` 필드 추가
    - `insertOutgoingOutbox` SQL에 `available_at = GREATEST(now(), $N)` 적용
  - `internal/mailservice/service.go`: `RecordOutgoing` 호출 시 `req.ScheduledAt` 전달
  - `internal/maildb/scheduled_mail.go`:
    - `CountStuckScheduledMail(ctx context.Context, stuckAfter time.Duration) (int64, error)` —
      `mail.outbound.batch` topic, status=pending, `available_at <= now()`, created older than stuckAfter
  - `internal/maildb/scheduled_mail_test.go`: nil-db 테스트
  - `internal/app/run.go`: `scheduled-mail-flusher` 잡에 실제 로직 연결
    — `CountStuckScheduledMail(ctx, 10*time.Minute)` 호출 → stuck > 0이면 경고 로그
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] `OutgoingMessage.ScheduledAt` 필드 존재, outbox `available_at` 반영
  - [ ] `CountStuckScheduledMail` nil-db 테스트 통과
  - [ ] `scheduled-mail-flusher` 잡이 no-op에서 실제 DB 조회로 교체됨
- **다음 태스크**: TASK-045

---

## 완료됨

- **TASK-043**: Batch Worker — MFA Grace Period Job (Phase 2-C item 4) ✅ (2026-05-09)
- **TASK-042**: DNS SRV 자동발견 — CalDAV/CardDAV (Phase 4-B item 5) ✅ (2026-05-09)
- **TASK-041**: SSO DB 마이그레이션 + 도메인별 세션 수명 (Phase 3-C item 5) ✅ (2026-05-09)
- **TASK-040**: OIDC PKCE (RFC 7636) + CalDAV IncludeScheduling 활성화 ✅ (2026-05-09)
- **TASK-039**: SSO 플로우 완성 — SAML ACS + OIDC Callback + JIT 프로비저닝 (Phase 3-C 완성) ✅ (2026-05-09)
- **TASK-038**: SSO Configuration Admin API + SSO 플로우 핸들러 (Phase 3-C 초기) ✅ (2026-05-09)
- **TASK-037**: SCIM 2.0 Provisioning API (RFC 7643/7644) — Phase 3-B ✅ (2026-05-09)
- **TASK-036**: LDAP Gateway (RFC 4511) — Phase 3-A ✅ (2026-05-09)
- **TASK-035**: SSE Config Stream — configstore.Notifier 연동 ✅ (2026-05-09)
- **TASK-034**: Batch Worker — Quota Alert Check (quota-alert-check 잡, Phase 2-C) ✅ (2026-05-09)
- **TASK-033**: Batch Worker — Token Cleanup (token-cleanup 잡) ✅ (2026-05-09)
- **TASK-032**: Batch Worker — TOTP Used-Code Pruning ✅ (2026-05-09)
- **TASK-031**: Delta Sync FanOut — mail.stored → deltasync.FanOut 연동 ✅ (2026-05-09)
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
