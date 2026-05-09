# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-048
- **제목**: WebDAV PUT — 스트리밍 파일 업로드 + Drive 쿼터 적용
- **배경**: Phase 4 항목 8. WebDAV Gateway는 현재 PROPFIND/MKCOL/GET/DELETE/MOVE/COPY/LOCK/UNLOCK을 지원하지만
  PUT은 501 Not Implemented를 반환한다. macOS Finder, Windows Explorer, Cyberduck 등 프로덕션 클라이언트는
  파일 업로드를 위해 PUT을 기대함. 로드맵 항목 8: "PUT streams body directly to object storage; no full-file buffering",
  항목 10: "Drive quota enforced before PUT completes."
- **구현 대상**:
  - `internal/drive/service.go`: `CreateFile` 메서드 추가 — storage.Store에 직접 스트리밍 후 노드 생성
  - `internal/drive`: `CreateFileRequest` 타입, `BuildNodeObjectPath` 기반 저장소 경로 생성
  - `internal/httpapi/webdav.go`: `WebDAVService` 인터페이스에 `CreateFile` 추가, `handlePut` 구현
  - `internal/httpapi/webdav_service.go`: 어댑터에 `CreateFile` 위임
  - `internal/httpapi/webdav_test.go`: PUT 업로드/덮어쓰기/쿼터 초과 테스트
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] WebDAV PUT으로 파일 업로드 후 GET으로 다운로드 가능
  - [x] 같은 경로 PUT 두 번 시 기존 파일 덮어쓰기(또는 적절한 에러)
  - [x] Drive 쿼터 초과 시 507 Insufficient Storage 반환
  - [x] Content-Type 헤더 기반 MIMEType 저장

## 완료됨

- **TASK-048**: WebDAV PUT — 스트리밍 파일 업로드 + Drive 쿼터 적용 ✅ (2026-05-09)
- **TASK-047**: WebDAV Gateway 런타임 통합 — ModeWebDAV + WebDAVServiceAdapter ✅ (2026-05-09)
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
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```