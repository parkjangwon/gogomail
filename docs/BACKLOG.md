# gogomail Task Backlog

에이전트는 이 파일에서 첫 번째 미완료 항목( `[ ]` )을 꺼내 `ACTIVE_TASK.md`로 이동한다.
완료된 태스크는 `[x]`로 체크한다. 순서를 바꾸거나 건너뛰지 않는다.

---

## Phase 2: Runtime Config Store & Settings Hierarchy

- [x] TASK-001: iMIP RFC 6047 wire format 테스트 추가 (`internal/scheduling/handler_test.go`)
- [x] TASK-002: Phase 2-A — Runtime Config Store
  - Migration: `runtime_config` 테이블 + `companies.parent_id` 자기참조 컬럼
  - `internal/configstore` 패키지: `ConfigStore` 인터페이스 + `PostgresConfigStore` (LISTEN/NOTIFY + 인메모리 캐시)
  - Admin API CRUD: `GET/POST/PUT/DELETE /admin/v1/companies/{id}/config/{key}`, 동일 domain/user 경로
  - Propagate API: `POST /admin/v1/companies/{id}/config/propagate?scope=subtree|children|domains`
  - 생성 훅: 자회사/도메인 생성 시 직속 부모 설정 자동 복사
  - 테스트: 트리 해결 순서, locked 차단, propagate 전파 범위, 생성 복사
  - 참고: `docs/backend-roadmap.md` § Phase 2 › 2-A

- [ ] TASK-003: Phase 2-B — 2FA / TOTP (RFC 6238)
  - Migration: `user_mfa_secrets`, `totp_used_codes` 테이블
  - `internal/authmfa` 패키지: TOTP 생성/검증, ±2 window, 리플레이 방지
  - Recovery codes (8개 단일 사용)
  - Auth flow 연동: `auth.mfa.mode` 설정값 기반 강제/선택/비활성화
  - JWT 클레임 `mfa_verified: true` 추가
  - 참고: `docs/backend-roadmap.md` § Phase 2 › 2-B

- [ ] TASK-004: Phase 2-C — Batch Worker & Distributed Job Lock
  - `internal/batchlock` 패키지: `PostgresJobLock` (`pg_try_advisory_lock`)
  - `--mode=batch-worker` wiring: job registry + ticker loop + graceful shutdown
  - 초기 등록 잡 5개 구현 (ScheduledMailFlusher, QuotaAlertCheck, MFAGracePeriod, TokenCleanup 등)
  - 테스트: 동시 2 인스턴스 → 하나만 실행 검증
  - 참고: `docs/backend-roadmap.md` § Phase 2 › 2-C

- [ ] TASK-005: Phase 2-D — 실시간 설정 전파 (SSE) + 스코프 보안
  - `internal/configstore.Notifier` 인터페이스 + subscriber fan-out
  - `GET /api/v1/config/stream` (사용자) + `GET /admin/v1/config/stream` (관리자) SSE 엔드포인트
  - 스코프 보안: `user` 스코프 관리자 직접 쓰기 차단 (403)
  - 테스트: DB 설정 변경 → SSE 이벤트 수신 통합 테스트
  - 참고: `docs/backend-roadmap.md` § Phase 2 › 2-D

- [ ] TASK-006: Phase 2-E — Open API 키 관리 (도메인 관리자용)
  - Migration: `domain_api_keys` 테이블 (CIDR 배열 포함)
  - `internal/apikeys` 패키지: 키 생성/검증/CIDR 체크
  - `ApiKeyMiddleware`: `gm_` prefix 감지 → JWT 경로와 분기
  - Admin API CRUD + rotate 엔드포인트
  - 기존 Mail/Calendar/Contacts API에 scope 검증 레이어
  - 테스트: CIDR 허용/차단, 스코프 부족 거부, 만료/폐기 키 거부, rotate 후 구 키 무효화
  - 참고: `docs/backend-roadmap.md` § Phase 2 › 2-E

---

## Phase 3: Enterprise Identity & Directory

- [ ] TASK-007: Phase 3-A — LDAP Gateway (RFC 4511)
  - `internal/ldapgw` 패키지: LDAP v3 프로토콜 리스너
  - BindRequest (simple bind), SearchRequest (cn/mail/uid/displayName 등 RFC 4519 속성)
  - Read-only 강제: Modify/Delete/ModifyDN → `unwillingToPerform`
  - LDAPS (포트 636) + StartTLS 지원
  - 참고: `docs/backend-roadmap.md` § Phase 3 › 3-A

- [ ] TASK-008: Phase 3-B — SCIM 2.0 Provisioning API (RFC 7642/7643/7644)
  - `internal/scimsvc` 패키지: `/scim/v2/Users` + `/scim/v2/Groups` CRUD
  - ServiceProviderConfig + ResourceTypes 디스커버리 엔드포인트
  - ETag 기반 낙관적 잠금, 페이지네이션, 감사 로그
  - 참고: `docs/backend-roadmap.md` § Phase 3 › 3-B

- [ ] TASK-009: Phase 3-C — SAML 2.0 / OIDC SSO
  - SAML 2.0 SP 모드 + OIDC Relying Party 모드 (PKCE, RFC 7636)
  - 도메인별 IdP 설정, JIT 프로비저닝
  - Admin API `/admin/v1/sso-configurations` CRUD
  - 참고: `docs/backend-roadmap.md` § Phase 3 › 3-C

---

## Phase 4: Storage (WebDAV)

- [ ] TASK-010: Phase 4 — Drive WebDAV Gateway (RFC 4918)
  - `internal/webdavgw` 패키지: PROPFIND/PROPPATCH/MKCOL/COPY/MOVE/LOCK/UNLOCK
  - RFC 3744 ACL, RFC 4331 quota 헤더
  - 참고: `docs/backend-roadmap.md` § Phase 4

---

## Phase 5: Mail Security

- [ ] TASK-011: Phase 5-A — Milter Adapter
  - `internal/milter` 패키지: sendmail milter v2/v6 프로토콜
  - 참고: `docs/backend-roadmap.md` § Phase 5

- [ ] TASK-012: Phase 5-B — DNSBL (RFC 5782)
  - SMTP 수신 경로에 DNSBL 조회 플러그인 경계 추가
  - 참고: `docs/backend-roadmap.md` § Phase 5

---

## Phase 6: POP3

- [ ] TASK-013: Phase 6 — POP3 Server (RFC 1939)
  - `internal/pop3d` 패키지: USER/PASS/STAT/LIST/RETR/DELE/NOOP/RSET/QUIT
  - UIDL/TOP/STLS/CAPA/AUTH 확장
  - POP3S (포트 995)
  - 참고: `docs/backend-roadmap.md` § Phase 6

---

## Phase 7: Push Notifications

- [ ] TASK-014: Phase 7-A — FCM / APNs / Web Push Adapters
  - `internal/pushnotify` 패키지: PushSink 인터페이스 + FCM/APNs/WebPush 어댑터
  - device_tokens 테이블, event worker 연동
  - 참고: `docs/backend-roadmap.md` § Phase 7-A

- [ ] TASK-015: Phase 7-B — Delta Sync Boundary
  - 디바이스별 delta-sync cursor, IMAP IDLE fan-out
  - 참고: `docs/backend-roadmap.md` § Phase 7-B

---

## 규칙

- 이 파일의 항목은 `docs/backend-roadmap.md`에서 파생된다.
- 로드맵에 없는 항목은 추가하지 않는다.
- 백로그가 비면 `docs/backend-roadmap.md`에서 다음 Phase 항목을 여기에 추가한다.
- 프론트엔드 태스크는 사용자 승인 없이 추가하지 않는다.
