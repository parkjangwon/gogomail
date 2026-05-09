# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-037
- **제목**: SCIM 2.0 Provisioning API (RFC 7643/7644) — Phase 3-B
- **배경**: `internal/scim` 패키지에 타입/필터 유틸만 존재. HTTP 핸들러 없음.
  Okta, Azure AD, Google Workspace 등 IdP가 SCIM 2.0으로 gogomail 사용자를 프로비저닝할 수 있어야 한다.
  AdminAPI와 같은 서버에 `/scim/v2/Users` 엔드포인트를 추가한다.
- **구현 대상**:
  - `internal/httpapi/scim.go`: SCIM 2.0 REST 핸들러
    - `RegisterSCIMRoutes(mux, svc SCIMUserService, token string)`
    - `GET /scim/v2/Users` — 목록 (필터 eq/co/sw, startIndex/count 페이지네이션)
    - `POST /scim/v2/Users` — 생성
    - `GET /scim/v2/Users/{id}` — 단건 조회
    - `PUT /scim/v2/Users/{id}` — 전체 교체
    - `DELETE /scim/v2/Users/{id}` — 삭제
    - `GET /scim/v2/ServiceProviderConfig` — 디스커버리
    - `GET /scim/v2/ResourceTypes` — 디스커버리
  - `internal/maildb/scim_users.go`: `SCIMUserRepository` (users 테이블 CRUD)
    - `GetSCIMUser(ctx, id) → scim.UserResource`
    - `ListSCIMUsers(ctx, filter, offset, limit) → ([]scim.UserResource, int, error)`
    - `CreateSCIMUser(ctx, req) → scim.UserResource`
    - `ReplaceSCIMUser(ctx, id, req) → scim.UserResource`
    - `DeleteSCIMUser(ctx, id) → error`
  - `internal/httpapi/scim_test.go`: 단위 테스트 (fake service)
  - `internal/app/run.go`: `RegisterSCIMRoutes`를 AdminAPI에 연결
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] GET /scim/v2/Users 목록 + 필터 테스트
  - [x] POST /scim/v2/Users 생성 테스트
  - [x] GET /scim/v2/Users/{id} 조회 테스트
  - [x] DELETE /scim/v2/Users/{id} 삭제 테스트
  - [x] ServiceProviderConfig / ResourceTypes 디스커버리 테스트
  - [x] run.go AdminAPI에 등록
- **다음 태스크**: TASK-038

---

## 완료됨

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
