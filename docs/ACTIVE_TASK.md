# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-050
- **제목**: LDAP Auth — maildb.AuthenticateLDAP 커밋 + 단위 테스트 ✅ (2026-05-09)
- **배경**: `internal/maildb/ldap_auth.go`는 구현됐지만 미커밋 상태. 
  `internal/app/run.go:753`에서 `ldapgw.NewServer(ln, maildb.NewRepository(db), ...)`로
  이미 연결되어 있으나 git 미추적 + 테스트 없음. 빌드/go test는 통과하나
  히스토리 및 리뷰 불투명 상태.
- **구현 완료**:
  - ✅ `internal/maildb/ldap_auth_test.go`: nil DB guard test 추가
  - ✅ `internal/maildb/ldap_auth.go`: git commit (구현 완료)
  - ✅ `internal/ldapgw/pdu_debug_test.go`: git commit
  - ✅ `internal/ldapgw/server_debug_test.go`: git commit
  - ✅ `go test ./...` 통과 (5375 tests)
  - ✅ nil DB에서 패닉 없이 false 반환
  - ✅ commit: 849d3e04

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
- **TASK-008**: SCIM Boolean Parsing & Case-Insensitive Attributes — parser.go ✅ (2026-05-09)
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
