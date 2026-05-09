# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-042
- **제목**: DNS SRV 자동발견 — CalDAV/CardDAV `_caldavs._tcp` / `_carddavs._tcp` (Phase 4-B item 5)
- **배경**: TASK-023에서 `/.well-known/caldav` 와 `/.well-known/carddav` Well-Known URI(RFC 6764 §5)를 구현했다.
  iOS/macOS Calendar·Contacts 앱은 그 전에 DNS SRV 레코드(`_caldavs._tcp.domain`, `_carddavs._tcp.domain`)를 조회한다(RFC 6764 §3).
  운영자가 레코드 설정을 검증할 수 있는 Admin API가 없고, SRV → Well-Known 전체 파이프라인 검증 도구도 없다.
- **구현 대상**:
  - `internal/httpapi/dnsdisc.go`:
    - `SRVResult` struct (Host, Port, Priority, Weight)
    - `LookupCalDAVSRV(ctx, domain string) ([]SRVResult, error)` — `_caldavs._tcp.{domain}` 조회
    - `LookupCardDAVSRV(ctx, domain string) ([]SRVResult, error)` — `_carddavs._tcp.{domain}` 조회
    - `DNSDiscoveryChecker` struct: resolver 주입 가능 (테스트용 mock)
    - `CheckAutodiscovery(ctx, domain string) AutodiscoveryReport` — SRV + Well-Known 통합 검증
  - `internal/httpapi/dnsdisc_test.go`:
    - mock resolver로 SRV 응답 테스트
    - SRV 없을 때 fallback Well-Known only 동작 테스트
  - `internal/httpapi/routes.go` (또는 `run.go`):
    - `GET /admin/v1/autodiscovery/check?domain=X` — `AutodiscoveryReport` JSON 반환
  - `internal/httpapi/dnsdisc_handler_test.go`: HTTP handler 통합 테스트
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] `LookupCalDAVSRV` / `LookupCardDAVSRV` mock resolver 테스트 통과
  - [ ] `/admin/v1/autodiscovery/check?domain=X` 200 + JSON report 반환
  - [ ] SRV 없을 때 report에 `srv_found: false` 포함
- **다음 태스크**: TASK-043

---

## 완료됨

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
