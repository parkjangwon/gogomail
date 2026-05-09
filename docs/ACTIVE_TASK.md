# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-040
- **제목**: OIDC PKCE (RFC 7636) — S256 code_challenge + CalDAV IncludeScheduling 활성화
- **배경**: Phase 3-C item 2는 "PKCE + authorization code flow (RFC 7636)"을 요구하나
  TASK-038/039에서 state만 사용하고 code_challenge는 미구현. 또한 CalDAV handler의
  `IncludeScheduling` 플래그가 false로 남아 POST /inbox, /outbox가 403을 반환함.
- **구현 대상**:
  - `internal/sso/sso.go`:
    - `GenerateOIDCStateWithPKCE(domainID string) (state, codeVerifier string, error)` — code_verifier 생성 + state 인코딩
    - `ParseOIDCStateFields(state string) (domainID, codeVerifier string, error)` — state 디코딩 (도메인 + code_verifier 분리)
    - `PKCEChallenge(verifier string) string` — BASE64URL(SHA256(verifier))
  - `internal/httpapi/sso.go`:
    - OIDC initiate: `GenerateOIDCStateWithPKCE` 사용, `code_challenge=S256` 포함
    - OIDC callback: `ParseOIDCStateFields`로 code_verifier 추출, 토큰 교환 시 전달
    - `exchangeOIDCCode`에 `codeVerifier string` 인자 추가
  - `internal/app/run.go`:
    - `caldavgw.Handler.IncludeScheduling = true` (GOGOMAIL_CALDAV_SCHEDULING 환경변수 게이트)
  - `internal/config/config.go`: `CalDAVScheduling bool` 필드 추가
  - 테스트:
    - `internal/sso/sso_test.go`: PKCE round-trip 테스트
    - `internal/httpapi/sso_test.go`: OIDC initiate에 code_challenge 포함 검증, callback code_verifier 전달 검증
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] `PKCEChallenge(v)` = `BASE64URL(SHA256(v))` 테스트
  - [x] OIDC initiate Location 헤더에 `code_challenge_method=S256` 포함
  - [x] OIDC callback — token endpoint에 `code_verifier` 전달됨
  - [x] CalDAV POST /inbox (IncludeScheduling=true) → 204 테스트
- **다음 태스크**: TASK-041

---

## 완료됨

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
