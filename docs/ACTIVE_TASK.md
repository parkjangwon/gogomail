# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-045
- **제목**: Batch Worker — OrgChartSyncJob 인터페이스 + 플러그인 경계 (Phase 2-C item 2)
- **배경**: Phase 2-C item 2("OrgChartSyncJob — 인터페이스만, 어댑터는 플러그인")가 미구현.
  배치 워커에 `org-chart-sync` 잡이 등록되어 있지 않다.
  외부 HR 시스템 어댑터가 없을 때는 no-op으로 skip하고,
  어댑터가 주입되면 그것을 호출하는 플러그인 경계만 구현한다.
- **구현 대상**:
  - `internal/orgchart/sync.go`:
    - `OrgChartSyncAdapter` 인터페이스: `SyncOrgChart(ctx context.Context) error`
    - `NoopOrgChartAdapter` struct: 항상 nil 반환 (어댑터 미설정 기본값)
  - `internal/orgchart/sync_test.go`:
    - `TestNoopAdapterReturnsNil`
  - `internal/app/run.go`: `org-chart-sync` 잡 등록 — `NoopOrgChartAdapter` 사용
    (어댑터를 인터페이스로 받아 외부 플러그인이 주입 가능한 구조)
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] `OrgChartSyncAdapter` 인터페이스 정의 존재
  - [ ] `org-chart-sync` 잡이 배치 워커에 등록됨
  - [ ] NoopAdapter 단위 테스트 통과
- **다음 태스크**: TASK-046

---

## 완료됨

- **TASK-044**: Batch Worker — Scheduled Mail Flusher + OutgoingMessage.ScheduledAt (Phase 2-C item 1) ✅ (2026-05-09)
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
