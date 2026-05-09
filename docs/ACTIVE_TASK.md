# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-039
- **제목**: SSO 플로우 완성 — SAML ACS + OIDC Callback + JIT 프로비저닝 (Phase 3-C 완성)
- **배경**: TASK-038에서 `/auth/sso/saml/acs`와 `/auth/sso/oidc/callback`이 501 스텁으로 남았다.
  이번 태스크에서 실제 assertion 파싱·토큰 교환·JIT 프로비저닝·JWT 발급을 완성한다.
- **구현 대상**:
  - `internal/sso/sso.go` 추가:
    - `ParseSAMLNameID(xmlData []byte) (string, error)` — SAML XML에서 NameID(email) 추출
    - `GenerateOIDCStateForDomain(domainID string) (string, error)` — domainID 인코딩 state 생성
    - `ParseOIDCStateDomain(state string) (string, error)` — state에서 domainID 추출
    - `ParseIDTokenEmail(idToken string) (string, error)` — JWT payload에서 email 클레임 추출
  - `internal/maildb/sso_user.go` 신규:
    - `SSOUserInfo{UserID, DomainID, CompanyID, Email string}`
    - `GetUserByEmail(ctx, email) (SSOUserInfo, error)`
    - `JITCreateSSOUser(ctx, email, domainID, displayName string) (SSOUserInfo, error)`
  - `internal/httpapi/sso.go` 변경:
    - `SSOFlowService` 인터페이스에 `GetUserByEmail`, `JITCreateSSOUser` 추가
    - `RegisterSSORoutes(mux, svc, tm *auth.TokenManager)` — tokenManager 인자 추가
    - SAML ACS 완성: base64 디코드 → XML 파싱 → 사용자 조회/프로비저닝 → JWT 발급
    - OIDC callback 완성: state 디코딩 → 코드 교환 → ID token 파싱 → JWT 발급
  - `internal/app/run.go`: `RegisterSSORoutes(mux, repository, tokenManager)` 업데이트
  - `internal/maildb/sso_user_test.go`: nil-db 테스트
  - `internal/httpapi/sso_test.go`: SAML ACS + OIDC callback 테스트 추가
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] `ParseSAMLNameID` 단위 테스트 통과
  - [x] `ParseIDTokenEmail` 단위 테스트 통과
  - [x] GetUserByEmail / JITCreateSSOUser nil-db 테스트 통과
  - [x] SAML ACS — SAMLResponse → 200 + JWT 토큰 테스트 통과
  - [x] OIDC callback — code exchange → 200 + JWT 토큰 테스트 통과
  - [x] JIT 프로비저닝 — 미존재 사용자 자동 생성 테스트 통과
- **다음 태스크**: TASK-040

---

## 완료됨

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
