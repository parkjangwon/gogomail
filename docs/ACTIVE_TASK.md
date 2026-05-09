# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-038
- **제목**: SSO Configuration Admin API + SSO 플로우 핸들러 (Phase 3-C 초기)
- **배경**: `internal/sso` 패키지에 SAML/OIDC 유틸만 존재. DB 저장소·HTTP 핸들러 없음.
  도메인별 IdP 설정(sso_configurations)을 Admin API로 관리하고,
  `/auth/sso/initiate`, `/auth/sso/saml/acs`, `/auth/sso/oidc/callback` 엔드포인트로
  SAML/OIDC 플로우를 초기 구현한다.
- **구현 대상**:
  - `internal/maildb/sso_configs.go`: `SSOConfigRepository`
    - `GetSSOConfig(ctx, domainID) → SSOConfig`
    - `UpsertSSOConfig(ctx, cfg SSOConfig) error`
    - `DeleteSSOConfig(ctx, domainID) error`
    - `SSOConfig` 타입: DomainID, Provider (saml|oidc), EntityID, SSOURL,
      Certificate, ClientID, ClientSecret, DiscoveryURL, JITProvisioning bool
  - `internal/httpapi/sso.go`: SSO HTTP 핸들러
    - `GET /auth/sso/initiate?domain={domain}` — SAML AuthnRequest redirect 또는 OIDC auth code redirect
    - `POST /auth/sso/saml/acs` — SAML ACS: assertion 파싱 + session token 발급
    - `GET /auth/sso/oidc/callback?code=&state=` — OIDC code→token 교환 + session 발급
    - `RegisterSSORoutes(mux, svc SSOService)`
  - `internal/httpapi/admin.go`에 `/admin/v1/sso-configurations` CRUD 추가
    - `GET /admin/v1/sso-configurations/{domain_id}`
    - `PUT /admin/v1/sso-configurations/{domain_id}`
    - `DELETE /admin/v1/sso-configurations/{domain_id}`
  - `internal/httpapi/sso_test.go`: 단위 테스트 (fake SSOService)
  - `internal/maildb/sso_configs_test.go`: nil DB 기본 테스트
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] SSOConfig UpsertSSOConfig/GetSSOConfig/DeleteSSOConfig nil-db 테스트
  - [x] initiate 엔드포인트 → SAML redirect / OIDC redirect 테스트
  - [x] Admin API CRUD 테스트 (GET/PUT/DELETE /admin/v1/sso-configurations/{domain_id})
  - [x] run.go에 RegisterSSORoutes 연결
- **다음 태스크**: TASK-039

---

## 완료됨

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
