# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-043
- **제목**: Batch Worker — MFA Grace Period Job (Phase 2-C item 4)
- **배경**: `run.go`의 `mfa-grace-period` 잡은 현재 no-op 스텁. Phase 2-C item 4("MFAGracePeriodJob — 2FA 유예기간 만료 사용자 처리")를 실제 구현해야 한다.
  도메인 정책으로 MFA가 필수이나 아직 설정하지 않은 사용자에게 유예기간을 부여하고,
  기한 경과 시 배치 잡이 이를 감지해 이벤트를 emit한다.
- **구현 대상**:
  - `migrations/0078_mfa_grace_deadline.sql`:
    `user_mfa_secrets` 테이블에 `mfa_grace_deadline TIMESTAMPTZ NULL` 컬럼 추가
  - `internal/maildb/mfa_grace.go`:
    - `SetMFAGraceDeadline(ctx context.Context, userID string, deadline time.Time) error`
    - `FindExpiredMFAGraceUsers(ctx context.Context, limit int) ([]string, error)` — `enabled=false AND mfa_grace_deadline IS NOT NULL AND mfa_grace_deadline < now()`
    - `ClearMFAGraceDeadline(ctx context.Context, userID string) error` — 처리 완료 후 null 초기화
  - `internal/maildb/mfa_grace_test.go`: nil-db 테스트
  - `internal/app/run.go`: `mfa-grace-period` 잡에 실제 로직 연결
    — `FindExpiredMFAGraceUsers` 호출 → 각 userID 로그 + `ClearMFAGraceDeadline` 호출
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] 마이그레이션 파일 0078 존재
  - [ ] `FindExpiredMFAGraceUsers` nil-db 테스트 통과
  - [ ] `mfa-grace-period` 잡이 no-op 스텁에서 실제 DB 조회로 교체됨
- **다음 태스크**: TASK-044

---

## 완료됨

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
