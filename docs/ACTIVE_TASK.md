# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-047
- **제목**: WebDAV Gateway 런타임 통합 — WebDAV 핸들러를 앱 런타임에 연결
- **배경**: TASK-021/024/046에서 WebDAV 핸들러(OPTIONS/PROPFIND/MKCOL/GET/DELETE/MOVE/COPY/PROPPATCH/LOCK/UNLOCK)와
  Depth:infinity 가드, 메트릭 계측이 구현됨. 하지만 `RegisterWebDAVRoutes`는 테스트에서만 호출되고
  실제 앱 런타임(`internal/app/run.go`)에는 연결되어 있지 않음. 운영 환경에서 WebDAV 게이트웨이를
  구동하려면 런타임 통합이 필요함.
- **구현 대상**:
  - `internal/httpapi/webdav_service.go`: `drive.Service`를 감싸는 `WebDAVService` 어댑터
  - `internal/config/config.go`: `WebDAVAddr`, `WebDAVDepthInfinityEnabled` 설정 추가
  - `internal/config/config_file.go`, `validate.go`: 설정 파싱/검증
  - `internal/app/mode.go`: `ModeWebDAV` 모드 추가
  - `internal/app/run.go`: `runWebDAVGateway` 함수, 모드 스위치, HTTP 서버 구동
  - `internal/app/run_test.go`: 런타임 통합 테스트
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] `ModeWebDAV` 모드로 WebDAV 서버 구동 가능
  - [x] `WebDAVAddr` 설정으로 리스닝 주소 지정 가능
  - [x] WebDAV 서버가 `drive.Service`를 통해 실제 Drive 노드 서빙
  - [x] `LockNode`/`UnlockNode` 어댑터 스텁 (핸들러는 인메모리 락 사용)
  - [x] `TrashNode` 시그니처 어댑팅 (`drive.Service`는 `(Node, int64, error)` 반환)

## 완료됨

- **TASK-046**: Phase 4-A WebDAV Gateway — LOCK/UNLOCK + Depth:infinity 가드 + 메트릭 ✅ (2026-05-09)
- **TASK-045**: Batch Worker — OrgChartSyncJob 인터페이스 + 플러그인 경계 (Phase 2-C item 2) ✅ (2026-05-09)
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
- **TASK-034**: Batch Worker — Quota Alert Check (Phase 2-C) ✅ (2026-05-09)
- **TASK-033**: Batch Worker — Token Cleanup (token-cleanup 잡) ✅ (2026-05-09)
- **TASK-032**: Batch Worker — TOTP Used-Code Pruning (Phase 2-C) ✅ (2026-05-09)
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