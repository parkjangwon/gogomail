# gogomail next steps

This file is the short task handoff for future coding agents.

---

## 백로그 (에이전트 루프 — 순서대로 처리)

ACTIVE_TASK.md가 COMPLETE이면 여기서 첫 번째 항목을 선택해 ACTIVE_TASK.md로 이동한다.

| ID | 제목 | 상태 |
|----|------|------|
| TASK-016 | Resumable Chunked Upload — Content-Range 범위 커밋 | 완료 |
| TASK-017 | CalDAV/CardDAV 네이티브 클라이언트 호환성 픽스처 | 완료 |
| TASK-018 | IMAP FETCH BODY 실제 클라이언트 픽스처 확장 | 완료 |
| TASK-019 | Drive 파일 공유 — Directory delegation 통합 | 완료 |
| TASK-020 | OpenAPI → TypeScript 클라이언트 생성 | 완료 |
| TASK-021 | WebDAV Gateway — Drive RFC 4918 지원 | 완료 |
| TASK-022 | POP3 게이트웨이 런타임 통합 | 완료 |
| TASK-023 | Well-Known URIs (RFC 6764) — CalDAV/CardDAV 자동발견 | 완료 |
| TASK-024 | WebDAV Quota (RFC 4331) — quota-used-bytes / quota-available-bytes | 완료 |
| TASK-025 | Milter 클라이언트 (RFC 5764) — SMTP 필터 프로토콜 | 완료 |
| TASK-026 | Milter Hook — SMTP 파이프라인 연동 | 완료 |
| TASK-027 | DNSBL/RBL 체크 (RFC 5782) — 멀티존 IP 블랙리스트 | 완료 |
| TASK-028 | Push Notification 패키지 (FCM/APNs/WebPush) | 완료 |
| TASK-029 | 디바이스 토큰 Admin API (Phase 7-A 완성) | 완료 |
| TASK-030 | Delta Sync Cursor — Postgres 영속 스토어 (Phase 7-B) | 완료 |
| TASK-031 | Delta Sync FanOut — mail.stored → deltasync.FanOut 연동 (Phase 7-B item 2) | 완료 |
| TASK-032 | Batch Worker — TOTP Used-Code Pruning (used-code-cleanup 잡, Phase 2-C) | 완료 |
| TASK-033 | Batch Worker — Token Cleanup (token-cleanup 잡, 만료 공유 링크 삭제) | 완료 |
| TASK-034 | Batch Worker — Quota Alert Check (quota-alert-check 잡, Phase 2-C) | 완료 |
| TASK-049 | WebDAV Auth — Bearer token + Basic auth over HTTPS | 완료 |
| TASK-050 | LDAP Auth — maildb.AuthenticateLDAP 커밋 + 단위 테스트 | 완료 |
| TASK-061 | Organization Structure — 조직도 백엔드 (LDAP 연동) | 완료 |
| TASK-062 | Spam Filter Hardening — RFC 5764 Milter 표준 + 스코링 | 완료 |
| TASK-063 | Admin Console Schema + RBAC + Custom Roles | 완료 |
| TASK-064 | Admin Auth & Session — JWT, login, refresh | 완료 |
| TASK-065 | User Management CRUD | 완료 |
| TASK-066 | Organization Management | 완료 |
| TASK-067 | Audit Logs (Level 1 + 2) | 완료 |
| TASK-068 | Identity Provider Abstraction | 완료 |
| TASK-069 | Database Identity Mode | 완료 |
| TASK-070 | LDAP Identity Config & Sync | 완료 |
| TASK-071 | LDAP Sync UI & Logs | 완료 |
| TASK-072 | External RDBMS Config & Sync | 완료 |
| TASK-073 | External RDBMS Sync UI | 완료 |
| TASK-074 | Mail Log Queries & UI | 완료 |
| TASK-075 | Login/Security Audit Logs | 완료 |
| TASK-076 | Statistics & Dashboard | 완료 |
| TASK-077 | API Metering | 완료 |
| TASK-078 | Dashboard UI | 완료 |
| TASK-079 | Audit Policy Config UI | 완료 |
| TASK-080 | Export & Reports | 완료 |
| TASK-081 | Role Management UI | 완료 |
| TASK-082 | Domain Settings UI | 완료 |
| TASK-083 | API Settings UI | 완료 |
| TASK-084 | Alerts & Notifications | 완료 |
| TASK-085 | Admin Console Frontend (Phase 1) | 완료 |
| TASK-086 | Admin Console Frontend (Phase 2) | 완료 |
| TASK-087 | Admin Console Frontend (Phase 3) | 완료 |
| TASK-088 | Mail Infrastructure Hardening | 완료 |
| TASK-089 | Protocol Gateway Hardening | 완료 |
| TASK-090 | Message Storage & Delivery Optimization | 진행 중 |

### TASK-049 상세
- **제목**: WebDAV Auth — Bearer token + Basic auth over HTTPS
- **배경**: Phase 4-A 항목 9. WebDAV Gateway는 현재 `user_id` 쿼리 파라미터로 인증을 우회한다.
  프로덕션 환경에서는 Bearer token(`Authorization: Bearer <token>`) 또는 Basic auth over HTTPS만
  허용해야 한다. Mail API, Admin API, SCIM 엔드포인트는 이미 Bearer token 인증을 구현했으므로
  같은 패턴을 재사용한다.
- **구현 대상**:
  - `internal/httpapi/webdav.go`: `handlePut`, `handleGet`, `handlePropfind` 등 모든 WebDAV 핸들러에서
    `Authorization` 헤더 파싱 — Bearer token 우선, Basic auth 폴백
  - `internal/httpapi/webdav.go`: `X-WebDAV-User-ID` 헤더 제거 (쿼리 파라미터 인증 대체)
  - `internal/httpapi/webdav_test.go`: Bearer token 인증 성공/실패 테스트, Basic auth 테스트
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] Bearer token 없는 요청 시 401 Unauthorized
  - [ ] 유효한 Bearer token 요청 시 정상 처리
  - [ ] Basic auth over HTTPS 시 정상 처리 (HTTP에서는 403)
  - [ ] 쿼리 파라미터 `user_id` 인증 제거
- **다음 태스크**: TASK-050

### TASK-023 상세
- **제목**: Well-Known URIs (RFC 6764) — CalDAV/CardDAV 자동발견
- **배경**: Phase 4-B 항목. Apple Mail, iOS, macOS, Thunderbird는 `/.well-known/caldav`와
  `/.well-known/carddav` URI로 서버를 자동 발견한다. 현재 미구현으로 사용자가 서버 주소를
  직접 입력해야 한다.
- **구현 대상**: `internal/httpapi/wellknown.go` — RFC 6764 §5 준수 리다이렉트 핸들러
  - `GET /.well-known/caldav` → `301` to `/caldav/`
  - `GET /.well-known/carddav` → `301` to `/carddav/`
  - `PROPFIND /.well-known/caldav` → `301` (WebDAV 클라이언트 지원)
  - HTTP/HTTPS 양쪽에서 동작
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] `/.well-known/caldav` 요청 시 301 리다이렉트 응답
  - [ ] `/.well-known/carddav` 요청 시 301 리다이렉트 응답
- **다음 태스크**: TASK-024

### TASK-022 상세
- **제목**: POP3 게이트웨이 런타임 통합
- **배경**: `internal/pop3d`에 POP3 서버 핵심 구현 존재. 앱 런타임 미연결, AUTH 미구현.
- **구현 대상**: `internal/pop3d/pop3d.go`, `internal/mailservice/pop3_adapter.go`,
  `internal/config/config.go`, `internal/app/mode.go`, `internal/app/run.go`
- **완료 조건**: `go test ./...` 통과 + AUTH PLAIN + ModePOP3 구동
- **다음 태스크**: TASK-023

### TASK-017 상세
- **제목**: CalDAV/CardDAV 네이티브 클라이언트 호환성 픽스처
- **배경**: Phase 4-B 하드닝 항목. Apple iCal, Thunderbird Lightning, DAVx⁵ 실제 요청 형태를
  픽스처로 캡처해 `internal/caldavgw` / `internal/carddavgw` 회귀 테스트 추가.
- **구현 대상**: `internal/caldavgw/*_test.go`, `internal/carddavgw/*_test.go` 픽스처 추가
- **완료 조건**: `go test ./internal/caldavgw/... ./internal/carddavgw/...` 통과 + 새 픽스처 최소 5종
- **다음 태스크**: TASK-018

### TASK-018 상세
- **제목**: IMAP FETCH BODY 실제 클라이언트 픽스처 확장
- **배경**: `internal/imapgw` MIME literal fetch가 기본 케이스만 커버.
  Apple Mail, Thunderbird, K-9 Mail 형태의 `BODY[TEXT]`, `BODY[HEADER]`, `BODY[1.TEXT]`
  literal 응답 픽스처를 추가해 회귀 방지.
- **구현 대상**: `internal/imapgw/*_test.go` 픽스처 추가
- **완료 조건**: `go test ./internal/imapgw/...` 통과 + 픽스처 최소 5종
- **다음 태스크**: TASK-019

### TASK-019 상세
- **제목**: Drive 파일 공유 — Directory delegation 통합
- **배경**: `internal/drive` HTTP API는 구현됨. `internal/accesspolicy.DelegatedAccessAuthorizer`도 존재.
  Drive HTTP 엔드포인트에 `drive` scope delegation 체크가 없어 크로스 유저 접근이 미구현.
- **구현 대상**: `internal/httpapi/drive.go` — cross-user 경로에 `DelegatedAccessAuthorizer` 적용
- **완료 조건**: `go test ./...` 통과 + 위임된 read/write/manage 롤로 Drive 접근 테스트
- **다음 태스크**: TASK-020

### TASK-020 상세
- **제목**: OpenAPI → TypeScript 클라이언트 자동 생성
- **배경**: `docs/openapi.yaml`이 완성되어 있음. `openapi-typescript` 또는 `oapi-codegen`으로
  TS 타입/클라이언트 생성 파이프라인 추가. 프론트엔드 게이트와 무관한 백엔드 계약 작업.
- **구현 대상**: `Makefile` 또는 `scripts/gen-ts-client.sh`, `clients/typescript/` 생성물
- **완료 조건**: `make gen-ts-client` 실행 시 `clients/typescript/` 아래 타입 파일 생성
- **다음 태스크**: TASK-021

### TASK-021 상세
- **제목**: WebDAV Gateway — Drive RFC 4918 지원
- **배경**: Phase 4-A. `internal/webdavgw`는 WebDAV XML 유틸만 존재. HTTP 핸들러 + Drive 연동 미구현.
  WebDAV는 OpenAPI로 정의 불가하므로 REST가 아닌 별도 프로토콜로 구현.
- **구현 대상**: `internal/httpapi/webdav.go` — RFC 4918 PROPFIND/PROPPATCH/MKCOL/GET/PUT/DELETE/COPY/MOVE 지원
- **완료 조건**: `go test ./...` 통과 + WebDAV PROPFIND로 Drive 노드 목록 조회 가능
- **다음 태스크**: TASK-022 (백로그 테이블 확장 후)

---

## Read first

Before changing code, read:

1. `AGENTS.md`
2. `docs/CURRENT_STATUS.md`
3. `docs/backend-roadmap.md`
4. `docs/backend-api-contracts.md`
5. `docs/backend-release-readiness.md`
6. `DESIGN.md`
7. `docs/openapi.yaml`
8. `docs/storage-backends.md`
9. recent `git log --oneline`

## Immediate backend priorities

### Redis-backed module nil safety (completed)

- `RedisLimiter.Allow` now guards against nil Redis client, returning `true` (allow)
  instead of panicking, matching the defensive pattern in `RedisFixedWindowLimiter.Allow`.
- `RedisDeduplicator.CheckAndSet` now guards against nil Redis client, returning
  `(true, nil)` (new duplicate, allow) instead of panicking.
- `RedisBackpressure.Accept` and `RedisBackpressure.State` now guard against nil
  Redis client, returning permissive defaults (accept/allow). `SetState` returns an
  error for nil client since explicit state write requires a backend.
- All nil guard fixes include unit test coverage for nil client behavior.

### Database module nil safety and validation (completed)

- `MigrateUp` now rejects nil database handle with explicit error.
- `CurrentMigrationVersion` now rejects nil database handle with explicit error.
- `migrationVersionFromFilename` edge cases now have comprehensive unit test coverage
  for invalid version prefixes, zero versions, negative versions, and malformed filenames.

### mailauth SPF/DMARC test coverage (completed)

- Added comprehensive tests for `splitSPFTerm`, `spfQualifierResult`, and `parseMechanism`
  helper functions covering all SPF qualifiers (+-~?) and mechanism parsing with CIDR.
- Added tests for SPF neutral (`?all`), all mechanisms (a, mx, ptr), and DMARC
  policies (none, quarantine, reject) to improve RFC 7208/6376 compliance verification.

### Mail flow log hybrid storage with configurable stats backend (completed)

- Mail flow logs use hybrid storage: PostgreSQL for ACID audit compliance,
  OpenSearch for scalable aggregation queries.
- `GOGOMAIL_MAIL_FLOW_OPENSEARCH_BOOTSTRAP=true` enables OpenSearch indexing.
- `GOGOMAIL_MAIL_FLOW_STATS_BACKEND=auto|postgres|opensearch` configures stats backend.
- `MailFlowStatsProvider` interface abstracts PostgreSQL and OpenSearch implementations.
- `PostgresMailFlowStatsProvider` uses existing `maildb.Repository` methods directly.
- `OpenSearchMailFlowStatsProvider` uses `MailFlowStatsSearcher` for aggregation.
- Auto mode uses OpenSearch when bootstrap is enabled, falls back to PostgreSQL
  if OpenSearch is unavailable.
- `adminService` bridges `mailflow.MailFlowStatsResult` to `maildb.MailFlowLogStatsView`
  for API compatibility.

### Storage portability

Current state:

- Draft search (`GET /api/v1/drafts/search`) now supports `to`, `cc`, and `bcc`
  filter parameters, with normalization, CR/LF injection guards, length limits,
  unknown-key rejection, SQL filtering against `to_addrs`/`cc_addrs`/`bcc_addrs`,
  and OpenAPI spec coverage aligned.
- Message search (`GET /api/v1/search`) now supports `to`, `cc`, and `bcc` filter
  parameters with normalization, validation, SQL filtering, and full OpenSearch
  integration. The search index stores `to_addrs_lc`, `cc_addrs_lc`, and
  `bcc_addrs_lc` as keyword fields; the OpenSearch query path applies wildcard
  filters for these fields alongside the existing `from_addr_lc` and `subject_lc`
  filters. The `canUseSearchIDSource` bypass for to/cc/bcc is removed — OpenSearch
  relevance search now handles all filter combinations. Webmail capabilities
  `search.filters` and OpenAPI spec updated to match.
- API metering `recordFailOpen` now logs recovered panics with route and method
  context, improving visibility when the metering sink is under backpressure or
  fails.
- Ignored `Close()` errors for Redis clients, network listeners, database
  connections, and IMAP append spools are now logged to `slog` or `stderr`
  across all service entry points (`internal/app/run.go`), ensuring resource
  cleanup failures are observable.
- IMAP `STORE`, `MOVE`, `EXPUNGE`, and their `UID` variants now use unified
  command dispatch branches, removing redundant field-length checks that
  delegated to the same handler logic.
- IMAP mailbox display names now preserve original name spacing by removing
  `strings.TrimSpace` from the `mailbox.Name` return path, with regression
  coverage for spaced mailbox names such as `" INBOX "`.
- Webmail capability discovery now limits advertised message-search filters to
  the runtime-supported `q`, `folder_id`, `from`, `subject`, and
  `has_attachment` query keys, with OpenAPI and regression coverage aligned.
- Admin console capability discovery now has an operation-level OpenAPI
  `/admin/v1` server pin plus a runtime regression proving the public
  `/api/v1` base does not serve it, preventing generated-client base-path
  ambiguity before admin frontend work starts.
- Health probes are now pinned to the service-root OpenAPI server, and service
  info is pinned to `/api/v1`, with runtime regressions for wrong-base forms.
- OpenAPI contract tests now derive registered Mail and Drive API routes from
  `mail.go` and `drive.go` and require every `/api/v1` operation to pin the
  Mail API server at the operation level, preventing generated clients from
  inheriting the global `/admin/v1` server for user-facing routes.
- S3-compatible `ListObjectsV2` success XML now rejects direct text inside
  structured standard object metadata wrappers such as `Owner` and
  `RestoreStatus`, keeping ignored provider text from crossing the storage
  adapter while preserving known AWS child metadata compatibility.
- S3-compatible success responses that include requester-pays
  `x-amz-request-charged` metadata now treat blank, whitespace-only, or
  whitespace-padded values as invalid provider metadata before rejecting exact
  nonblank requester-pays mode as unsupported across adapter success paths.
- S3-compatible full-object `GET` now rejects contradictory `Content-Length`
  metadata even when Go's normalized response length is known to be zero,
  preserving exact provider length identity before exposing a bounded reader.
- S3-compatible offset-zero `200 OK` range compatibility now applies the same
  raw-header versus normalized-length agreement when both metadata surfaces are
  known, preventing downgraded full-window range reads from trusting
  contradictory zero-length provider metadata.
- CalDAV and CardDAV discovery now advertise `sync-collection` in both
  `OPTIONS` DAV tokens and collection `supported-report-set` only when the
  runtime store implements the relevant sync change-log interface.
- CalDAV and CardDAV `REPORT` parsing now rejects duplicate `DAV:limit`
  controls and duplicate nested `DAV:nresults` controls, keeping bounded
  query/sync pagination semantics deterministic before backend scans begin.
- CalDAV and CardDAV `sync-collection` parsing now also rejects duplicate
  `DAV:sync-token` and `DAV:sync-level` controls, keeping sync anchors
  unambiguous before snapshot or change-log work begins.
- CalDAV and CardDAV object `GET`/`HEAD` handling now ignores
  `If-Modified-Since` whenever `If-None-Match` is present, preserving HTTP
  conditional precedence for native DAV clients caching `.ics` and `.vcf`
  bodies.
- CalDAV and CardDAV path/href parsing now rejects encoded path separators
  before URL decoding, preventing `%2F`, `%5C`, `%252F`, or `%255C` from
  remapping principal, collection, or object boundaries inside request paths
  and REPORT hrefs.
- CalDAV and CardDAV object `PUT`/`DELETE` now carry the observed strong
  object ETag into repository mutation guards even for successful
  `If-Match: *` existing-resource preconditions, reducing stale mutation races
  while preserving WebDAV existence semantics.
- CalDAV and CardDAV object `DELETE` now evaluates `If-None-Match` before
  mutation, returning HTTP 412 for `*` or matching ETags and preserving the
  existing `.ics`/`.vcf` representation.
- CalDAV and CardDAV collection `DELETE` now evaluates `If-None-Match` against
  the calendar/address-book collection ETag before recursive deletion,
  returning HTTP 412 for `*` or matching validators and preserving child
  `.ics`/`.vcf` members.
- CalDAV and CardDAV collection `PROPPATCH` uses the same `If-None-Match`
  precondition gate before reading WebDAV XML bodies, keeping metadata
  mutations cheap and fail-closed when clients send `*` or a matching
  collection validator.
- CalDAV and CardDAV collection `DELETE`/`PROPPATCH` now carry the observed
  collection ETag into repository mutation guards after conditional preflight,
  so stale `If-Match: *` collection races are rechecked inside the storage
  transaction before recursive delete or metadata update commits.
- CalDAV `MKCALENDAR` and CardDAV extended `MKCOL` now evaluate collection
  creation preconditions before reading WebDAV XML request bodies: existing
  targets reject matching `If-None-Match`, missing targets reject `If-Match`
  or `If-Unmodified-Since`, and `If-None-Match: *` still permits safe
  create-only requests for absent collections.
- CalDAV/CardDAV collection creation now validates missing-target
  UUID-shaped collection path IDs before conditional create evaluation,
  preserving HTTP 400 syntax failures ahead of 412 state preconditions and XML
  body reads while existing legacy collection IDs retain normal existence
  semantics.
- CalDAV and CardDAV object `PUT` now reject `If-Unmodified-Since` for
  non-existent resources before reading request bodies, keeping timestamp
  preconditions fail-closed for native DAV clients that intended to update an
  existing `.ics` or `.vcf` representation.
- CalDAV and CardDAV webmail REST APIs now expose JSON endpoints for webmail
  frontend integration. `CalendarHandler` uses a `CalendarRepo` interface and
  implements CRUD for calendars and calendar objects with `text/calendar` content
  type support. `ContactHandler` uses a `ContactRepo` interface and implements
  CRUD for address books and contacts with `text/vcard`, `application/vcard+xml`,
  and `text/x-vcard` content type support. Both handlers support `user_id` query
  authentication when `tokenManager` is nil, with `rejectUnknownQueryKeys`
  allowing `user_id` in that mode. ETag-based conditional requests are supported.
  Comprehensive unit tests with fake repository implementations provide coverage.
- Drive JSON mutation handlers now have regression coverage for required
  `application/json` content type, unknown-field rejection, and trailing-token
  rejection before service dispatch.
- Public Drive share-link routes now reject whitespace-normalized or
  non-printable path tokens before limiter/service dispatch, preserving exact
  bearer-token semantics for public metadata and download routes.
- Public Drive shared download routes now reject malformed or unsatisfiable
  byte ranges with HTTP 416 plus `Content-Range: bytes */<size>` before any
  full/range object open, and OpenAPI pins that shared-download range error
  contract for generated public clients.
- Admin storage capability support flags now come from active backend labels,
  avoiding over-broad local/NFS, MinIO, or AWS/S3-compatible claims. Explicit
  compatibility labels are now extensible safe tokens in the Admin API surface
  contract, sorted/de-duplicated by runtime while unknown labels do not
  activate support booleans. OpenAPI now also marks active labels as non-empty
  and unique, and storage operations as unique, with runtime coverage for the
  default advertised operation list.
- S3-compatible ETags are now validated as bounded printable ASCII opaque
  identity metadata across optional `PutObject` success headers, `HEAD`/`Stat`,
  `ListObjectsV2`, and `CopyObjectResult`, rejecting malformed quote nesting,
  whitespace padding, controls, and non-ASCII provider values before they reach
  shared storage callers.
- S3-compatible `HEAD`/`Stat` content-type metadata now must be an unpadded
  valid ASCII MIME media type with optional parameters, preventing malformed or
  whitespace-normalized provider MIME metadata from reaching Drive, cleanup, or
  reconciliation callers.
- Shared storage `DeletePrefix` now fails closed when a truncated `List` page
  omits its continuation cursor, before deleting any listed object, and S3
  coverage verifies that continuation tokens are carried into the next
  cleanup page.
- Shared storage `DeletePrefix` now also revalidates every listed object
  against the requested canonical prefix before deletion, preserving completed
  progress and returning a structured out-of-scope listing error if a backend
  returns safe sibling keys.
- S3-compatible `List` now validates continuation tokens only on truncated
  pages and clears final-page cursors, so compatible providers that include
  unusable `NextContinuationToken` values on `IsTruncated=false` pages do not
  break Drive/lifecycle listings.
- Local/NFS `List` now returns a single-object final page when the requested
  prefix exactly names an existing object, matching S3 `Prefix` behavior and
  keeping exact-object reconciliation probes portable across local, NFS, MinIO,
  and AWS/S3-compatible storage.
- S3-compatible `List` now rechecks provider-returned keys against the
  requested logical prefix after canonical bucket-prefix mapping, so
  S3-compatible cleanup scans and `DeletePrefix` cannot touch sibling prefixes
  even if a provider returns an overly broad page.
- S3-compatible `GetRange` now accepts safe `200 OK` full-range compatibility
  responses only when `Content-Range` or `Content-Length` proves the body maps
  exactly to the requested byte window, while still rejecting ambiguous
  non-zero-offset or mismatched responses.
- S3-compatible `206 Partial Content` range responses reject invalid or
  mismatched `Content-Length` headers when present, draining the body before
  returning an error so provider metadata contradictions do not reach callers
  as apparently valid bounded readers.
- S3-compatible `Content-Range` parsing rejects internal whitespace inside the
  `start-end/size` byte-range grammar, keeping malformed provider metadata
  from being normalized before range validation.
- S3-compatible `200 OK` range compatibility responses with a matching
  `Content-Range` now also validate any present `Content-Length` against the
  requested window, matching the stricter `206 Partial Content` path.
- S3-compatible range responses now reject duplicate `Content-Range` headers
  on both `206 Partial Content` and safe `200 OK` compatibility paths, so
  byte-window identity cannot depend on HTTP header collapse.
- S3-compatible `Content-Length` parsing requires exact unsigned decimal
  digits for `HEAD` metadata and range-response validation, rejecting signed
  or whitespace-padded values such as `+5` or ` 5` instead of normalizing them
  as valid sizes. Duplicate `Content-Length` headers are also rejected for
  `HEAD`/`Stat`, full-object `GET`, and range validation before callers see a
  bounded reader or object metadata.
- S3-compatible `HEAD`/`Stat` now validates the raw `Content-Length` header
  even when the HTTP response already has a populated `ContentLength` field,
  rejecting malformed or contradictory provider metadata. Duplicate
  `Last-Modified`, `ETag`, and `Content-Type` headers now also fail closed
  before metadata parsing, and present-but-blank or malformed
  `Last-Modified`, `ETag`, and `Content-Type` metadata are rejected instead of
  silently being exposed as empty optional metadata. `ETag` and `Content-Type`
  headers must also be unpadded before quote cleanup or MIME parsing.
- S3-compatible full-object `GET` now validates present `Content-Length`
  headers with the same exact unsigned decimal grammar and returns a bounded
  reader for known-length bodies, reporting `io.ErrUnexpectedEOF` on truncated
  full-object responses.
- S3-compatible `HEAD`/`Stat` now rejects blank or malformed present
  `Last-Modified` headers instead of silently returning zero timestamps,
  while preserving HTTP optional-whitespace compatibility around otherwise
  valid timestamp values.
- S3-compatible `Content-Range` start, end, and total-size numbers reuse that
  unsigned decimal parser, rejecting signed values such as `bytes +1-3/5` or
  `bytes 1-3/+5` before range metadata can be normalized.
- S3-compatible `ListObjectsV2` object-size parsing now also requires unsigned
  decimal digits, rejecting signed or whitespace-padded `<Size>` values such
  as `+5` or ` 5 ` before list metadata reaches cleanup, Drive, or
  reconciliation callers.
- S3-compatible `ListObjectsV2` object entries reject missing or blank `<Key>`
  elements instead of silently skipping malformed provider entries before
  prefix mapping and cleanup scans.
- S3-compatible `ListObjectsV2` pagination control requires an explicit
  canonical `<IsTruncated>true</IsTruncated>` or `<IsTruncated>false</IsTruncated>`
  value, rejecting missing or non-canonical forms before deciding whether a
  page is final.
- S3-compatible `ListObjectsV2` pagination controls now also reject duplicate
  top-level `<IsTruncated>` or `<NextContinuationToken>` elements, preventing
  ambiguous provider pages from silently changing final/truncated state or
  cursor identity during XML unmarshalling.
- S3-compatible `CopyObject` success XML accepts namespace-free or AWS S3
  namespace `CopyObjectResult` roots only, rejecting same-local-name XML from
  unexpected namespaces before copy/move is reported successful.
- S3-compatible `CopyObjectResult` core child elements now use the same
  namespace boundary as the root, preventing foreign-namespace `ETag`,
  `LastModified`, or embedded `Error` elements from being collapsed into a
  successful copy response.
- S3-compatible `CopyObjectResult` `ETag` metadata is now required and uses
  the same bounded safe single-line validation as `Stat` and `List`, rejecting
  missing, blank, whitespace-padded, double-quoted, or malformed copy success
  metadata before copy/move callers treat the provider response as durable.
- S3-compatible `CopyObjectResult` `LastModified` metadata now rejects
  present-but-blank, malformed, or whitespace-padded timestamp values instead
  of accepting ambiguous successful copy metadata.
- S3-compatible `CopyObjectResult` success XML now rejects duplicate top-level
  `ETag` or `LastModified` metadata, nested `Error` elements, and unknown
  top-level success children, formatting nested standard S3 error details as
  bounded one-line diagnostics with request-id and host-id context instead of
  collapsing provider-side copy failures or ambiguous copy metadata into a
  successful copy/move result. Top-level and nested copy
  `Error` bodies share the same capped streaming XML field parser as status
  errors.
- S3-compatible `ListObjectsV2` response XML applies the same namespace
  boundary to `ListBucketResult`, accepting namespace-free or AWS S3 namespace
  roots only before pagination, prefix filtering, cleanup, or Drive callers see
  listed object metadata.
- S3-compatible `ListObjectsV2` control and object-metadata elements now use
  the same namespace boundary as the root, preventing foreign-namespace
  `IsTruncated`, `Contents`, `Key`, `Size`, `ETag`, or `LastModified` elements
  from being treated as canonical provider metadata.
- S3-compatible `ListObjectsV2` standard metadata such as `Prefix`, `Name`,
  `KeyCount`, `MaxKeys`, `StorageClass`, and `Owner` now also shares that
  namespace boundary, while normal namespace-free and AWS-namespaced metadata
  remains compatible.
- S3-compatible `ListObjectsV2` simple standard metadata such as `Prefix` and
  `StorageClass` now rejects nested XML before unmarshalling, while AWS
  structured fields such as `Owner` remain accepted when nested children are
  namespace-free or AWS S3-namespaced, use known AWS child names, and do not
  repeat the same child name.
- S3-compatible `ListObjectsV2` object metadata now rejects duplicate
  single-value `StorageClass` and `ChecksumType` elements, while preserving
  repeated `ChecksumAlgorithm` compatibility for providers that return multiple
  checksum algorithms.
- S3-compatible `ListObjectsV2` simple root metadata now rejects duplicate
  standard elements such as `<KeyCount>` or `<Prefix>`, and validates
  `KeyCount` as an unsigned decimal that exactly matches the returned
  `<Contents>` count before pagination or cleanup callers trust the page.
- S3-compatible `ListObjectsV2` root `<MaxKeys>` metadata now rejects signed,
  whitespace-padded, or under-counting values before list pages reach Drive,
  lifecycle cleanup, or reconciliation callers.
- S3-compatible `ListObjectsV2` root `<KeyCount>` and `<MaxKeys>` metadata now
  reject present-but-blank values instead of treating empty elements as omitted
  optional numeric metadata.
- S3-compatible `ListObjectsV2` root `<Prefix>` metadata, when present, must
  be nonblank and match the requested provider prefix exactly, including
  configured storage prefixes, while providers that omit the echo remain
  compatible.
- S3-compatible `ListObjectsV2` root `<Name>` metadata, when present, must
  be nonblank and match the configured bucket name so wrong-bucket or
  blank-bucket compatible-provider responses fail closed.
- S3-compatible `ListObjectsV2` root `<EncodingType>` metadata is rejected
  when present, including blank elements, because gogomail does not request
  encoded-key list mode.
- S3-compatible `ListObjectsV2` root `<ContinuationToken>` metadata, when
  present, must match an explicitly requested cursor exactly and is rejected
  when no request cursor was sent.
- S3-compatible `ListObjectsV2` root `<StartAfter>` metadata is rejected when
  present, including blank elements, because gogomail does not request
  start-after list mode and relies on continuation-token pagination.
- S3-compatible requester-pays success response headers are rejected across the
  adapter because requester-pays mode is outside the current portable storage
  contract.
- S3-compatible `ListObjectsV2` object `ChecksumType` metadata now shares the
  same namespace and simple-field nested-XML boundary as `ChecksumAlgorithm`.
- S3-compatible `ListObjectsV2` object metadata rejects duplicate
  single-value `StorageClass`, `ChecksumType`, `Owner`, and `RestoreStatus`
  fields before XML unmarshalling can collapse ambiguous provider metadata,
  while preserving repeated `ChecksumAlgorithm` compatibility.
- S3-compatible `ListObjectsV2` delimiter grouping controls are rejected:
  returned `<Delimiter>` elements, including blank elements, and
  `<CommonPrefixes>` cannot be treated as ordinary object pages because
  gogomail does not request grouped listing.
- S3-compatible `ListObjectsV2` mapped object keys with leading/trailing
  whitespace or encoded separators now fail closed instead of being silently
  skipped after they match the configured storage prefix.
- S3-compatible `ListObjectsV2` object `LastModified` metadata now fails
  closed when a present provider value is blank, malformed, or
  whitespace-padded, while still allowing missing values for compatible
  providers that omit optional timestamp metadata.
- S3-compatible `ListObjectsV2` object `ETag` metadata now fails closed when a
  present provider value is blank, whitespace-padded, malformed, line-bearing,
  double-quoted, empty after quote cleanup, or larger than the bounded metadata
  limit, instead of silently dropping suspect listed-object metadata.
- S3-compatible `ListObjectsV2` object metadata now rejects duplicate
  per-object `<Key>`, `<Size>`, `<ETag>`, or `<LastModified>` elements before
  XML unmarshalling can collapse conflicting provider values into one listed
  object. Nested child elements inside those simple metadata fields are also
  rejected before list results reach cleanup, Drive, or reconciliation callers.
- Shared storage object paths and prefixes now reject encoded separators such
  as `%2F` and `%5C` before local/NFS or S3-compatible adapter use, preserving
  one portable logical key boundary across local filesystems, MinIO, AWS S3,
  and strict compatible gateways.
- Local/NFS `Move` now keeps the normal filesystem rename fast path but falls
  back to copy-delete on cross-device `EXDEV`, preserving backend-neutral file
  relocation across NFS/bind-mount style deployments where a direct rename can
  fail despite readable source and writable destination paths.
- Admin console capability OpenAPI security now models both `X-Admin-Token`
  and bearer-token auth alternatives, with runtime coverage for ambiguous
  credential rejection.
- API usage export capability discovery now has the same Admin API server and
  admin-auth OpenAPI metadata plus runtime auth coverage.
- Admin readiness bootstrap operations now also pin the Admin API server and
  admin-auth alternatives in OpenAPI for API usage ledger retention readiness,
  DAV sync retention readiness, and API usage export handoff readiness.
- API usage ledger list/export/stats OpenAPI operations now also pin
  `/admin/v1` and explicit admin-token/bearer auth alternatives, matching the
  runtime `adminAuth` route boundary for generated operator clients.
- API usage daily/monthly aggregate OpenAPI operations now use the same
  `/admin/v1` server pin and admin-token/bearer auth alternatives as their
  runtime admin-authenticated routes.
- API usage export batch list/create/detail/export OpenAPI operations now also
  pin `/admin/v1` and admin-token/bearer auth alternatives, matching their
  runtime admin-authenticated route boundary.
- API usage export artifact list/create/detail/write/download/verification
  OpenAPI operations now also pin `/admin/v1` and admin-token/bearer auth
  alternatives, matching their sensitive runtime `adminAuth` route boundary.
- API usage export manifest digest/signature OpenAPI operations now also pin
  `/admin/v1` and admin-token/bearer auth alternatives, matching their
  operator-only audit/export proof route boundary.
- Core queue stats, delivery route counters, and IMAP UID backfill OpenAPI
  operations now also pin `/admin/v1` and admin-token/bearer auth alternatives,
  matching their runtime operator diagnostics/repair boundary.
- Tenant, domain, and user administration OpenAPI operations now also pin
  `/admin/v1` and admin-token/bearer auth alternatives, matching their runtime
  organization identity, domain policy, DNS, quota, and user lifecycle boundary.
- Outbox event, audit log, Directory principal/alias/delegation/group
  membership, and SMTP backpressure OpenAPI operations now also pin
  `/admin/v1` and admin-token/bearer auth alternatives, matching their runtime
  operator forensics, identity, delegated-access, and flow-control boundary.
- Quota pressure, attachment upload cleanup, Drive upload session, Drive node,
  Drive usage, and Drive cleanup failure OpenAPI operations now also pin
  `/admin/v1` and admin-token/bearer auth alternatives, matching their runtime
  operator storage/Drive boundary across local, NFS, MinIO, and S3-compatible
  deployments.
- API usage ledger retention run and DAV sync retention run OpenAPI operations
  now also pin `/admin/v1` and admin-token/bearer auth alternatives, matching
  their runtime operator-only destructive/audited retention boundary.
- Quota reconciliation, delivery attempt, exhausted delivery attempt, and push
  notification attempt/statistics OpenAPI operations now also pin `/admin/v1`
  and admin-token/bearer auth alternatives, matching their runtime operational
  observability and provider outcome boundary.
- Suppression list, trusted relay, delivery route, DKIM key/DNS verification,
  and outbox retry OpenAPI operations now also pin `/admin/v1` and
  admin-token/bearer auth alternatives, matching their runtime outbound mail operations
  and domain signing boundary.
- OpenAPI contract tests now derive registered `/admin/v1` routes from
  `admin.go` and require every matching operation to pin `/admin/v1` plus
  admin-token/bearer auth, preventing future admin route additions from
  silently drifting to ambiguous generated-client base/auth contracts.
- Mail API list/search handlers now apply the documented default `limit=50`
  when `limit` is omitted or empty, with regression coverage for message lists,
  thread lists, thread-message lists, active search, and draft search to
  preserve OpenAPI/runtime pagination alignment for generated webmail and admin
  clients.
- IMAP read-only selected-state mutation handling now validates malformed
  `APPEND`, `STORE`, `MOVE`, `UID STORE`, `UID MOVE`, and `UID EXPUNGE`
  requests before returning read-only `NO` failures for syntactically valid
  mutations, including invalid UID/sequence sets, STORE modes/flags, APPEND
  options, and destination mailbox names. `APPEND` now canonicalizes the
  destination mailbox before the read-only check, so appends to the currently
  `EXAMINE`-selected mailbox do not reach backend mutation dispatch.
- IMAP mailbox mutation handling rejects `CREATE INBOX`, `DELETE INBOX`,
  `RENAME INBOX ...`, and `RENAME ... INBOX`, keeping the special INBOX
  namespace out of generic folder mutation paths. The special-name check is
  exact case-insensitive matching without trimming decoded mailbox names, so
  quoted names such as `" INBOX "` are preserved as ordinary mailbox names.
- IMAP authenticated `SELECT`/`EXAMINE` attempts now deselect the current
  mailbox before attempting the new selection, so failed selection attempts
  leave no stale selected mailbox for later selected-state commands.
- IMAP selected-state sequence-set commands now drain queued mailbox events
  before command dispatch, including UID-addressed subcommands such as
  `UID FETCH *`, keeping `*` and range resolution aligned with live
  `EXISTS`/`EXPUNGE`/`FLAGS` updates instead of the selection-time message
  count.
- IMAP `APPEND` now joins that selected-mailbox pre-command event drain path,
  so already queued FLAGS/EXISTS/EXPUNGE updates are emitted before APPEND
  mutation responses and are not delayed until a later `NOOP` or selected
  command.
- IMAP `FETCH` body-part selectors now enforce RFC-shaped `nz-number` syntax
  for MIME part paths and partial counts, rejecting leading-zero forms such as
  `BODY[01]`, `BODY[1.02.TEXT]`, and `BODY.PEEK[]<12.034>` at the parser
  boundary. MIME part, partial offset, and partial count values are capped to
  the unsigned 32-bit IMAP `number` range, and padded MIME path atoms are
  rejected instead of trimmed.
- IMAP `SEARCH HEADER` and `FETCH` `HEADER.FIELDS`/`HEADER.FIELDS.NOT`
  parsing now accepts visible RFC 5322-style custom field names containing
  `_`, `+`, or `.`, while preserving fail-fast rejection for empty,
  whitespace/control-bearing, colon-suffixed, non-ASCII, or IMAP
  atom-special-bearing field names.
  `FETCH` header-field section detection now also requires either an exact
  top-level body section or a valid numeric MIME part path before the
  `HEADER.FIELDS` marker, keeping malformed section prefixes from riding the
  supported header-subset path.
- IMAP command dispatch validates command and UID subcommand atoms before
  routing so malformed atom-special-bearing or non-ASCII command names do not
  fall through as unknown commands.
- IMAP command parsing returns tagged `BAD` for malformed command lines when
  the command tag is still syntactically recoverable, while malformed or
  missing tags continue to receive untagged `BAD`.
- IMAP command-line framing now requires RFC CRLF endings for ordinary
  commands, literal suffix lines, `AUTHENTICATE PLAIN` SASL continuations, and
  `IDLE` continuations, returning tagged `BAD` plus `BYE` for LF-only input
  before command handlers run.
- `UID` dispatch validates missing, malformed, unknown, or state-independent
  malformed subcommands before authentication or selected-mailbox state, while
  valid unauthenticated UID commands still return `NO authentication required`.
  Bare `UID` commands return `BAD UID requires subcommand` instead of looking
  like an unsupported implemented command family.
- IMAP `STATUS` and advertised RFC 5819 `LIST-STATUS` now reject empty
  parenthesized status data-item lists as explicit status-data-item errors,
  keeping malformed `STATUS inbox ()`, `STATUS inbox ( )`, and
  `LIST "" * RETURN (STATUS ())` requests distinct from unsupported,
  duplicate, or malformed-return status item handling. Duplicate status data
  items are now diagnosed separately from unknown/unsupported status items,
  including before authentication checks. LIST-STATUS now also rejects
  duplicated `STATUS` return options before mailbox lookup so later status
  return controls cannot overwrite earlier requested status data.
- IMAP `STATUS` status item lists now reject malformed inner whitespace such
  as `( UIDNEXT)` or `(UIDNEXT  RECENT)` instead of collapsing quoted/literal
  list values into valid status data items, while LIST-STATUS keeps its
  existing normalized return-option path regression-covered.
- IMAP RFC 5258 `LIST-EXTENDED` now rejects unparenthesized `RETURN` option
  lists even when no `STATUS` return option is present, keeping `CHILDREN`,
  `SPECIAL-USE`, and `SUBSCRIBED` return controls on the same parser boundary
  as LIST-STATUS.
- IMAP RFC 5258 `LIST-EXTENDED` selection option lists now consume the full
  parenthesized option list and reject whitespace-padded quoted or literal list
  values such as `" (SPECIAL-USE) "` instead of trimming them into valid
  selection controls.
- IMAP `LIST RETURN` option lists now reject whitespace-padded quoted or
  literal list values such as `RETURN " (CHILDREN) "` instead of trimming them
  into valid parenthesized return controls.
- IMAP `SEARCH`, `SORT`, and `THREAD` charset arguments now reject
  whitespace-padded quoted or literal atoms such as `CHARSET " UTF-8 "` instead
  of trimming them into supported charsets, and `THREAD` algorithm arguments
  now reject padded `ORDEREDSUBJECT` values on the same control-atom boundary.
- IMAP `SORT` and `UID SORT` criterion lists now use the same strict
  parenthesized atom-list shape as other IMAP control lists, rejecting leading,
  trailing, or nested list shapes such as `( DATE)`, `(DATE )`, and `((DATE))`
  before authentication or selected-mailbox state checks. Standard sort
  criterion atoms and `REVERSE` are interpreted case-insensitively, so client
  probes such as `SORT (reverse subject) UTF-8 ALL` remain compatible without
  relaxing the list grammar.
- IMAP `SORT` and `UID SORT` criterion lists now also reject exact quoted or
  command-literal parenthesized lists such as `"(DATE)"` and `{6+}\r\n(DATE)`
  before state checks, preserving raw RFC list framing for sort controls.
- IMAP `THREAD` and `UID THREAD` algorithm controls now reject exact quoted or
  command-literal `ORDEREDSUBJECT` values before state checks, preserving RFC
  atom framing for advertised THREAD algorithms.
- IMAP `SEARCH RETURN (...)` and `SORT`/`THREAD` `RETURN (SAVE)` option lists
  now reject whitespace-padded quoted or literal list values such as
  `RETURN " (COUNT) "` or `RETURN " (SAVE) "` instead of trimming them into
  valid ESEARCH/SEARCHRES controls.
- IMAP `SEARCH`, `UID SEARCH`, `SORT`, `UID SORT`, `THREAD`, and `UID THREAD`
  `RETURN` introducers and exact return option-lists now reject quoted or
  command-literal controls such as `"RETURN"` or `RETURN "(COUNT)"`, preserving
  RFC atom/list boundaries before authentication or selected-mailbox state.
- IMAP `FETCH` and `UID FETCH` data items now reject whitespace-padded quoted
  or literal values such as `" (FLAGS) "` or `" FLAGS "` instead of trimming
  them into valid fetch attributes.
- IMAP `FETCH` and `UID FETCH` now accept RFC 3501 `RFC822<offset.count>`
  partial full-message fetches, preserve the `RFC822<offset>` response atom,
  and mark messages seen like ordinary `RFC822` fetches.
- IMAP BODY/BODYSTRUCTURE rendering now validates MIME media type, subtype,
  parameter-list names, and transfer-encoding tokens against RFC 2045-style
  token boundaries, using conservative defaults for malformed
  tspecial/control-bearing source metadata instead of emitting invalid
  structure tokens, mixed fallback type/subtype pairs, duplicate canonical
  parameter keys, or empty parameter values to clients. Parameter values are
  also trimmed and bounded at UTF-8 boundaries before rendering so oversized
  filenames or boundaries cannot inflate fetch responses.
- Malformed MIME disposition tokens now render as `NIL` instead of falling back
  to `ATTACHMENT`, so IMAP BODYSTRUCTURE does not invent attachment semantics
  for invalid source metadata.
- BODYSTRUCTURE content ID and description nstrings are now trimmed and
  bounded at UTF-8 boundaries before quoting, preventing oversized source
  metadata from inflating IMAP fetch responses.
- Authenticated selected-state commands validate malformed `FETCH`, `STORE`,
  `COPY`, `MOVE`, `SEARCH`, `SORT`, and `THREAD` syntax before returning
  selected-mailbox state errors for valid commands.
- IMAP `SELECT` and `EXAMINE` now require optional `CONDSTORE` select
  parameters to use the RFC-shaped parenthesized select-param list, rejecting
  bare `CONDSTORE` and over-parenthesized `((CONDSTORE))` before authentication
  or backend mailbox lookup.
- IMAP `SELECT` and `EXAMINE` now also reject whitespace-padded quoted or
  literal `CONDSTORE` select-param lists such as `" (CONDSTORE) "` instead of
  trimming them into valid RFC 4551 select parameters.
- Selected-state action commands also validate malformed `FETCH`, `STORE`,
  `COPY`, and `MOVE` arity or modified UTF-7 destination mailbox names before
  authentication failures, while well-formed unauthenticated commands still
  return `NO authentication required`.
- Search-oriented selected-state commands validate malformed `SEARCH`, `SORT`,
  and `THREAD` argument shape, return options, and sort/thread argument lists
  before authentication failures, while well-formed unauthenticated commands
  still return `NO authentication required`.
- IMAP `THREAD` and `UID THREAD` now reject unsupported algorithms before
  authentication or selected-mailbox state checks, keeping advertised
  `THREAD=ORDEREDSUBJECT` capability behavior explicit for clients that probe
  `REFERENCES` or future algorithms.
- IMAP literal parsing regression coverage now locks malformed literal-marker
  placement, trailing atom data after literal payloads, and unused literal
  payloads to parser-level `BAD` responses before command handlers run.
- IMAP IDLE recovery now has regression coverage for unexpected command input:
  non-`DONE` lines such as `NOOP` or `DONE NOW` return a tagged `BAD` for the
  pending IDLE command, leave idle state, and keep the session usable.
- IMAP `IDLE` now requires an exact case-insensitive `DONE` continuation
  token, rejecting leading/trailing whitespace variants as malformed
  termination instead of silently ending the idle state.
- Selected-state no-argument commands validate extra arguments on `CHECK`,
  `IDLE`, `CLOSE`, `UNSELECT`, and `EXPUNGE` before returning authentication
  or selected-mailbox state errors.
- `STARTTLS` validates its no-argument syntax before TLS availability and
  authentication-state checks.
- `UID` dispatch validates subcommand arity and destination mailbox-name syntax
  before authentication or selected-mailbox state for `FETCH`, `STORE`,
  `EXPUNGE`, `COPY`, and `MOVE`.
- `LOGIN` and `AUTHENTICATE` validate malformed argument shape before
  plaintext `[PRIVACYREQUIRED]` responses on TLS-required listeners, while
  syntactically valid but unsupported SASL mechanisms return tagged `NO`
  responses so probing clients can fall back cleanly.
- `LOGIN` now treats an empty quoted password as syntactically valid and lets
  the backend authentication boundary reject it with
  `[AUTHENTICATIONFAILED]`, keeping protocol syntax errors distinct from
  ordinary credential failures.
- `AUTHENTICATE PLAIN` SASL-IR initial responses now validate malformed PLAIN
  payloads before plaintext privacy policy checks, preserving
  syntax-before-policy diagnostics without authenticating before TLS.
- `AUTHENTICATE PLAIN` continuation cancellation now requires an exact `*`
  token, rejecting whitespace-padded cancellation attempts as malformed SASL
  responses while keeping the session usable.
- `AUTHENTICATE PLAIN` SASL response tokens now reject leading or trailing
  whitespace around the base64 atom, including quoted SASL-IR values such as
  `" <base64> "`, while preserving spaces inside decoded credentials.
- SASL PLAIN decoding rejects oversized encoded and decoded responses before
  credential splitting or backend authentication, keeping `AUTHENTICATE PLAIN`
  continuation and `SASL-IR` literal paths bounded.
- Successful `LOGIN` and `AUTHENTICATE PLAIN` responses now include the
  authenticated `[CAPABILITY ...]` response code, keeping post-auth capability
  discovery explicit for RFC-shaped clients.
- Connection greetings now include a state-aware `[CAPABILITY ...]` response
  code: plaintext TLS-required sessions expose `STARTTLS`/`LOGINDISABLED`,
  while implicit TLS sessions expose immediate `SASL-IR`/`AUTH=PLAIN`.
- Mailbox management and subscription commands validate malformed `LIST`,
  `LSUB`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, and `UNSUBSCRIBE`
  argument shape or modified UTF-7 mailbox names before authentication
  failures, while well-formed unauthenticated commands still return
  `NO authentication required`. `LSUB` now explicitly rejects LIST-EXTENDED
  option probes such as `(SPECIAL-USE)` prefixes or `RETURN (...)` tails with
  an `LSUB`-specific tagged `BAD`, keeping subscribed-mailbox discovery
  separate from advertised extended `LIST` semantics. `CAPABILITY` now
  advertises RFC 5258 `LIST-EXTENDED` alongside RFC 5819 `LIST-STATUS`, so
  clients can legally send the extended `LIST` selection and return options
  already implemented by the gateway. `LIST-EXTENDED` now supports RFC 5258
  `SUBSCRIBED` selection and `RETURN (SUBSCRIBED)`, so clients can discover
  subscribed folders through standard `LIST` flows and receive `\Subscribed`
  attributes only when requested. `LIST` and `LSUB` also normalize leading
  hierarchy delimiters in mailbox patterns as root-absolute
  selectors before matching internal root-relative mailbox names. `LIST`
  reference names with a leading hierarchy delimiter are normalized the same
  way before joining relative patterns, keeping namespace/root-style list
  probes aligned with the server's root-relative mailbox store. The decoded
  pattern matcher is prepared once per `LIST`/`LSUB` command and reused across
  mailbox rows, keeping large folder-tree discovery allocation-aware.
- IMAP RFC 5258 `LIST-EXTENDED` now accepts parenthesized mailbox pattern
  lists such as `LIST "" ("INBOX" "Sent")`, applies `RETURN` options to the
  union of matching folders, de-duplicates overlapping pattern results, and
  preserves quoted mailbox patterns containing spaces such as
  `"Archive 2026"` inside the pattern list. Pattern-list members may also be
  printable ASCII IMAP command literals immediately after `(`, keeping literal
  mailbox names aligned with the same matcher/status path without normalizing
  control-bearing or raw non-ASCII literal bytes into different patterns.
  Embedded atom fragments such as `Archive{12}` remain malformed, so literal
  markers stay token-delimited instead of widening the IMAP atom grammar.
- IMAP UIDPLUS `COPYUID` generation now uses an explicit copy-result mapping
  from source UID to destination message summary across the gateway, service,
  and PostgreSQL repository boundary, so sparse UID requests and concurrent
  source disappearance cannot force the server to guess source/destination
  pairing from request order alone.
- IMAP `MOVE`/`UID MOVE` UIDPLUS response codes now derive source UID sets
  from returned move results rather than the requested UID slice, matching the
  explicit source/destination contract already present on `MoveMessageResult`.
- IMAP `MOVE`/`UID MOVE` now emit UIDPLUS `COPYUID` in an untagged `OK`
  before source `EXPUNGE` responses, preserving RFC 6851-compatible ordering
  for clients that need the UID map before sequence numbers are removed.
- IMAP `COPY`/`UID COPY` and `UID MOVE` now preserve quoted destination
  mailbox names containing spaces, escaped quotes, and command-literal mailbox
  strings through parser tokenization, mailbox lookup, and backend mutation
  requests, covering common desktop/mobile folder names such as
  `"Team Archive"` plus literal-framed folder text.
- IMAP UID set response rendering now compacts contiguous ascending runs into
  RFC sequence-set ranges, reducing bulk `COPYUID`, ESEARCH, and SEARCHRES
  response size without reordering non-contiguous UID lists.
- IMAP `COPY`/`UID COPY`/`MOVE`/`UID MOVE` now omit UIDPLUS `COPYUID` response
  codes when destination mailbox metadata reports `UIDNotSticky`, preserving
  RFC 4315 semantics for non-persistent UID stores.
- IMAP `APPEND` results can now mark UID metadata as non-sticky and suppress
  UIDPLUS `APPENDUID`, keeping append response codes aligned with the same
  RFC 4315 non-persistent UID boundary used by COPY/MOVE.
- IMAP `UID EXPUNGE` sparse and mixed UID-set behavior is regression-covered:
  missing UID members are ignored, existing unmarked messages remain active,
  and only existing `\Deleted` messages produce `EXPUNGE` responses.
- IMAP saved SEARCHRES state now updates against adjusted multi-`EXPUNGE`
  sequence numbers, so `$` references remain aligned after batch expunges.
- IMAP SEARCHRES `$` is now accepted as a bare `SEARCH` sequence-set
  criterion, with protocol coverage for `SEARCH $` and `UID SEARCH $ ...`
  reuse after `SEARCH RETURN (SAVE)`. Saved `$` criteria are also
  regression-covered through `SORT`, `UID SORT`, `THREAD`, and `UID THREAD`.
- IMAP SEARCHRES `$` reuse now requires an exact `$` atom for sequence-set and
  UID-set helpers, rejecting whitespace-padded quoted/literal values instead
  of normalizing them into saved-result references.
- IMAP `CLOSE` now clears saved SEARCHRES `$` state as part of selected-mailbox
  teardown, keeping saved results scoped to the same selection lifecycle as
  `SELECT`, `EXAMINE`, and `UNSELECT`.
- IMAP `DELETE` of the currently selected mailbox now clears saved SEARCHRES
  `$` state and closes the mailbox event subscription together with selected
  metadata, keeping mailbox-removal lifecycle behavior predictable.
- IMAP `RENAME` now resolves the source mailbox wire name to the backend's
  canonical mailbox ID before mutation dispatch, aligning mailbox-management
  commands with the canonical-ID boundaries used by operational commands.
- IMAP `ENABLE CONDSTORE` after a non-persistent-mod-sequence selection now
  persists selected `NOMODSEQ` state, so later `FETCH MODSEQ`,
  `CHANGEDSINCE`, `MODSEQ` search, and `UNCHANGEDSINCE` mutations stay behind
  the RFC 7162 persistent-mod-sequence guard.
- IMAP `SELECT`/`EXAMINE` now cancel a newly opened mailbox event subscription
  if response writing fails before the subscription is installed into
  connection state, keeping broken-client paths resource-safe.
- IMAP selected-mailbox `RENAME` now tracks a backend-returned canonical
  mailbox ID and resubscribes event delivery to that ID while preserving
  same-selection SEARCHRES state. It also refreshes selected
  `HIGHESTMODSEQ`/`NOMODSEQ` metadata from the backend-returned mailbox so
  CONDSTORE gates do not retain stale mod-sequence state after rename.
- IMAP mailbox event publishing is now guarded against concurrent subscription
  cancellation by delivering non-blocking updates under the broker lock,
  preventing send-on-closed-channel panics without letting slow subscribers
  stall publisher paths.
- Selected-mailbox discovery commands validate malformed `NAMESPACE`, `SELECT`,
  `EXAMINE`, and `STATUS` argument shape, CONDSTORE options, status item lists,
  or modified UTF-7 mailbox names before authentication failures, while
  well-formed unauthenticated commands still return `NO authentication
  required`.
- `APPEND` validates missing literals, malformed append options, and modified
  UTF-7 mailbox names before authentication failures, while well-formed
  unauthenticated appends still consume the RFC literal and return
  `NO authentication required` before backend storage.
- `ENABLE` validates missing capability arguments before authentication
  failures, while well-formed unauthenticated enable attempts still return
  `NO authentication required` without mutating session feature state.
- `ENABLE` also validates malformed capability atoms before authentication or
  session mutation, keeping RFC 5161 syntax errors separate from unsupported
  but well-formed capabilities.
- `ENABLE` preserves RFC 5161-compatible unknown capability handling:
  unsupported but syntactically valid capability names are ignored and can
  produce an empty `ENABLED` response when no requested capability is enabled.
  Duplicate `CONDSTORE` capability probes are regression-covered so the
  `ENABLED` response stays singular even when clients retry the same
  capability atom with different casing.
- Storage backend portability now has a shared contract test that exercises
  special but canonical object keys and the full object lifecycle across local
  storage and optional S3-compatible integration coverage; use it as the smoke
  matrix before local/NFS, MinIO, or AWS S3 backend flips.
- Storage profile smoke coverage now checks that the NFS YAML profile's
  `storage_root` and explicit `local` compatibility label survive both
  config-loader parsing and CLI `--config` handoff. MinIO and AWS S3 profile
  smoke coverage now also verifies region, bucket, prefix, and credential
  fields so config-only storage flips cannot lose required object-storage
  settings unnoticed. The CLI `--config` profile handoff path asserts the same
  fields before app startup.
- YAML config overlays accept `storage_root` as the file-level alias for the
  local/NFS object root, matching `GOGOMAIL_STORAGE_ROOT`; `mailstore_root`
  remains supported for backward compatibility.
- `LIST`/`LSUB` CHILDREN attributes infer immediate parents from nested
  `FullPath` values when backend rows do not carry `ParentID`, preserving
  `\HasChildren` metadata for deeper hierarchies such as `Projects/2026/Jan`.
- `APPEND`, `STORE`, and `UID STORE` flag-list parsing rejects unparenthesized
  or unbalanced flag lists instead of silently trimming stray parentheses.
- IMAP `APPEND` internaldate parsing enforces RFC 3501 fixed-width
  `date-day-fixed` syntax, accepting zero-padded or space-padded days while
  rejecting bare one-digit dates such as `"5-May-2026 ..."`. Date-month atoms
  are canonicalized ASCII-case-insensitively before parsing, preserving strict
  date shape while accepting common uppercase or lowercase client month
  literals.
- `STORE` and `UID STORE` honor selected-mailbox `[PERMANENTFLAGS]`, rejecting
  otherwise valid system flags when the mailbox did not advertise them as
  permanent before backend mutation dispatch. Empty add/remove flag lists stay
  no-ops, while empty replacement is rejected when no permanent flags are
  permitted.
- IMAP message sequence sets reject numbers above the selected mailbox size
  with tagged `BAD` responses, preserving RFC 3501 bounds behavior.
- IMAP quoted-string parsing rejects adjacent tokens after a closing quote and
  unsupported backslash escapes before authentication or backend work, keeping
  command tokenization aligned with RFC 3501 quoted-special handling.
- IMAP mailbox wire-name formatting preserves ordinary internal spacing while
  still collapsing control-character runs, preventing folder list/status
  responses from changing distinct user-visible mailbox names.
- IMAP UID `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` commands resolve
  `*` UID sequence ranges against selected-mailbox UIDs, so common client
  requests such as `UID FETCH 1:*` include the last visible UID without
  expanding through non-existent UID gaps.
- IMAP `SEARCH UID <sequence-set>` and `UID SEARCH UID <sequence-set>` resolve
  `*` UID ranges against the selected mailbox's visible UIDs, aligning
  search-key filtering with UID command range handling.
- IMAP command tag validation rejects `+` in tags before command routing,
  matching RFC 3501 tag grammar and avoiding ambiguity with continuation
  protocol markers.
- IMAP command tags now also reject quoted or literal-framed values such as
  `"a1" NOOP` or `{2}\r\na1 NOOP` as untagged malformed commands, preserving
  the raw atom-only tag boundary before command routing.
- IMAP `SEARCH`/`UID SEARCH` date criteria reject malformed date atoms that
  still contain quote characters after command parsing, so broken inputs such
  as `SINCE 05-May-2026"` are not silently normalized.
- IMAP `SEARCH`/`UID SEARCH` date criteria accept one-digit date-day atoms such
  as `SINCE 5-May-2026` while preserving malformed quote rejection, improving
  client compatibility without weakening syntax guardrails. Search date-month
  atoms are also canonicalized ASCII-case-insensitively before parsing.
- IMAP `SEARCH`/`UID SEARCH` date criteria now reject whitespace-padded date
  strings such as `SINCE " 05-May-2026 "` instead of trimming them into valid
  date atoms.
- IMAP `SEARCH` and `UID SEARCH` reject `CHARSET` prefixes that omit the
  required following search-key before authentication or selected-mailbox
  checks, keeping RFC 3501 search grammar errors distinct from state errors.
- IMAP `FETCH` and `UID FETCH` reject malformed fetch data-item syntax such as
  nested `((FLAGS))` before authentication or selected-mailbox checks, keeping
  RFC 3501 fetch grammar diagnostics ahead of state-machine errors.
- IMAP `FETCH` and `UID FETCH` reject unsupported data items before
  authentication or selected-mailbox checks instead of silently returning
  default attributes, while preserving supported `BODY`, `BODY.PEEK`,
  `RFC822.*`, `HEADER.FIELDS`, partial fetch, MIME section, macro, and
  `CHANGEDSINCE` forms.
- IMAP `STORE` and `UID STORE` reject malformed `UNCHANGEDSINCE`, store mode,
  and flag-list syntax before authentication or selected-mailbox checks,
  keeping RFC 3501/CONDSTORE mutation diagnostics ahead of state errors.
- IMAP `STORE`/`UID STORE` mode atoms and `UNCHANGEDSINCE` markers reject
  whitespace-padded quoted or literal values instead of trimming them into
  valid mutation controls.
- IMAP `STORE`/`UID STORE` flag-list values now reject whitespace-padded
  quoted or literal lists such as ` (\\Seen) ` while preserving exact `()`
  and parenthesized flag-list semantics.
- IMAP APPEND/STORE flag-list parsing now rejects malformed inner list
  whitespace such as `( \\Seen)`, `(\\Seen )`, `(\\Seen  \\Flagged)`, or
  tab-separated flag names instead of collapsing them into valid flags.
- IMAP APPEND/STORE flag-list parsing now also rejects duplicate canonical
  system flags such as `(\\Seen \\Seen)`, keeping flag-lists set-shaped before
  backend mutation or APPEND body handling.
- IMAP `SELECT` now canonicalizes mailbox `PermanentFlags` before rendering
  `FLAGS`/`PERMANENTFLAGS` and before selected-state STORE permission checks,
  so backend duplicate, alias, lower-case, or unknown flag metadata cannot
  leak into wire responses or permission state.
- IMAP selected-state commands reject malformed message sequence-set and UID
  set syntax, including signed values such as `+1`/`+7`, before authentication
  or selected-mailbox checks while preserving selected-mailbox bounds checks
  for execution time.
- IMAP selected-state sequence-set and UID-set command arguments now also
  reject quoted or literal-framed set tokens such as `"1"` or `{1}\r\n1` for
  `FETCH`, `STORE`, `COPY`, `MOVE`, `UID FETCH`, `UID STORE`, `UID COPY`,
  `UID MOVE`, and `UID EXPUNGE`, keeping RFC set atoms distinct from IMAP
  string values.
- IMAP `SEARCH` and `UID SEARCH` reject malformed search sequence-set and
  `UID` search-key set syntax before authentication or selected-mailbox checks,
  so signed values such as `SEARCH +1` and `UID SEARCH UID +7` fail as grammar
  errors rather than state errors.
- IMAP `SEARCH` top-level sequence-set criteria and `UID SEARCH UID`
  sequence-set values now reject quoted or literal-framed set strings such as
  `SEARCH "1"` or `UID SEARCH UID {1+}\r\n7`, preserving RFC set atoms while
  leaving string values available for text/header search keys.
- IMAP `SORT`, `UID SORT`, `THREAD`, and `UID THREAD` reuse the same
  syntax-only search-key validation before authentication or selected-mailbox
  checks, keeping malformed embedded search criteria consistent across the
  search/sort/thread command family.
- IMAP `SORT`, `UID SORT`, `THREAD`, and `UID THREAD` embedded search
  sequence-set criteria now also reject quoted or literal-framed set operands
  such as `SORT (DATE) UTF-8 "1"` or `THREAD ORDEREDSUBJECT UTF-8 {1+}\r\n1`.
- IMAP command tokenization rejects embedded quote characters inside unquoted
  atoms while preserving escaped quotes inside proper quoted strings, keeping
  RFC 3501 atom and quoted-string handling separate.
- IMAP command and `UID` subcommand words now must be raw atom tokens, so
  quoted or literal-framed command probes such as `"NOOP"`, `{4}\r\nNOOP`,
  `UID "COPY"`, or `UID {4}\r\nCOPY` fail as malformed commands instead of
  being dequoted into executable command names.
- IMAP parenthesized `SEARCH`/`UID SEARCH` groups reject empty `()` groups
  instead of treating them as match-all, while preserving valid `(ALL)` groups.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` numeric thresholds reject malformed
  values that still contain quote characters after command parsing, so broken
  inputs such as `MODSEQ 20"` are not silently normalized.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` entry types reject malformed atoms that
  still contain quote characters after command parsing, preventing broken
  `MODSEQ "/flags/\\Seen" all" 17` style inputs from being silently normalized.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` entry types now also reject
  whitespace-padded `ALL`, `PRIV`, or `SHARED` atoms instead of trimming them
  into valid RFC 7162 entry-type controls.
- IMAP RFC 2971 `ID` parameter-list parsing rejects unsupported quoted escapes
  and adjacent quoted tokens without whitespace, while preserving valid escaped
  quoted-special characters inside ID strings.
- IMAP RFC 2971 `ID` parameter-list parsing now rejects quote and backslash
  atom-special characters inside unquoted ID tokens, keeping the raw ID parser
  aligned with normal IMAP atom handling.
- IMAP RFC 2971 `ID` unquoted field/value tokens now reuse the common IMAP
  atom validator, so literal markers, response specials, wildcard specials,
  quoted specials, and controls are rejected consistently.
- IMAP RFC 2971 `ID` parameter-list parsing now accepts bounded synchronizing
  and `LITERAL+` string literals inside the parenthesized field/value list,
  while missing or unused literal payloads remain tagged `BAD` syntax errors.
- IMAP RFC 2971 `ID` now accepts the bare no-argument command form as an empty
  client parameter set, returning server identity without weakening `NIL` or
  parenthesized field/value-list validation.
- IMAP `SEARCH`/`UID SEARCH` `LARGER` and `SMALLER` size criteria require
  RFC 3501 `number` atoms, rejecting signed values such as `+20` and
  leading-zero values such as `020`, and values above the unsigned 32-bit IMAP
  `number` range, instead of silently treating them as valid sizes.
- IMAP `SEARCH`/`UID SEARCH`/`SORT`/`THREAD` numeric search operands now also
  reject quoted or literal-framed values for `LARGER`, `SMALLER`, and MODSEQ
  thresholds/entry types, while preserving quoted MODSEQ entry names as
  RFC string operands.
- IMAP `SEARCH`/`UID SEARCH`/`SORT`/`THREAD` charset and date controls now
  reject quoted values such as `CHARSET "UTF-8"` or `SINCE "05-May-2026"`,
  keeping command controls atom-only while leaving text/header search operands
  string-capable.
- IMAP `KEYWORD` and `UNKEYWORD` search operands now reject quoted
  flag-keyword values such as `KEYWORD "custom"`, preserving RFC atom
  semantics for flag names while text/body/header search operands remain
  string-capable.
- IMAP `STORE` and `UID STORE` mode controls now reject quoted or
  command-literal flag update mode tokens such as `"+FLAGS"` or literal
  `+FLAGS`, and quoted `UNCHANGEDSINCE` markers are rejected before mutation
  state checks.
- IMAP `FETCH` and `UID FETCH` data-item controls now reject exact quoted or
  command-literal data item atoms such as `"FLAGS"` or literal `FLAGS`, while
  preserving the existing diagnostics for whitespace-padded malformed fetch
  item lists.
- IMAP `ENABLE` capability operands now reject quoted or command-literal
  values such as `"CONDSTORE"` or literal `CONDSTORE`, keeping capability
  negotiation on the RFC atom boundary before authentication.
- IMAP `AUTHENTICATE` mechanism names and SASL-IR initial responses now
  reject quoted values such as `"PLAIN"` or quoted base64, keeping mechanism
  selection and initial-response parsing atom-only before privacy checks.
- IMAP parenthesized mutation/status controls now reject quoted or
  command-literal lists such as `STORE 1 +FLAGS "(\\Seen)"`,
  `APPEND inbox "(\\Seen)" {..}`, and `STATUS inbox "(MESSAGES)"`, while
  preserving quoted mailbox names, APPEND message literals, APPEND
  internaldate, LOGIN credentials, ID strings, and SEARCH text/header strings.
- IMAP `SELECT` and `EXAMINE` optional `CONDSTORE` select parameters now also
  reject quoted or command-literal `(CONDSTORE)` lists while preserving normal
  mailbox string/astring handling.
- IMAP `LIST` selection option-lists, `RETURN` introducers, and `RETURN`
  option-lists now reject quoted or command-literal controls such as
  `"(SPECIAL-USE)"`, `"RETURN"`, and `"(STATUS (MESSAGES))"` while preserving
  valid parenthesized mailbox pattern lists.
- IMAP `SEARCH`/`UID SEARCH` size and MODSEQ numeric criteria now reject
  whitespace-padded numeric strings such as `LARGER " 20 "` or
  `MODSEQ " 20 "` instead of trimming them into valid number atoms.
- IMAP mod-sequence numeric inputs require digit-only atoms across
  `SEARCH MODSEQ`, `FETCH CHANGEDSINCE`, and conditional `STORE`
  `UNCHANGEDSINCE`, rejecting signed values such as `+17`. Positive
  `mod-sequence-value` contexts reject zero, while `UNCHANGEDSINCE 0` remains
  a real zero-allowed conditional guard instead of being treated as no
  modifier.
- IMAP `FETCH`/`UID FETCH` `CHANGEDSINCE` and `STORE`/`UID STORE`
  `UNCHANGEDSINCE` modifier values now reject whitespace-padded numeric atoms
  instead of trimming them into valid CONDSTORE thresholds.
- IMAP UID and message sequence-set numbers require digit-only atoms, rejecting
  signed values such as `UID FETCH +7` and `FETCH +1` before command execution.
- IMAP UID and message sequence-set numbers now also reject leading-zero
  `nz-number` spellings such as `FETCH 01 FLAGS` or `UID FETCH 1:02 FLAGS`
  instead of normalizing them during range expansion.
- IMAP UID and message sequence-set syntax now rejects whitespace-padded
  quoted or literal set strings such as `SEARCH " 1 "` or
  `UID SEARCH UID " 7 "` instead of trimming them into valid set atoms.
- IMAP UID and message sequence-set range components now also reject embedded
  whitespace in quoted or literal set strings such as `"1: 2"`, `"1 :2"`, or
  `"1, 2"` instead of trimming range endpoints or comma-separated members
  during validation or expansion.
- IMAP UID and message sequence-set expansion accepts common client-scale
  ranges such as `1:1000` and `1:*` while still enforcing an explicit expansion
  cap, reducing false `BAD` responses during mailbox synchronization.
- IMAP UID set resolution intersects authenticated selected-mailbox UID ranges
  and comma-separated UID sets with visible message UIDs, so sparse requests
  such as `UID FETCH 1:999` and `UID FETCH 1,7,999` skip missing UIDs instead
  of failing the whole command.
- IMAP MIME body-part paths and partial body fetch windows require digit-only
  number atoms, rejecting signed forms such as `BODY[+1]` and
  `BODY[]<+12.34>`, and partial fetch counts must be non-zero as required by
  RFC 3501 `nz-number` grammar. MIME part, offset, and count values are capped
  to IMAP's unsigned 32-bit `number` range, and partial fetch tokens also
  reject trailing characters after the closing `>`. Padded MIME path atoms are
  rejected before section lookup.
- IMAP partial fetch offsets now reject leading-zero RFC `number` atoms such
  as `BODY.PEEK[]<00.34>` or `<012.34>` instead of normalizing them to valid
  offsets, while preserving the valid zero-offset form `<0.count>`.
- IMAP command literal size framing now follows RFC 3501 `number` grammar,
  preserving valid `{0}` literals while rejecting leading-zero forms such as
  `{00}`, `{001}`, and `{001+}`, plus signed or malformed forms such as
  `{+1}`, `{-1}`, and `{1++}`, with a tagged `BAD` framing response before
  reading literal bytes.
- IMAP `SEARCH`, `SORT`, and `THREAD` charset arguments reject malformed atoms
  that still contain quote characters or outer whitespace after command
  parsing, preventing broken values such as `UTF-8"` or `" UTF-8 "` from being
  silently normalized. Unsupported charsets now return the same RFC-shaped
  `[BADCHARSET (US-ASCII UTF-8)]` diagnostics before authentication or
  selected-mailbox checks, so probe clients can fall back deterministically.
- IMAP `THREAD` algorithm arguments reject malformed atoms that still contain
  quote characters or outer whitespace after command parsing, preventing broken
  values such as `ORDEREDSUBJECT"` or `" ORDEREDSUBJECT "` from being silently
  normalized.
- IMAP `SEARCH`/`UID SEARCH` text, body, and header string arguments reject
  malformed atoms that still contain quote characters after command parsing,
  preventing broken values such as `SUBJECT IMAP"` from being normalized.
- IMAP `SEARCH` text arguments preserve valid RFC quoted-special escaped
  quotes from proper quoted strings, so standards-shaped searches such as
  `SUBJECT "Project \"Q2\""` remain compatible while malformed atom quotes are
  rejected by command parsing.
- IMAP `SEARCH`/`UID SEARCH` `KEYWORD` and `UNKEYWORD` criteria reject
  malformed keyword atoms that still contain quote characters after command
  parsing, preventing broken values such as `KEYWORD custom"` from being
  silently normalized. They now use the common IMAP atom validator, rejecting
  system flags such as `\Seen` and response-special atoms such as `bad]flag`
  instead of treating them as RFC `flag-keyword` search criteria.
- IMAP command tokenization rejects dangling quote characters at the end of
  unquoted atoms, preventing broken commands such as `SUBJECT IMAP"` and
  `LIST "" INBOX"` from reaching command-specific normalization while
  preserving valid escaped quotes inside proper quoted strings.
- IMAP `FETCH`/`UID FETCH` `HEADER.FIELDS` and `HEADER.FIELDS.NOT` lists
  validate RFC-shaped header field names instead of trimming stray brackets or
  accepting IMAP atom-specials, rejecting malformed requests such as
  `HEADER.FIELDS ([Subject])`.
- IMAP `FETCH`/`UID FETCH` now accepts empty `HEADER.FIELDS ()` and
  `HEADER.FIELDS.NOT ()` lists. Empty include lists return the blank header
  terminator, while empty exclude lists return the full header block as RFC
  clients expect.
- IMAP `FETCH`/`UID FETCH` rejects whitespace-only, padded, or collapsed
  `HEADER.FIELDS`/`HEADER.FIELDS.NOT` lists such as `HEADER.FIELDS ( )`,
  keeping exact `()` compatibility without silently normalizing malformed
  field-list syntax.
- IMAP `FETCH`/`UID FETCH` now has regression coverage for partial-window
  empty top-level header-field-list requests such as `HEADER.FIELDS ()<0.1>`
  and `HEADER.FIELDS.NOT ()<0.10>`.
- IMAP `FETCH`/`UID FETCH` now applies those empty header-field-list semantics
  to `message/rfc822` MIME-part sections too, so forwarded-message probes such
  as `BODY[1.HEADER.FIELDS ()]` and `BODY[2.HEADER.FIELDS.NOT ()]` behave like
  top-level header subset fetches.
- Nested `message/rfc822` header-field partial fetches now have regression
  coverage for forwarded-message previews, including non-empty
  `HEADER.FIELDS`, empty `HEADER.FIELDS`, and empty `HEADER.FIELDS.NOT`
  windows on attached messages.
- IMAP `SEARCH HEADER` now rejects malformed header field names before
  authentication or selected-mailbox state, so broken criteria such as empty
  field names, `Bad Field`, or `Subject:` do not masquerade as empty searches.
  `HEADER.FIELDS` filtering no longer trims malformed stored field names such
  as `Subject : value` into valid matches.
- IMAP `SEARCH`, `UID SEARCH`, `SORT`, and `THREAD` embedded criteria now
  reject unknown/unsupported search-key atoms such as `X-GM-RAW` before
  authentication or selected-mailbox state, keeping capability probes distinct
  from valid stateful searches.
- IMAP `FETCH`/`UID FETCH` `CHANGEDSINCE` requires the RFC-shaped
  parenthesized modifier form and rejects bare or over-closed variants such as
  `FETCH 7 FLAGS CHANGEDSINCE 17`.
- IMAP `FETCH`/`UID FETCH` macros remain valid only as standalone macro
  arguments, rejecting malformed list usage such as `FETCH 1 (FAST)` or
  `UID FETCH 7 (FLAGS FAST)`.
- IMAP `STORE`/`UID STORE` `UNCHANGEDSINCE` requires the RFC-shaped
  parenthesized modifier form and rejects malformed over-closed values such as
  `(UNCHANGEDSINCE 27))`.
- IMAP `FETCH`/`UID FETCH` data items reject over-parenthesized tokens before
  item normalization, preventing malformed requests such as `FETCH 1
  ((FLAGS))` and `UID FETCH 7 BODY.PEEK[]))` from being repaired.
- Local filesystem storage remains the default and can be backed by local disk
  or NFS-style mounted storage. `GOGOMAIL_STORAGE_BACKEND=nfs` is now accepted
  as an explicit alias for the same local filesystem adapter, and runtime
  storage wiring treats `local` and `nfs` as bidirectional compatibility labels
  for Drive rows.
- Local/NFS storage configuration requires a non-empty bounded
  `GOGOMAIL_MAILSTORE_ROOT` without line breaks when
  `GOGOMAIL_STORAGE_BACKEND=local` or `nfs`, so broken filesystem roots fail
  during config validation instead of surfacing later as storage probe errors.
- Local/NFS-style storage writes stage data through unique temporary files in
  the destination directory before `rename`, avoiding fixed `.tmp` collisions
  while preserving atomic object replacement semantics.
- Local/NFS-style storage writes honor context cancellation during body copy,
  cleaning staged temp objects and avoiding partial object commits after a
  canceled request.
- Local/NFS and S3-compatible `Get`/`GetRange` readers now observe context
  cancellation after open/request dispatch, so canceled downloads and previews
  stop at the storage adapter boundary instead of continuing to stream bytes.
- Local/NFS `GetRange` now reports `io.ErrUnexpectedEOF` when a requested
  window extends beyond the available object bytes, matching the S3-compatible
  range-reader corruption signal instead of silently returning a short range.
- Local/NFS storage no longer treats filesystem symbolic links as storage
  objects or path parents: reads, range reads, metadata probes, deletes,
  copies, moves, writes, and prefix listings reject symlinked intermediate
  directories, while list pages still hide final-object symlinks. Mounted
  storage therefore cannot escape object-key semantics through host-specific
  link behavior. Local/NFS direct deletes also reject directories instead of
  treating filesystem folders as object keys.
- Local and S3-compatible storage writes reject nil `Put` bodies before
  filesystem or HTTP request work, keeping empty object creation explicit and
  adapter behavior consistent.
- Local/NFS-style storage deletes treat already-missing objects as success,
  aligning cleanup semantics with S3-compatible object deletion.
- S3-compatible storage requests reject canceled contexts before object-key
  validation, SigV4 signing, or HTTP dispatch, keeping cancellation behavior
  aligned with local/NFS storage and reducing wasted request work.
- S3-compatible `PUT`, failed `GET`, successful `GET` close, and `DELETE`
  responses drain a small bounded response-body window before close, improving
  HTTP connection reuse for normal S3/MinIO responses without allowing
  oversized bodies to stall cleanup.
- S3-compatible `PutObject`, full-object `GET`, `HEAD`/`Stat`, and
  `ListObjectsV2` now require exact `200 OK` responses, rejecting
  accepted/deferred writes, unexpected partial-content, or other non-OK 2xx
  statuses before callers can treat ambiguous provider responses as durable or
  complete backend-neutral results.
- S3-compatible missing-object reads now wrap `os.ErrNotExist` for `GET`,
  ranged `GET`, and `HEAD`/`Stat` `404 Not Found` responses, keeping
  backend-neutral missing-object checks consistent with local/NFS storage while
  preserving sanitized S3 status diagnostics.
- S3-compatible status-error diagnostics now format standard S3 `<Error>`
  XML bodies as bounded one-line `Code: Message` previews with request-id and
  host-id context, while preserving the sanitized plain-text fallback for
  non-XML compatible-provider errors. Truncated S3 XML error bodies still use
  the same streaming field preview path instead of leaking raw XML snippets,
  individual XML error fields are capped before formatting, and empty-field,
  duplicate-safe-field, nested-safe-field, or foreign-namespace safe-field
  standard S3 error XML suppresses raw XML fallback entirely.
- S3-compatible `ListObjectsV2` `200 OK` responses now fail closed when the
  body is a top-level standard S3 `<Error>` document, surfacing the same
  bounded embedded-error diagnostic instead of a generic invalid list control.
- S3-compatible `PutObject` and `DeleteObject` success responses now also
  reject top-level standard S3 `<Error>` bodies before reporting completed
  writes or cleanup, keeping compatible-provider throttling/auth/policy
  failures from crossing the shared storage contract as false success. They
  now also reject non-whitespace non-error success bodies instead of accepting
  arbitrary provider text or XML as an empty standard success response.
  `PutObject` also validates optional success `ETag` headers when present,
  rejecting blank, whitespace-padded, duplicate, or malformed identity metadata
  without requiring providers to send the header.
- Local/NFS and S3-compatible readiness probes read the verification object
  through a tight expected-size bound, preventing malformed or proxy-inflated
  probe responses from allocating unbounded memory during health checks.
- Local/NFS and S3-compatible readiness probes now also verify `Stat` metadata
  for the probe object, catching broken filesystem metadata or S3 `HEAD` paths
  before an instance reports ready.
- Local/NFS and S3-compatible readiness probes now also verify a short
  `GetRange` against the probe object, catching broken filesystem seek/range
  handling or S3 `Range` response compatibility before partial-read workflows
  report ready.
- The storage interface is backend-neutral (`Put`, `Get`, `GetRange`, `Stat`,
  `Copy`, `Move`, `List`, `Delete`) and object paths share strict canonical key
  validation before adapter use, including valid UTF-8 object paths, prefixes,
  and list cursors.
- S3-compatible `Stat` and `List` now bound and sanitize provider-returned
  `Content-Type`/ETag metadata before exposing `ObjectInfo`, dropping unsafe
  multiline, invalid UTF-8, or oversized metadata while preserving object
  identity and size for compatible providers.
- `GOGOMAIL_STORAGE_BACKEND=s3` can wire AWS S3-compatible object storage, and
  `GOGOMAIL_STORAGE_BACKEND=minio` uses the same adapter with path-style
  requests for local MinIO-style deployments. Both use endpoint, region, bucket,
  prefix, credential, and session-token settings.
- Production `s3` configs now require an explicit
  `GOGOMAIL_STORAGE_S3_ENDPOINT`, even for AWS regional endpoints, so operators
  can audit the object-store target directly while development/test configs can
  still derive AWS endpoints from region. In production, the `s3` endpoint must
  use HTTPS, preserving transport integrity for streaming SigV4
  `UNSIGNED-PAYLOAD` requests while leaving local HTTP MinIO development on the
  explicit `minio` backend.
- S3-compatible runtime wiring now supports private object-store TLS trust via
  `GOGOMAIL_STORAGE_S3_CA_CERT_FILE`, validates that the file contains a PEM
  certificate, and injects a dedicated TLS 1.2+ HTTP client into the existing
  storage adapter. `GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY=true` is accepted
  only outside production for local/self-signed compatibility testing.
- Drive runtime wiring now registers the configured S3-compatible store under
  both `s3` and `minio` labels, so rows created under local MinIO can still be
  served after an AWS S3-style backend flip and vice versa when object keys and
  bucket contents have been migrated.
- Drive runtime wiring can also opt into explicit legacy storage labels through
  `GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS`, giving operators a controlled
  migration bridge for local/NFS-to-S3-compatible Drive cutovers after object
  bytes are replicated while leaving unmapped legacy labels fail-closed.
- App-level storage option construction now has direct coverage for MinIO
  path-style pinning, ordinary S3 virtual-hosted defaults, and the explicit
  `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` override.
- S3-compatible bucket validation rejects IP-address-shaped names plus
  AWS-reserved bucket prefixes and suffixes before storage adapter construction,
  and requires bucket names to start and end with a letter or digit, keeping AWS
  and MinIO-style deployment failures early and explicit.
- S3-compatible `ListObjectsV2` responses now reject `IsTruncated=true` pages
  that omit a continuation token, preventing Drive/lifecycle cleanup scans from
  accepting a page that cannot be advanced safely.
- S3-compatible `ListObjectsV2` success bodies now must decode as
  `ListBucketResult` XML, so unexpected success XML cannot silently become an
  empty canonical object listing.
- S3-compatible `ListObjectsV2` key decoding no longer trims provider-returned
  object keys before prefix/object-path validation, preventing distinct
  whitespace-bearing keys from being silently normalized into canonical
  gogomail object paths.
- S3-compatible `ListObjectsV2` object-size validation now runs after canonical
  prefix mapping, so foreign bucket-prefix entries are skipped before their
  metadata can fail a valid gogomail object listing.
- S3-compatible `ListObjectsV2` request queries now use SigV4 canonical URI
  encoding instead of form-style query escaping, preserving spaces, literal
  `+`, `/`, `=`, and `@` characters in signed prefixes and opaque continuation
  tokens for AWS S3, MinIO, and stricter compatible providers.
- Shared storage list cursors reject leading/trailing whitespace and control
  characters instead of trimming opaque provider tokens, keeping local/NFS and
  S3-compatible pagination identity exact for Drive, lifecycle, and
  reconciliation scans.
- S3-compatible `ListObjectsV2` pages reject provider responses that return
  more matching objects than the requested bounded page size, keeping S3,
  MinIO, and local/NFS pagination under the same storage contract.
- S3-compatible `ListObjectsV2` pages also filter mapped object paths by the
  caller's requested logical prefix after removing the configured storage
  prefix, preserving local/NFS prefix isolation semantics during Drive,
  lifecycle, and reconciliation cleanup scans.
- Shared storage object keys and prefixes now reject encoded separators such
  as `%2F` and `%5C` before filesystem path construction, request signing, or
  list-page exposure, so object names cannot depend on provider-specific
  single- or double-decoding behavior. The shared validator also rejects
  double-encoded separator forms such as `%252F` and `%255C`, and S3 now uses
  that same validator instead of a duplicate adapter-local check.
- S3-compatible endpoint validation rejects userinfo, query strings, fragments,
  non-HTTP schemes, CR/LF-bearing targets, and non-canonical base paths before
  storage adapter construction. Endpoint base paths also reject encoded path
  separators such as `%2F` and `%5C`, keeping SigV4 signing and object
  addressing deterministic.
- S3-compatible request construction automatically switches dotted bucket names
  on HTTPS endpoints to path-style addressing, avoiding AWS S3 virtual-hosted
  TLS wildcard certificate mismatches without changing ordinary bucket defaults;
  this behavior is regression-covered at the request URL level.
- S3-compatible request construction also switches localhost and IP-address
  endpoints to path-style addressing, avoiding `bucket.localhost` or
  `bucket.127.0.0.1` drift for local MinIO and other local compatible stores
  even when the generic `s3` backend is used; this behavior is also
  regression-covered for localhost, IPv4, and IPv6 endpoints.
- S3-compatible object key escaping preserves literal `+` characters as `%2B`
  in segment-escaped request paths, keeping object identity and SigV4 canonical
  request paths aligned for AWS S3, MinIO, and strict compatible providers.
- S3-compatible endpoint base paths are segment-escaped with the same literal
  `+` preservation, keeping reverse-proxy or base-path deployments aligned with
  SigV4 canonical request paths.
- S3-compatible uploads set a deterministic `Content-Length` for seekable PUT
  bodies without buffering the object in memory, improving compatibility for
  file-backed mail and attachment writes while keeping hot paths streaming-first.
- S3-compatible deletes accept completed `200 OK`/`204 No Content` responses
  plus idempotent `404 Not Found`, while rejecting accepted/deferred or other
  ambiguous non-OK 2xx statuses so cleanup workers do not mark uncertain
  deletes as complete.
- Local/NFS and S3-compatible storage expose a shared object `Move` contract
  for future Drive/file relocation workflows. Local/NFS uses efficient
  filesystem rename semantics and falls back to copy-delete only on
  cross-device `EXDEV`; S3-compatible storage uses signed server-side copy
  followed by source delete. Post-copy source delete failures now return a
  structured cleanup error with source and destination paths, so callers can
  safely distinguish recoverable duplicate cleanup from pre-copy move failure.
- S3-compatible `Copy` now requires exact `200 OK` responses with bounded
  `CopyObjectResult` bodies and rejects empty bodies, unexpected XML, and
  embedded `<Error>` XML inside `200 OK` responses, keeping AWS S3/compatible
  copy failures from being accepted as successful Drive or lifecycle object
  duplication.
- Shared storage exposes a bounded `DeletePrefix` helper over the existing
  `List` and idempotent `Delete` contracts, giving future Drive folder
  deletion, attachment lifecycle, and reconciliation jobs a cursor-driven
  cleanup path without relying on provider-specific recursive delete behavior.
  If a listing source returns an unsafe object path, cleanup now reports a
  structured partial-progress error that separates listing corruption from
  ordinary delete failures. If a listing source returns a safe but
  out-of-scope sibling object, cleanup now reports a structured
  partial-progress error before deleting it.
- S3-compatible secret access keys and session tokens reject spaces, tabs, and
  line breaks during config validation and adapter construction, making copied
  env/config credential mistakes fail fast before runtime S3 authentication
  errors.
- S3-compatible access key IDs reject spaces, tabs, and line breaks during
  config validation and adapter construction, preventing copied credential
  mistakes from being silently trimmed before SigV4 signing.
- S3-compatible access key IDs, secret access keys, and session tokens also
  reject oversized direct adapter inputs using the same bounds as startup
  config validation, keeping SigV4 request construction bounded.
- `docs/storage-backends.md` documents local/NFS, MinIO, and AWS S3-style
  configuration, including the `GOGOMAIL_STORAGE_ROOT` compatibility alias for
  `GOGOMAIL_MAILSTORE_ROOT`, and the development compose stack includes
  `minio-init` to create the default `gogomail` bucket for local
  S3-compatible runs.
- Optional S3-compatible integration coverage now accepts
  `GOGOMAIL_TEST_S3_CA_CERT_FILE` and
  `GOGOMAIL_TEST_S3_INSECURE_SKIP_VERIFY`, letting operators smoke-test
  private CA or local self-signed MinIO/S3 endpoints through the same injected
  HTTP client path used by runtime S3 TLS configuration.
- Runtime startup now accepts `--config=<path>` for a flat YAML overlay on top
  of existing defaults/env values. The parser rejects unsupported keys and
  typed-value mismatches, and `configs/config.example.yaml` includes the
  storage backend/S3 knobs needed to switch local/NFS/MinIO/AWS S3 profiles by
  editing config rather than code. Validated profile overlays now also exist at
  `configs/storage.local.yaml`, `configs/storage.nfs.yaml`,
  `configs/storage.minio.yaml`, and `configs/storage.s3.yaml`, with loader
  and CLI handoff tests proving each profile parses and validates before
  release.
- The `gogomail` CLI startup path now has focused regression coverage for
  `--config`: valid YAML reaches the injected app runtime, while invalid YAML
  config or mode input returns code 2 before components start.

Next:

- Run optional integration coverage against MinIO or another S3-compatible test
  endpoint by setting `GOGOMAIL_TEST_S3_ENDPOINT`,
  `GOGOMAIL_TEST_S3_BUCKET`, `GOGOMAIL_TEST_S3_ACCESS_KEY_ID`, and
  `GOGOMAIL_TEST_S3_SECRET_ACCESS_KEY`.

### 1. Hierarchical quota ledger

Current state:

- Mailbox quota is enforced on selected mail write/delete paths.
- Company/domain/user quota read and update APIs exist.
- Mail storage growth/delete paths atomically update company, domain, and user
  quota ledgers in one transaction.
- Attachment upload metadata creation and stale upload cleanup also reserve and
  release bytes through the same company/domain/user quota ledger.
- Admin quota usage/detail views expose remaining capacity, child allocation,
  allocatable capacity, and over-allocation indicators.
- Admin API exposes a read-only quota reconciliation report comparing ledger
  counters with active message rows and reserved/stored attachment rows.
- Admin API can apply operator-controlled quota reconciliation corrections with
  transaction-scoped advisory locking, affected quota-row locks, and bounded
  audit-log detail for dry-run/applied correction attempts.
- Admin API exposes bounded audit-log list/detail reads so quota correction,
  domain onboarding, mail receive, and delivery-status audit rows are
  inspectable through the operator API.
- Domain DNS check and quota correction audit records now reuse the shared
  hash-chain writer, keeping operator-visible audit rows tamper-evident.
- Admin API can run a bounded audit-log integrity check over recent rows,
  reporting hash and in-window prev-hash breaks for operator triage.
- User quota source is tracked as `default|custom`.
- Domain quota updates can apply a new default user quota to default-following
  users while preserving custom overrides.
- ADR 0003 defines company → domain → user unified storage pool semantics.
- ADR 0009 defines the Drive metadata boundary: Drive nodes are PostgreSQL
  metadata scoped by company/domain/user, object bytes stay behind the shared
  storage interface, and future Drive file writes must consume the same unified
  user quota ledger as mailbox and attachments.
- The `drive_nodes` migration establishes folder/file metadata, active sibling
  uniqueness, storage object references for files, and active/trashed/deleted
  lifecycle state without starting frontend implementation.
- `internal/drive` validates Drive node names, types, and statuses before
  future repository/API code can persist path-bearing, control-character, or
  unsupported lifecycle values.
- `internal/drive.Repository.CreateFolder` can create active folder nodes for
  active users, derive company/domain scope from the user row, validate active
  parent folders, and rely on the `drive_nodes` active sibling uniqueness
  constraint before any Drive HTTP API is exposed. Folder creation SQL uses
  only the bound request parameters, keeping the production folder-create path
  aligned with the HTTP contract.
- `internal/drive.Repository.CreateFileFromObject` validates file metadata,
  verifies the referenced object through the shared storage `Stat` contract,
  and increments the company/domain/user quota ledger in the same transaction
  as the `drive_nodes` file insert.
- `internal/drive.Repository.ListNodes` can read bounded active/trashed/deleted
  folder contents with folder-first stable ordering, preparing Drive list views
  before an HTTP API is exposed.
- `internal/drive.Repository.TrashNode` can mark an active file/folder and its
  active descendants as trashed in one transaction, preserving object bytes and
  quota usage for future restore or delayed permanent deletion.
- `internal/drive.Repository.RestoreNode` can mark a trashed file/folder and
  its trashed descendants active again in one transaction, clearing `trashed_at`
  while keeping active sibling name conflicts protected by the database
  uniqueness constraint.
- `internal/drive.Repository.PermanentDeleteNode` can mark a trashed
  file/folder and its trashed descendants deleted, release deleted file bytes
  from the company/domain/user quota ledger, and return storage object
  references for later backend cleanup.
- `internal/drive.CleanupDeletedObjects` consumes those object references,
  validates backend/path safety, de-duplicates repeats, honors cancellation,
  and deletes objects through the configured storage stores with
  progress-preserving errors. Partial cleanup failures now expose the failed
  object and every not-yet-attempted object so committed permanent-delete
  metadata cannot leave trailing storage objects outside the retry queue.
- `internal/drive.Service.PermanentDeleteNode` now composes repository
  permanent-delete with backend object cleanup and returns cleanup progress
  alongside the committed metadata/quota result.
- Drive object path builders now standardize staged uploads, committed node
  objects, and user cleanup prefixes under `drive/users/{user_id}/...`, with
  path-segment-safe ID checks before storage paths are emitted.
- Drive object cleanup failures now have a PostgreSQL retry record boundary:
  structured cleanup errors can be recorded with user/node/object context for
  every object not proven deleted after a committed permanent delete, pending
  failures are de-duplicated per backend/path, repeated failures increment
  attempts, object paths must stay under the owning user's
  `drive/users/{user_id}/...` prefix, and error text is one-line/UTF-8 bounded.
- Drive cleanup-failure records now have bounded repository list and resolve
  methods with status/user filters, oldest-first pending ordering, limit caps,
  and pending-only resolution for worker/admin use.
- `internal/drive.Service.RetryObjectCleanupFailures` can process bounded
  pending cleanup records, delete referenced objects through configured stores,
  resolve successful records, and re-record failed attempts with fresh bounded
  diagnostics.
- `drive-cleanup-worker` can now run the Drive cleanup retry service on a
  validated interval or in run-once mode, using the configured local/NFS,
  MinIO, or S3-compatible object store.
- Mail API now exposes first Drive HTTP routes for bounded node listing, folder
  creation, single-node metadata reads, trash, restore, and permanent delete,
  with OpenAPI response envelopes and the existing user auth/fallback path.
- Mail API now exposes `POST /api/v1/drive/files/finalize` for converting a
  staged object into quota-accounted Drive file metadata through the shared
  storage `Stat` contract.
- Mail API now exposes `PUT /api/v1/drive/files/staged/{upload_id}/body` for
  bounded direct staged object uploads, returning canonical storage path, size,
  and SHA-256 for the finalize request.
- Mail API now exposes `PATCH /api/v1/drive/nodes/{id}/name` for validated
  active file/folder renames using the Drive normalized-name rules.
- Mail API now exposes `PATCH /api/v1/drive/nodes/{id}/parent` for moving
  active Drive files/folders into another folder or back to root with cycle
  prevention.
- `drive_upload_sessions` now defines the database and validation boundary for
  resumable Drive uploads, including declared/received sizes, lifecycle status,
  storage metadata, and expiration indexes.
- `internal/drive.Repository.CreateUploadSession` now records pending Drive
  upload sessions for active users and optional active parent folders.
- Mail API now exposes `POST /api/v1/drive/upload-sessions` for creating
  pending Drive upload sessions with declared size, storage backend, and
  optional RFC3339 expiration.
- Mail API now exposes `GET /api/v1/drive/upload-sessions/{id}` for upload
  session status refresh and retry-state hydration.
- Mail API now exposes `DELETE /api/v1/drive/upload-sessions/{id}` for
  explicit cancelation of pending/uploading/failed Drive upload sessions.
- `internal/drive.Service.StoreUploadSessionBody` now streams retry bodies to
  distinct session object paths, verifies declared size and optional checksum,
  records storage metadata, and cleans failed/superseded objects best-effort.
- Mail API now exposes `PUT /api/v1/drive/upload-sessions/{id}/body` for
  retry-safe full-body upload-session storage with optional SHA-256 checking.
- `internal/drive.Repository.FinalizeUploadSession` now commits uploaded
  session bodies into quota-accounted Drive file metadata and marks the session
  finalized in one transaction.
- Mail API now exposes `POST /api/v1/drive/upload-sessions/{id}/finalize`,
  completing the create/read/cancel/body/finalize Drive upload-session API
  flow for full-body uploads.
- Webmail capabilities now advertise Drive node operations, upload-session
  create/read/cancel/body/finalize support, checksum preconditions, and Drive
  upload size/TTL limits for production client bootstrap.
- `internal/drive.Repository.ExpireUploadSessions` now marks stale
  pending/uploading/failed Drive upload sessions expired in bounded batches,
  and the Drive service deletes stored session bodies from the configured
  backend after metadata expiry.
- `drive-cleanup-worker` now expires stale Drive upload sessions on each tick
  before retrying pending permanent-delete object cleanup failures.
- Mail API now exposes `GET /api/v1/drive/upload-sessions` with status and
  limit filters, giving future Drive upload managers a reconnect/recovery
  surface.
- Admin API now exposes `GET /admin/v1/drive-upload-sessions` with required
  user scope plus status/limit filters, and admin capabilities advertise Drive
  upload-session inspection.
- Drive upload session body storage now accepts `Content-Range` headers for
  complete body uploads. The `PUT /api/v1/drive/upload-sessions/{id}/body`
  endpoint parses and validates RFC 7233 `Content-Range` headers, accepting
  both `bytes */<size>` asterisk form and `bytes 0-<size-1>/<size>` explicit
  range form when the range matches the session's declared size. Malformed
  Content-Range headers or mismatched sizes return HTTP 400 with descriptive
  errors. Comprehensive unit tests cover parsing edge cases and validation
  scenarios.
- Admin API now exposes `GET /admin/v1/drive-nodes` with required user scope
  plus parent/status/name/limit filters, giving operator consoles a bounded
  Drive inventory view without reusing user-facing auth paths.
- Drive node listing now supports a bounded `q` name filter on both Mail and
  Admin API list surfaces, with case-insensitive normalization and literal
  wildcard handling.
- Admin API now exposes `GET /admin/v1/drive-nodes/{id}` with required user
  scope and lifecycle status filtering for single-node metadata inspection.
- Admin API now exposes `GET /admin/v1/drive-usage` with required user scope
  for quota, node-count, byte-count, and pending upload-session dashboard
  summaries.
- Mail API now exposes `GET /api/v1/drive/usage`, and webmail capabilities
  advertise `usage_summary`, so production Drive panels can show per-user quota
  and storage summaries without admin routes.
- Mail API now exposes `GET /api/v1/drive/nodes/{id}/download`, streaming
  active Drive file bytes through the configured storage backend with safe
  download headers, and webmail capabilities advertise `node_download`.
- Mail API now exposes `HEAD /api/v1/drive/nodes/{id}/download` for metadata
  and object-existence checks without transferring Drive file bytes.
- Mail API Drive downloads now accept one satisfiable `Range: bytes=...`
  request and return `206 Partial Content` through the shared local/NFS and
  S3-compatible `GetRange` storage primitive; webmail capabilities advertise
  `node_range_download`.
- Mail API Drive download, range-download, and download-header responses now
  expose sanitized `X-Gogomail-Drive-SHA256` when a node has a recorded
  whole-object digest.
- Admin API now exposes `POST /admin/v1/drive-upload-cleanup/candidates` for
  stale Drive upload-session cleanup counts and bounded candidate previews.
- Admin API now exposes `POST /admin/v1/drive-upload-cleanup/runs` for
  explicit one-shot stale Drive upload-session expiry outside the worker loop.
- Admin API now exposes `GET /admin/v1/drive-cleanup-failures` with user,
  status, and limit filters for operator cleanup-drift inspection.
- Admin API now exposes `POST /admin/v1/drive-cleanup-failures/{id}/resolve`
  for audited operator closure after external cleanup verification.
- Admin API now exposes `POST /admin/v1/drive-cleanup-failures/retry-runs`
  for audited one-shot retry of pending Drive object cleanup failures, with
  scanned/deleted/resolved/failed counts suitable for an operator console.
- `attachment_share_links` now defines the database and repository boundary for
  public large-attachment sharing, including hashed token verification,
  download-only permissions, and expiry tracking.
- Attachment share-link creation and resolution reuse the shared Mail and
  Drive quota ledger by preserving the underlying attachment ownership and
  status, ensuring that publicly shared large attachments are accounted for in
  the company/domain/user storage pool.
- A standalone `remote-signer` HTTP service now exists in `cmd/remote-signer`
  to satisfy production export-manifest signing requirements without coupling
  gogomail to a vendor KMS SDK. The signer runs as an isolated microservice,
  loads a base64-encoded Ed25519 private key from its environment, and exposes
  a bounded `/v1/sign` endpoint matching the `remote-ed25519` payload
  expectations.

Next:

- No immediate tasks in this section.

### 2. Message threading and search

Current state:

- Messages store `thread_id`, `in_reply_to`, `rfc_message_id`.
- Thread aggregation APIs exist for `GET /api/v1/threads` and
  `GET /api/v1/threads/{id}/messages`.
- New inbound and reply/forward outbound rows inherit thread IDs from local
  `References`/`In-Reply-To`/source messages.
- Reply composition writes RFC thread headers into outgoing `.eml`.
- Mail API exposes `GET /api/v1/search` backed by a small-deployment Postgres
  FTS index over active-message metadata.
- Received-message body indexing has an asynchronous boundary:
  `search-index-worker` consumes `mail.stored`, reads stored `.eml`, extracts
  bounded plain text through `internal/message`, and upserts
  `message_search_documents`.
- Postgres search includes indexed received body text without changing the
  existing search response envelope.
- Search clients can opt into relevance ordering, rank scores, and bounded
  headline snippets with `sort=relevance`, `include_rank=true`, and
  `include_highlights=true`; date ordering remains the default.
- `internal/searchindex` has an OpenSearch writer adapter behind the same
  indexing interface, and `search-index-worker` can select it with
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`.
- The OpenSearch writer can bootstrap a strict message index mapping for future
  query adapter work.
- `search-index-worker` can bootstrap that mapping on startup with
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP=true`.
- OpenSearch query-side groundwork can return ranked message IDs scoped to a
  user.
- `maildb` can hydrate ordered message ID search hits into active
  `MessageSummary` rows.
- `mailservice` can compose OpenSearch relevance ID hits with Postgres summary
  hydration when relevance sorting is requested.
- Mail API app wiring can inject the OpenSearch search source when
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`.
- OpenSearch indexed documents include parsed sender and attachment presence
  fields for search-filter parity.
- OpenSearch relevance search can apply folder, from, subject, and attachment
  filters before Postgres metadata hydration.
- OpenSearch relevance search can return subject/from/body highlights in the
  existing Mail API `search_highlights` shape.
- Optional OpenSearch integration coverage can validate bootstrap, indexing,
  and folder-aware relevance search against a real backend when
  `GOGOMAIL_TEST_OPENSEARCH_URL` is set.
- OpenSearch sender/subject filters use lower-cased keyword fields for
  Postgres-like case-insensitive filtering.
- OpenSearch highlight fragments are bounded before they are mapped into Mail
  API responses, and duplicate external hit IDs are deduplicated before
  Postgres summary hydration.
- Search index worker startup logs include non-secret backend diagnostics, and
  OpenSearch calls have an explicit configurable timeout.
- Postgres and OpenSearch relevance queries now share metadata-first tuning:
  subject and sender matches are weighted above indexed body text, with
  regression tests guarding both backend query shapes.
- Draft rows remain out of `GET /api/v1/search`; draft lookup now has a
  separate compose-focused `GET /api/v1/drafts/search` API over active draft
  subject, sender, recipients, body text, and attachment state, ordered by
  latest draft update with opaque cursor pagination. This keeps active-message
  Postgres/OpenSearch relevance semantics aligned while giving compose UIs a
  bounded search path.
- `GET /api/v1/search` now supports opaque cursor-based pagination for
  date-sorted results, returning `has_more`, `next_cursor`, and `limit` in the
  response envelope consistent with other paginated endpoints. Cursor
  pagination is rejected with HTTP 400 for `sort=relevance` since relevance
  results are unordered. OpenAPI spec and contract tests are updated.

Next:

- Add saved-search style draft filters only if compose UX needs them.

### 3. IMAP gateway planning

Current state:

- A bounded IMAP protocol server exists for the first RFC-shaped handshake,
  authentication, mailbox state, metadata fetch, body fetch, and flag-store
  commands.
- Message, folder, and flag models are IMAP-compatible by design.
- `internal/imapgw` defines native gateway DTOs, backend interfaces, mailbox
  helpers, RFC-shaped flag mapping, and a bounded TCP server shell over the
  service-backed store/session adapter.
- `imap_mailbox_state` and `imap_message_uid` migrations define durable
  UIDVALIDITY, UIDNEXT, mailbox MODSEQ, message UID, and message MODSEQ storage.
- `maildb` can ensure mailbox UID state and assign stable mailbox-local message
  UIDs transactionally.
- `maildb` can list/get folders as `internal/imapgw.Mailbox` DTOs, list mailbox
  messages as `internal/imapgw.MessageSummary` DTOs, and resolve UID-addressed
  messages to stored raw body paths.
- `mailservice` can open UID-addressed IMAP messages as raw `io.ReadCloser`
  bodies for future IMAP FETCH handling.
- `mailservice` can delegate IMAP STORE flag mutations to `maildb`, where
  `\Seen`, `\Flagged`, and `\Answered` map to persisted JSON flags and MODSEQ
  advances only for actual changes.
- `maildb` can backfill missing mailbox-local UIDs for active messages in
  bounded, stable-order batches.
- `event-worker` handles committed `mail.stored` events with an IMAP UID
  assignment handler, so newly received active messages become UID-visible
  asynchronously without adding IMAP work to the SMTP hot path.
- The IMAP `mail.stored` notification handler can publish UID-bearing
  `exists` mailbox events after UID assignment when a process-local mailbox
  event publisher is wired.
- Mail API move/delete paths remove stale IMAP UID rows transactionally so moved
  messages can receive fresh mailbox-local UIDs later.
- Optional PostgreSQL integration tests cover IMAP UID backfill and move
  invalidation when a test database URL is configured.
- `internal/imapgw` includes an in-memory mailbox event broker that live IDLE
  sessions and NOOP polling can subscribe to without blocking write paths.
- `mailservice.StoreIMAPFlags` can publish mailbox `flags` events through the
  broker boundary after repository flag mutations succeed.
- Mail API single and bulk flag mutations can publish mailbox `flags` events
  for messages that already have IMAP UID rows.
- Mail API single and bulk move mutations can publish mailbox `expunge` events
  for previously UID-visible source messages.
- Mail API single and bulk delete mutations can publish mailbox `expunge`
  events for previously UID-visible messages.
- `mailservice` exposes IMAP mailbox/message listing and event subscription
  methods for the protocol listener.
- Admin API exposes bounded IMAP UID backfill for future operator/bootstrap
  modes without enabling an IMAP protocol listener.
- IMAP mailbox event publication is best-effort after successful mutations, so
  IDLE/NOOP fan-out cannot make committed mail writes appear failed.
- Mail API move/delete expunge notifications carry mailbox sequence numbers
  from IMAP UID lookup, allowing selected `NOOP`/`IDLE` clients to receive
  renderable untagged `EXPUNGE` updates.
- `mailservice.IMAPStoreAdapter` satisfies `imapgw.Store` for protocol listener
  wiring through the service boundary.
- `mailservice.IMAPStoreAdapter` also satisfies `imapgw.MailboxSessionStore`
  for SELECT-style mailbox state, service-backed COPY/MOVE/EXPUNGE, and
  mailbox-event subscription.
- `gogomail --mode=imap` is now a separate gateway that opens the
  service-backed IMAP store adapter, wires a process-local mailbox event broker
  for live IDLE sessions, and serves the configured TCP protocol listener.
- `GOGOMAIL_IMAP_ADDR` is loaded and validated as required TCP listener
  metadata for the protocol listener.
- `GOGOMAIL_IMAP_TLS_CERT_FILE`, `GOGOMAIL_IMAP_TLS_KEY_FILE`, and
  `GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH` are loaded and validated so production
  IMAP auth cannot be enabled with cleartext credential policy.
- `GOGOMAIL_IMAP_MAX_CONNECTIONS` is now loaded from env/YAML and validated as
  a non-negative optional listener cap. Positive values bound concurrent
  `ServeConn` sessions and reject excess clients with an initial IMAP
  `BYE [ALERT]` instead of allowing unbounded connection goroutines.
- `GOGOMAIL_SMTP_MAX_CONNECTIONS` and
  `GOGOMAIL_SUBMISSION_MAX_CONNECTIONS` are now loaded from env/YAML and
  validated as non-negative optional listener caps. Positive values bound
  concurrent SMTP session goroutines and reject overflow clients with a
  transient `421 4.3.2` response before close.
- IMAP runtime TLS helper groundwork can load IMAP-specific certificate/key
  files with TLS 1.2 minimum and derive the server name from the IMAP listener
  host before falling back to `GOGOMAIL_SMTP_DOMAIN`.
- IMAP required mailbox-name commands now reject decoded empty targets before
  backend lookup or mutation while preserving empty `LIST`/`LSUB`
  root/pattern discovery semantics.
- ADR 0008 accepts the IMAP authentication/session direction: use a dedicated
  protocol auth adapter over local user password hashes, keep JWT out of IMAP,
  require TLS policy review before production enablement, keep `\Deleted`
  separate from gogomail soft-delete status, and handle MOVE as an IMAP
  source-expunge plus destination folder transition with fresh destination UIDs.
- `mailservice.NewIMAPAuthenticatorAdapter` now maps the existing
  Submission/local-password authentication boundary into `imapgw.Session`
  values, giving the listener a protocol-native authenticator without coupling
  IMAP to JWT middleware.
- `mailservice.NewIMAPBackendAdapter` composes the protocol authenticator with
  the service-backed store/session adapter, so the TCP listener can take one
  `imapgw.Backend` boundary.
- IMAP runtime now builds server options containing address, backend, TLS
  config, and insecure-auth policy for the TCP protocol server.
- `internal/imapgw.NewServer` now provides a protocol-server lifecycle shell
  with listener option validation, backend requirement checks, and TLS/insecure
  auth policy enforcement before the IMAP command parser is wired.
- The IMAP server shell can serve an initial connection greeting plus
  unauthenticated `CAPABILITY`, `NOOP`, `LOGIN`, and `LOGOUT` responses, giving
  TCP clients a bounded RFC-shaped handshake/auth surface before mailbox
  commands are enabled.
- Authenticated IMAP `SELECT` now maps to `imapgw.MailboxSessionStore`, returning
  permanent flags, `EXISTS`, `UIDVALIDITY`, `UIDNEXT`, and read-write completion
  metadata from the service-backed mailbox state.
- Authenticated IMAP `LIST` now maps to the service-backed mailbox list and
  returns sanitized quoted mailbox names with hierarchy delimiters, encoding
  non-ASCII names and ampersands as RFC 3501 modified UTF-7 while
  `UTF8=ACCEPT` remains unadvertised.
- Authenticated IMAP `STATUS` now maps to service-backed mailbox state and
  returns `MESSAGES`, `UIDNEXT`, `UIDVALIDITY`, and `UNSEEN` metadata.
- IMAP command parsing now supports basic quoted strings with backslash escapes,
  allowing common quoted `LOGIN` credentials and mailbox atoms while rejecting
  malformed quoted controls and unsupported command literal tokens. Bounded
  synchronizing command literals are consumed with a continuation response, and
  bounded non-synchronizing `LITERAL+` command literals are accepted when sent.
- IMAP `CAPABILITY` now advertises `AUTH=PLAIN` only before authentication, so
  post-login clients see capabilities for the selected protocol state.
- IMAP `AUTHENTICATE PLAIN` now accepts the standard continuation response,
  decodes SASL PLAIN credentials, returns tagged `BAD` for RFC cancellation,
  rejects mismatched delegated `authzid` values, and maps successful
  authentication into the same protocol session as `LOGIN`. Failed `LOGIN` and
  `AUTHENTICATE` attempts include RFC 5530 `[AUTHENTICATIONFAILED]` response
  codes for clients that parse machine-readable auth diagnostics.
- Authenticated selected-mailbox `UID FETCH` can now return UID, flags,
  RFC822 size metadata, and `BODY[]` literals streamed from the service-backed
  raw message fetch boundary. Untagged `FETCH` responses now use message
  sequence numbers, and `RFC822.SIZE` metadata requests do not trigger body
  streaming.
- `UID FETCH` accepts bounded numeric UID sets/ranges and recognizes
  `BODY.PEEK[]` as a body fetch request for read-without-side-effect clients.
- Non-UID `FETCH` accepts bounded message sequence sets, including `*`, and
  resolves them through the selected mailbox list before streaming the same
  metadata/body responses.
- `EXAMINE` now selects a mailbox read-only and blocks `UID STORE`, giving
  clients a standards-shaped read-only mailbox state.
- `EXAMINE` now passes read-only selection intent through the backend
  `SelectMailboxRequest`, so service adapters can distinguish read-only
  sessions from writable `SELECT`.
- `SELECT`/`EXAMINE` now establish mailbox event subscriptions before emitting
  selected-mailbox response data, avoiding ambiguous partial selection state
  when subscription setup fails.
- `CHECK` and `CLOSE` now cover selected-mailbox lifecycle calls; `CLOSE`
  silently expunges `\Deleted` messages for writable selections before clearing
  selected state, while read-only selections only clear state.
- `STATUS` now validates requested status data items and returns only the
  requested mailbox metadata fields.
- IMAP mailbox lookup now resolves wire names such as `INBOX` and
  `Archive/2026` to the stored mailbox ID before selected-mailbox state is used
  by follow-up commands.
- `LIST` now decodes RFC 3501 modified UTF-7 reference/pattern arguments,
  filters mailbox responses with exact, `*`, and `%` patterns over decoded
  names, and emits matching names in modified UTF-7 on the wire.
- `CAPABILITY` now advertises `SPECIAL-USE` and RFC 3348 `CHILDREN`; `LIST`
  includes RFC 3348 `\HasChildren` / `\HasNoChildren` hierarchy attributes
  plus RFC 6154 special-use attributes for system folders such as Drafts, Sent,
  Trash, Junk, Archive, All, and Flagged when those folder roles are present in
  storage metadata, and extended
  `LIST (SPECIAL-USE)`, `RETURN (SPECIAL-USE)`, and no-op
  `RETURN (CHILDREN)` forms are accepted.
- `CAPABILITY` now advertises RFC 5258 `LIST-EXTENDED` and RFC 5819
  `LIST-STATUS`; extended
  `LIST ... RETURN (STATUS (...))` emits the requested `STATUS` data directly
  after each matching selectable mailbox to reduce client folder-list round
  trips, can be combined with `RETURN (CHILDREN)`, and rejects malformed
  `RETURN (STATUS MESSAGES)` style status item lists before mailbox listing
  work.
- RFC 5258 `LIST-EXTENDED` now accepts `SUBSCRIBED` as a selection option and
  `RETURN (SUBSCRIBED)` as a return option, using the same subscription store
  as `LSUB` while keeping missing subscription names visible as `\Noselect`
  rows when clients explicitly list subscribed mailboxes.
- `CAPABILITY` now advertises RFC 8438 `STATUS=SIZE`; `STATUS` and
  `LIST-STATUS` can return active message octet totals per mailbox without
  fetching every message's `RFC822.SIZE`.
- `CAPABILITY` now advertises RFC 5256 `SORT`; `SORT` and `UID SORT` reuse the
  selected-mailbox search evaluator, require `US-ASCII` or `UTF-8` charset
  arguments, and return sorted sequence numbers or UIDs for the standard
  case-insensitive sort keys clients use for mailbox list ordering.
- `CAPABILITY` now advertises RFC 5256 `THREAD=ORDEREDSUBJECT`; `THREAD
  ORDEREDSUBJECT` and `UID THREAD ORDEREDSUBJECT` return ordered-subject thread
  trees from the selected-mailbox search result while keeping `REFERENCES`
  unadvertised until its Message-ID normalization and ancestry algorithm can be
  implemented without compatibility shortcuts.
- RFC 5256 base-subject extraction now decodes RFC 2047 encoded-word subjects
  before stripping reply/forward artifacts, improving internationalized
  `SORT SUBJECT` and `THREAD ORDEREDSUBJECT` compatibility.
- `LIST "" ""` and `LSUB "" ""` now return the hierarchy root with
  `\Noselect` and `/` delimiter metadata, matching client namespace delimiter
  probes before persistent subscription storage exists.
- `SELECT`/`EXAMINE` now emit `[PERMANENTFLAGS]` response codes so clients can
  distinguish writable and read-only flag state.
- `SELECT`/`EXAMINE` now emit RFC-shaped untagged `RECENT` counts alongside
  `EXISTS`, optional `[UNSEEN n]` first-unseen sequence hints, `UIDVALIDITY`,
  `UIDNEXT`, and optional `[HIGHESTMODSEQ ...]` metadata from durable mailbox
  UID state.
- `SELECT`/`EXAMINE` now emit RFC 4551-shaped `[NOMODSEQ]` when the session is
  CONDSTORE-aware through `SELECT ... (CONDSTORE)` or prior
  `ENABLE CONDSTORE` and the selected mailbox has no persistent mod-sequence
  baseline.
- `SELECT`/`EXAMINE` now emit `[UIDNOTSTICKY]` when the selected mailbox state
  reports non-sticky UIDs, keeping UIDPLUS-adjacent clients aware of mailbox
  UID persistence guarantees.
- `UID STORE` now supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses for those requests.
- `FETCH`/`UID FETCH` now include `INTERNALDATE` and RFC-shaped `ENVELOPE`
  attributes when requested, using the service-backed message summary fields.
- Service-backed IMAP summaries now hydrate stored `To`, `Cc`, and `Bcc`
  address JSON into RFC-shaped ENVELOPE address lists, so repository-backed
  `FETCH ENVELOPE`, address search, and address sort paths share the same
  recipient metadata as Mail API storage.
- Shared fetch failure paths now use the issued command name in tagged
  failures, keeping regular `FETCH` failures distinct from `UID FETCH`
  failures in client-visible responses.
- `FETCH`/`UID FETCH` now applies RFC 3501 `\Seen` side effects for successful
  `BODY[...]`, `RFC822`, and `RFC822.TEXT` literal reads, while
  `BODY.PEEK[...]` and `RFC822.HEADER` remain preview-safe and non-mutating.
- `FETCH`/`UID FETCH` now preserves `RFC822`, `RFC822.HEADER`, and
  `RFC822.TEXT` response data item names on the wire instead of rewriting them
  to their `BODY[...]` equivalent names.
- `CAPABILITY` now advertises `CONDSTORE` and `ENABLE`; RFC 5161-shaped
  `ENABLE CONDSTORE` marks sessions CONDSTORE-aware before mailbox selection.
- `FETCH`/`UID FETCH` now include RFC 4551-shaped `MODSEQ (n)` attributes when
  requested, backed by durable per-message IMAP mod-sequences.
- `SEARCH`/`UID SEARCH` now support RFC 4551-shaped `MODSEQ` criteria,
  including optional metadata entry/type arguments, and return the highest
  matched mod-sequence in non-empty SEARCH responses.
- `FETCH`/`UID FETCH` now support RFC 4551-shaped `CHANGEDSINCE` modifiers,
  returning only messages with greater per-message mod-sequences and
  implicitly adding `MODSEQ` attributes.
- Sessions now become CONDSTORE-aware after `FETCH MODSEQ`,
  `FETCH CHANGEDSINCE`, `SEARCH MODSEQ`, or `STATUS HIGHESTMODSEQ`; subsequent
  flag `FETCH` event/STORE echo responses include `MODSEQ`. The
  `STATUS HIGHESTMODSEQ` path is regression-covered across a following
  `SELECT` and `UID STORE` so awareness survives mailbox selection.
- `ENABLE CONDSTORE` issued after a mailbox is already selected now emits the
  selected mailbox's `HIGHESTMODSEQ` or `NOMODSEQ` before tagged completion,
  satisfying RFC 7162 first-enabling-command semantics and keeping MODSEQ
  baselines visible to late-enabling clients. Repeated `ENABLE CONDSTORE`
  after `SELECT ... (CONDSTORE)` or after `STATUS HIGHESTMODSEQ` plus `SELECT`
  is regression-covered so already aware sessions do not re-emit the selected
  mailbox baseline.
- Mailboxes selected with `NOMODSEQ` now reject `FETCH`/`UID FETCH`
  `MODSEQ`/`CHANGEDSINCE`, `SEARCH`/`SORT`/`THREAD` `MODSEQ`, and
  `STORE`/`UID STORE` `UNCHANGEDSINCE` before backend mutation or scan work,
  matching RFC 7162's non-persistent mod-sequence semantics.
- `STORE`/`UID STORE` now support RFC 4551-shaped `(UNCHANGEDSINCE n)`
  modifiers with transactional per-message mod-sequence checks, partial
  success for passing messages, and UID/sequence `[MODIFIED ...]`
  stale-write responses. Conditional store response/event paths filter modified
  stale UIDs out of successful `FETCH` echoes and mailbox flag notifications.
- `SELECT` and `EXAMINE` now accept the RFC 4551-shaped `(CONDSTORE)`
  parameter and mark the session CONDSTORE-aware.
- `FETCH`/`UID FETCH` keep a conservative single-part `BODYSTRUCTURE` fallback
  when only message headers are available, while metadata-only structure
  fetches now reopen the bounded raw message stream for richer MIME tree
  serialization.
- Single-part `BODY`/`BODYSTRUCTURE` responses now derive content type,
  parameters, content-transfer-encoding, ID, and description from bounded raw
  message headers instead of always reporting text/plain defaults.
- Metadata-only `BODYSTRUCTURE` fetches now use the streaming MIME-structure
  parser to return multipart child order, subtype, parameters, transfer
  encodings, dispositions, body octets, and text line counts without retaining
  attachment payloads.
- `BODYSTRUCTURE` now emits RFC 3501-shaped `message/rfc822` bodies with
  encapsulated message header-derived envelope metadata, parsed nested body
  structure, and line counts instead of treating attached messages as generic
  basic parts.
- The shared MIME-structure parser now descends into `message/rfc822` parts
  while counting the encapsulated message bytes/lines and capturing bounded
  envelope metadata, so forwarded-message attachments expose nested body
  metadata without retaining payloads.
- `FETCH`/`UID FETCH` can now return RFC 3501-shaped `BODY[n.HEADER]` and
  `BODY[n.TEXT]` literals for `message/rfc822` parts, including
  forwarded-message attachments inside multipart messages.
- `FETCH`/`UID FETCH` can now return `BODY[n.HEADER.FIELDS (...)]` and
  `BODY[n.HEADER.FIELDS.NOT (...)]` subsets for `message/rfc822` parts, so
  clients can preview forwarded-message headers without fetching whole nested
  headers.
- `FETCH`/`UID FETCH` can now follow multipart body-part numbering inside
  top-level `message/rfc822` parts, including nested part MIME headers such as
  `BODY[1.2]` and `BODY[1.2.MIME]`.
- IMAP literal-fetch regression coverage now includes multipart messages that
  attach a `message/rfc822` whose encapsulated body is itself multipart,
  guarding forwarded-message paths such as `BODY[2.2]` and `BODY[2.2.MIME]`.
- IMAP `BODYSTRUCTURE` regression coverage now includes the same forwarded
  multipart shape, guarding nested `MESSAGE/RFC822` serialization when the
  encapsulated message body is multipart.
- Malformed encapsulated `message/rfc822` literals now degrade gracefully for
  nested section fetches, returning an empty header section and raw text bytes
  instead of failing the whole IMAP `FETCH`.
- Combined `BODYSTRUCTURE` plus literal body/header fetches can reopen the raw
  message for MIME metadata while preserving the original reader for literal
  streaming, so common preview/header fetch batches keep rich structure
  responses.
- `FETCH`/`UID FETCH` now supports standard `FAST`, `ALL`, and `FULL` macros,
  including the non-extensible `BODY` attribute for `FULL`.
- `FETCH`/`UID FETCH` now support bounded header-only literals for
  `BODY[HEADER]`, `BODY.PEEK[HEADER]`, and `RFC822.HEADER`.
- Non-UID `FETCH` now uses the same bounded header literal path as `UID FETCH`
  for `BODY[HEADER]` and `RFC822.HEADER`.
- `FETCH`/`UID FETCH` now support bounded text-only section literals for
  `BODY[TEXT]`, `BODY.PEEK[TEXT]`, and `RFC822.TEXT`, rejecting oversized
  section bodies before unbounded allocation.
- `FETCH`/`UID FETCH` now support conservative single-part text literals for
  `BODY[1]` and `BODY.PEEK[1]`.
- `FETCH`/`UID FETCH` now supports bounded top-level multipart body-section
  literals such as `BODY[1]` and `BODY[2]`, letting clients read individual
  MIME parts without fetching the full message.
- `FETCH`/`UID FETCH` now supports bounded nested multipart body-section
  literals such as `BODY[1.2]` with a capped MIME part path depth.
- `FETCH`/`UID FETCH` now supports bounded partial windows over multipart
  body-section literals such as `BODY.PEEK[2]<4.4>`.
- `FETCH`/`UID FETCH` now answers conservative single-part MIME header requests
  for `BODY[1.MIME]` and `BODY.PEEK[1.MIME]`.
- `FETCH`/`UID FETCH` now streams actual multipart child MIME headers for
  `BODY[n.MIME]` and `BODY.PEEK[n.MIME]` when the selected part exists.
- `UID STORE` now accepts bounded UID sets/ranges so clients can mutate flags in
  batches instead of issuing one command per message.
- Non-UID `STORE` now accepts bounded sequence sets/ranges and maps them to the
  same service-backed flag mutation boundary as `UID STORE`.
- Non-UID `STORE` now supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses for those requests.
- `NOOP` now drains queued selected-mailbox events as untagged `EXISTS`,
  `EXPUNGE`, and flag `FETCH` updates, giving clients a polling path alongside
  live IDLE and suppressing stale or duplicate exact-count `EXISTS` events.
- `IDLE` is now advertised and accepted, streaming selected-mailbox events while
  the client is waiting and completing when the client sends `DONE`.
- `SEARCH ALL`, `SEARCH UID <set>`, and `UID SEARCH ALL` now work over the
  selected mailbox message list.
- `SEARCH`/`UID SEARCH` now accepts sequence-set criteria such as `2:*`,
  letting clients intersect standard search predicates with selected mailbox
  sequence ranges.
- `SEARCH`/`UID SEARCH` can combine supported criteria with RFC default AND
  semantics, including `ALL` plus flag, date, size, address, and UID filters.
- `SEARCH`/`UID SEARCH` supports RFC `NOT` and binary `OR` criteria
  composition over the supported search predicate set.
- `SEARCH`/`UID SEARCH` now accepts parenthesized search-key groups, combining
  grouped predicates with RFC default AND semantics and allowing grouped
  operands inside `OR`.
- `CAPABILITY` now advertises RFC 4731 `ESEARCH`; `SEARCH RETURN (...)` and
  `UID SEARCH RETURN (...)` return single untagged `ESEARCH` responses with
  requested `MIN`, `MAX`, compact `ALL`, `COUNT`, UID indicators, and
  CONDSTORE `MODSEQ` data.
- `CAPABILITY` now advertises RFC 5182 `SEARCHRES`; `SEARCH RETURN (SAVE)`
  stores the selected-session search result so `$` can be reused by later
  sequence-set and UID-set commands without sending the result set back to the
  client.
- `SORT`/`UID SORT` and `THREAD`/`UID THREAD` now accept `RETURN (SAVE)` before
  their normal arguments, save successful matched results for `$` reuse, clear
  the saved result on save-requested tagged `NO`, and leave tagged `BAD`
  malformed save attempts non-mutating.
- Direct `ESEARCH` and `UID ESEARCH` commands remain outside the advertised
  surface until RFC 7377 `MULTISEARCH` is intentionally implemented; they now
  return a targeted `BAD` diagnostic instead of looking like ordinary unknown
  commands.
- `SEARCH RETURN (SAVE)` now clears the selected-session `$` result when the
  save-requested search fails with tagged `NO`, while tagged `BAD` searches
  leave the previous result untouched as required by RFC 5182.
- `FETCH`/`UID FETCH` now supports partial full-body literals for
  `BODY[]<offset.count>` and `BODY.PEEK[]<offset.count>`.
- `FETCH`/`UID FETCH` now supports bounded partial section literals for common
  `BODY[HEADER]`, `BODY[TEXT]`, `BODY[1]`, and `BODY[1.MIME]` requests.
- `SEARCH`/`UID SEARCH` now support common flag criteria such as `UNSEEN`,
  `FLAGGED`, `ANSWERED`, and `DRAFT` for standard client views.
- `STORE`/`UID STORE` can persist the IMAP-specific `\Deleted` flag separately
  from gogomail's soft-delete status, and `FETCH`/`SEARCH` expose that flag
  through `FLAGS`, `DELETED`, and `UNDELETED`.
- `SEARCH`/`UID SEARCH` now supports `RECENT`, `OLD`, and `NEW` against
  per-message recentness. `NEW` matches messages that are both recent and
  unseen, while `OLD` matches non-recent messages, keeping RFC-shaped search
  behavior available when a backend can expose recent state.
- IMAP custom keyword flags are now supported at the protocol-core boundary:
  backend-provided permanent keyword atoms can be advertised, `FETCH FLAGS`
  renders canonical duplicate-free keywords, `SEARCH KEYWORD`/`UNKEYWORD`
  evaluates them, and `STORE` accepts custom keywords only when the selected
  mailbox advertises them. PostgreSQL `maildb` now persists those user
  keywords in the IMAP-specific `imap_keywords` JSONB flag array across append,
  store, copy, move, fetch, and search paths.
- `FETCH`/`UID FETCH` now supports `BODY[HEADER.FIELDS (...)]` and
  `BODY.PEEK[HEADER.FIELDS (...)]` for lightweight preview metadata reads.
- `FETCH`/`UID FETCH` now supports bounded partial windows over
  `BODY[HEADER.FIELDS (...)]`, `BODY.PEEK[HEADER.FIELDS (...)]`,
  `BODY[HEADER.FIELDS.NOT (...)]`, and `BODY.PEEK[HEADER.FIELDS.NOT (...)]`
  reads.
- `FETCH`/`UID FETCH` now echoes requested `HEADER.FIELDS` and
  `HEADER.FIELDS.NOT` section names in literal response items, including
  partial-window suffixes, instead of normalizing subset literals to
  `BODY[HEADER]`.
- `FETCH`/`UID FETCH` now supports `BODY[HEADER.FIELDS.NOT (...)]` and
  `BODY.PEEK[HEADER.FIELDS.NOT (...)]` for exclude-style header reads.
- `SEARCH`/`UID SEARCH` now supports `SINCE`, `BEFORE`, and `ON` over message
  `INTERNALDATE`, plus `SENTSINCE`, `SENTBEFORE`, and `SENTON` over envelope
  dates.
- `SEARCH`/`UID SEARCH` now supports basic `FROM`, `TO`, `CC`, `BCC`, and
  `SUBJECT` substring criteria over selected-mailbox summaries.
- `SEARCH`/`UID SEARCH` now supports bounded `BODY` and `TEXT` raw-message
  criteria scans, with `BODY` excluding the RFC 5322 header block.
- `SEARCH`/`UID SEARCH` now supports bounded RFC `HEADER <field> <value>`
  criteria scans over the raw message header block.
- `SEARCH`/`UID SEARCH` now preserves RFC 3501 zero-length search string
  semantics for quoted empty strings across envelope, body/text, and header
  substring criteria instead of treating them as guaranteed no-match requests.
  Escaped quote characters inside search strings remain literal query bytes
  rather than being trimmed from the query boundary.
- `SEARCH`/`UID SEARCH` now supports RFC 3501 `LARGER` and `SMALLER` criteria
  over message `RFC822.SIZE` metadata.
- `SEARCH`/`UID SEARCH` now accepts `CHARSET US-ASCII` and `CHARSET UTF-8`
  prefixes and returns an RFC-shaped `[BADCHARSET]` response for unsupported
  search charsets.
- Authenticated `NAMESPACE` now advertises the personal namespace and `/`
  hierarchy delimiter for mailbox discovery.
- `CAPABILITY` now advertises `NAMESPACE` alongside the implemented namespace
  command so client discovery matches the supported command surface.
- Authenticated `SUBSCRIBE`/`UNSUBSCRIBE` now persist mailbox subscription
  names, and `LSUB` returns the saved subscription set instead of every visible
  mailbox.
- IMAP subscription canonicalization preserves hierarchy delimiters, quoting,
  internal spacing, and real leading/trailing spaces while keeping
  case-insensitive matching, preventing distinct subscribed mailbox names from
  collapsing into the same `LSUB` row. The service/repository subscription
  boundary now preserves decoded mailbox-name spacing instead of trimming it
  before `SUBSCRIBE`/`UNSUBSCRIBE` persistence.
- Live IMAP mailbox-event subscription now preserves decoded mailbox ID
  spacing after validation, keeping IDLE/NOOP update fan-out keyed to the exact
  selected mailbox identity instead of a trimmed variant.
- Service-backed IMAP mailbox lookup for `SELECT`/`EXAMINE` now preserves the
  decoded mailbox ID before repository delegation, and PostgreSQL mailbox/
  APPEND-target lookup now allows compatibility alias fallback only for inputs
  without leading/trailing whitespace. Service-backed `APPEND` target lookup
  now preserves decoded destination mailbox ID before repository delegation,
  read-side `FETCH`/message listing follows the same identity boundary, and
  service-level `STORE`, `COPY`, `MOVE`, and `EXPUNGE` now preserve decoded
  mailbox IDs through repository and event boundaries. PostgreSQL UID/message
  operations now also reject padded mailbox UUIDs instead of trimming them
  before UUID-bound queries, and service-level admin UID backfill now preserves
  exact mailbox IDs before repository/audit handling. Next, review remaining
  IMAP helper DTO formatting that trims display names so it stays limited to
  rendered names rather than storage identity.
- `SUBSCRIBE` can now retain missing mailbox names so `LSUB` can expose them
  with `\Noselect`, matching client migration and deleted-mailbox recovery
  behavior that expects subscriptions to outlive selectable mailboxes.
- `LSUB` retains subscribed names after mailbox deletion with `\Noselect` and
  covers the RFC 3501 `%` hierarchy parent response case.
- IMAP mailbox-taking commands decode RFC 3501 modified UTF-7 mailbox
  arguments before crossing into the service boundary, covering selection,
  status, append, copy/move, mutation, and subscription paths while rejecting
  raw 8-bit and malformed alternate forms instead of leaking wire encoding into
  storage.
- IMAP quoted-string response formatting preserves ordinary internal spacing
  while still escaping quotes/backslashes and cleaning controls, so `LIST`,
  `LSUB`, `STATUS`, FETCH metadata, and MIME parameter values keep their wire
  identity.
- IMAP now advertises and supports RFC 2971 `ID`, validating bare no-argument
  probes, `NIL`, or bounded field/value parameter lists before returning
  gogomail server identity.
- IMAP now advertises and supports `UNSELECT`, clearing selected-mailbox state
  without invoking `CLOSE`/EXPUNGE semantics.
- `EXPUNGE` and `UID EXPUNGE` now delete only messages marked with the
  IMAP-specific `\Deleted` flag, emit RFC-shaped untagged sequence-number
  `EXPUNGE` responses, remove stale mailbox UID rows, and publish best-effort
  expunge events through the service boundary.
- `COPY` and `UID COPY` now resolve source message sequence/UID sets, validate
  the destination mailbox, duplicate active message metadata and attachment
  rows transactionally, assign fresh destination mailbox UIDs, return UIDPLUS
  `[COPYUID ...]` response codes when destination UIDs are available, and
  publish best-effort destination `EXISTS` events through the service boundary.
  Missing destination mailboxes now return `[TRYCREATE]`.
- `MOVE` and `UID MOVE` now resolve source sequence/UID sets through the
  selected mailbox, validate the destination mailbox, move active messages
  transactionally, assign fresh destination UIDs, and allow moves back into the
  selected mailbox by creating a fresh same-mailbox message before expunging
  the source UID. Responses return UIDPLUS `[COPYUID ...]` mappings in the
  final tagged OK when destination UIDs are available, advance and return
  source mailbox `[HIGHESTMODSEQ ...]` metadata for CONDSTORE-aware clients,
  emit RFC-shaped source `EXPUNGE` responses, and return `[TRYCREATE]` when
  the destination mailbox is missing.
- `APPEND` now has a protocol-to-backend request boundary for mailbox, optional
  flag-list, optional internal date-time, literal body, and size after bounded
  literal framing. The boundary now carries UIDPLUS-ready append metadata so
  successful storage can emit `[APPENDUID uidvalidity uid]`; the service layer
  spools and size-checks the literal, parses the RFC message, writes raw `.eml`
  through the configured storage backend, and `maildb` records metadata, quota,
  outbox, and mailbox UID state transactionally. Missing destination mailboxes
  now produce an RFC-shaped `[TRYCREATE]` response code, and quota rejection
  produces `[OVERQUOTA]`. APPEND commands without a synchronizing literal are
  now syntax `BAD` responses rather than unsupported-command responses.
  Successful append results include the new message sequence number for precise
  selected-mailbox `EXISTS` event counts. APPEND internaldate parsing accepts
  RFC 3501 space-padded one-digit date-days such as `" 5-May-2026 ..."` while
  rejecting bare one-digit dates such as `"5-May-2026 ..."`. Service-level
  APPEND rejects CR/LF-bearing or oversized user/mailbox
  identifiers before repository lookup, spooling, parsing, storage, or quota
  work.
- Service-level IMAP `STORE`, `COPY`, `MOVE`, and `EXPUNGE` mutations reject
  CR/LF-bearing or oversized user/mailbox identifiers before repository
  mutation dispatch or mailbox event publication.
- Service-level IMAP read/list/subscription/backfill operations reject
  CR/LF-bearing or oversized user/mailbox identifiers before repository reads,
  storage opens, event subscriptions, or UID backfill work.
- Service-level IMAP `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` calls
  reject zero UIDs before repository or storage work, keeping direct callers
  aligned with RFC 3501's positive UID model.
- Service-level IMAP `STORE`, `COPY`, and `MOVE` calls reject empty UID sets
  before repository work, while `EXPUNGE` preserves nil UID sets for `CLOSE`
  style "all deleted messages" semantics.
- Folder list/create/rename/delete service methods reject CR/LF-bearing or
  oversized user identifiers, and create/rename reject unsafe folder names,
  before repository work.
- Empty IMAP flag-lists are accepted where RFC-shaped clients can send them:
  `APPEND ()` stores without initial flags, `STORE FLAGS ()` clears supported
  flags, and empty `+FLAGS ()`/`-FLAGS ()` are successful no-ops.
- Selected-mailbox `APPEND` now prefers the backend-returned appended message
  sequence number for the untagged `EXISTS` count, falling back to a local
  increment only when precise sequence metadata is unavailable.
- Selected-mailbox `COPY` and same-mailbox `MOVE` now also prefer
  backend-returned destination message sequence numbers for untagged `EXISTS`
  counts, falling back to local increments only when precise metadata is
  unavailable.
- Selected-mailbox `EXPUNGE` events delivered through `NOOP` or `IDLE` now
  adjust saved SEARCHRES `$` sequence numbers the same way explicit `EXPUNGE`
  commands do, keeping subsequent `$` reuse aligned with visible mailbox state.
- `CREATE`, `DELETE`, and `RENAME` now delegate to the service folder boundary
  for authenticated flat user-mailbox management, resolving wire names before
  destructive or rename operations and preserving the existing folder
  validation/storage constraints.
- `CREATE INBOX` and `DELETE INBOX` now return explicit RFC 3501-shaped `NO`
  failures, and `RENAME INBOX` is rejected instead of being treated like a
  generic folder rename until its required special message-moving semantics are
  implemented.
- `EXAMINE` setup failures now return `NO EXAMINE failed` instead of
  `NO SELECT failed`, keeping tagged failure responses aligned with the command
  clients actually issued.
- Malformed recognized `UID` subcommands now reach their command-specific
  validators, so incomplete or structurally invalid `UID SEARCH`, `UID SORT`,
  `UID THREAD`, `UID FETCH`, `UID STORE`, `UID EXPUNGE`, and `UID COPY`
  produce precise tagged `BAD` responses before authentication/selected-state
  checks instead of a generic UID-dispatch failure.
- Missing-mailbox failures for `SELECT`, `EXAMINE`, `STATUS`, `DELETE`, and
  `RENAME` now return tagged `[NONEXISTENT]` response codes instead of generic
  command failures, so clients can distinguish absent folders from transient
  backend failures.
- Selected-state no-argument commands `CHECK`, `CLOSE`, `UNSELECT`, and
  `EXPUNGE` now reject extra arguments with tagged `BAD` responses instead of
  ignoring malformed input, protecting destructive expunge handling from
  ambiguous client commands.
- Any-state no-argument commands `CAPABILITY`, `NOOP`, and `LOGOUT` now reject
  extra arguments with tagged `BAD` responses instead of silently accepting
  malformed commands or ending the session for malformed logout attempts.
- `STATUS` now requires a parenthesized status item list, rejecting malformed
  `STATUS mailbox MESSAGES`-style requests before mailbox metadata lookup.
- Command dispatch now rejects malformed tags containing atom-special
  characters with untagged `BAD` responses before command handling, avoiding
  ambiguous tagged replies for invalid client command tags.
- Command parsing now rejects control characters or 8-bit non-ASCII bytes
  inside unquoted atoms and quoted strings, aligning parser behavior with RFC
  3501's 7-bit string/atom boundary before command dispatch.
- IMAP quoted-string response rendering now replaces invalid UTF-8 and
  non-ASCII runes with `?`, keeping ENVELOPE, BODYSTRUCTURE, STATUS, LIST, and
  related quoted strings 7-bit safe until an explicit IMAP UTF-8 extension is
  advertised.
- IMAP ENVELOPE subject, message-id, in-reply-to, and address display/mailbox/
  host nstrings now share the bounded UTF-8-safe metadata text path before
  response quoting, preventing oversized backend metadata from inflating FETCH
  responses.
- IMAP ENVELOPE address lists are capped after placeholder filtering,
  preventing abnormal recipient fan-out metadata from amplifying FETCH
  responses without letting malformed empty entries hide later valid addresses.
- Malformed empty or incomplete ENVELOPE address entries are dropped before
  rendering, so backend placeholder data cannot emit stray `(NIL NIL NIL NIL)`
  or display-name-only address tuples.
- `STARTTLS` is now supported on plaintext IMAP listeners with configured TLS,
  and is advertised only before the connection upgrades.
- `STARTTLS` completion now includes an updated `[CAPABILITY ...]` response
  code for the post-TLS command surface.
- Plaintext IMAP sessions advertise `LOGINDISABLED` and reject
  `LOGIN`/`AUTHENTICATE` with `[PRIVACYREQUIRED]` when insecure auth is
  disabled before STARTTLS.
- IMAP now advertises `LITERAL+` and accepts bounded non-synchronizing command
  literals such as `APPEND ... {n+}` without an extra continuation round trip,
  while preserving the existing synchronizing literal path for conservative
  clients.
- IMAP command reading now supports bounded literals in non-final command
  positions and multiple literals in one command, so literalized credentials,
  mailbox names, and search strings are no longer constrained to terminal
  APPEND-style framing.
- IMAP command reading now enforces the command-literal memory cap across the
  cumulative literal payloads in one command, so multiple individually valid
  literals cannot exceed the per-command memory ceiling.
- IMAP server coverage now verifies `LOGIN` commands that carry both the user
  name and password as separate synchronizing literals, including the
  credentials delivered to the backend auth boundary.
- IMAP command and IDLE line reads now enforce the command-line byte cap while
  reading from the socket instead of after an unbounded line allocation.
- Oversized lines received during an active IMAP `IDLE` continuation now produce
  the pending command tag's `BAD command line is too long` response followed by
  `BYE`, matching ordinary command framing-error handling.
- IMAP oversized command literals now produce an RFC-shaped tagged `BAD`
  response when possible followed by `BYE`, so clients receive a clear protocol
  outcome while the server still closes unrecoverable framing errors instead of
  attempting unsafe stream resynchronization.
- `AUTHENTICATE PLAIN` now supports `SASL-IR` initial responses, reducing
  authentication round trips for compatible IMAP clients.
- `LOGIN` and SASL PLAIN decoded credentials now reject blank, CR/LF-bearing,
  or oversized authentication identities plus oversized or CR/LF-bearing
  passwords before backend auth work, while preserving intentional
  leading/trailing spaces in RFC string credentials. SASL PLAIN still rejects
  empty decoded passwords; `LOGIN` allows empty quoted passwords to flow into
  backend authentication as credential failures.
- SASL PLAIN encoded and decoded response bytes are now bounded before
  credential splitting, preventing literal initial responses from forcing
  avoidable decode allocation beyond the configured credential caps.
- Authenticated selected-mailbox `UID STORE` now maps `FLAGS`, `+FLAGS`, and
  `-FLAGS` for supported system flags to the service-backed flag mutation
  boundary and returns updated flag metadata.
- `gogomail --mode=imap` now opens the configured TCP listener and serves the
  IMAP server shell with greeting, `CAPABILITY`, `NOOP`, `LOGIN`, `SELECT`,
  `FETCH`/`UID FETCH`, `STORE`/`UID STORE`, `SEARCH`, `SORT`, `IDLE`,
  `STARTTLS`, `CREATE`/`DELETE`/`RENAME`, `APPEND`, `COPY`, `MOVE`, `EXPUNGE`,
  `CLOSE`, `UNSELECT`, and `LOGOUT` over the service-backed mailbox/session
  boundary.
- `gogomail --mode=imap` now starts a dedicated Redis consumer group for
  committed `mail.stored` events and publishes UID-bearing `EXISTS` updates
  into its process-local mailbox event broker so live IDLE sessions can observe
  newly delivered mail.
- `internal/message` now has a bounded streaming MIME-structure parser that
  walks multipart trees, preserves raw transfer-encoding metadata, counts body
  octets/lines, and avoids retaining attachment payloads for future IMAP
  `BODYSTRUCTURE` serialization.

Next:

- Extend MIME literal fetches with captured real-client fixture variants as
  they become available.

Frontend note:

- When frontend implementation starts, use Next.js with TypeScript and shadcn/ui,
  follow `DESIGN.md`, and aim for a Notion Mail-like UI/UX.

### 4. Pipeline extension hooks

Current state:

- SMTP pipeline defines stages/hooks but they are not fully pluggable.
- Attachment scan hook exists as a disabled-by-default synchronous SMTP-stage
  adapter, and `GOGOMAIL_ATTACHMENT_SCAN_BACKEND=webhook` wires a bounded HTTP
  scanner with an optional bounded bearer token into Edge, Inbound, and
  Submission MTA app boundaries. `docs/webhook-integrations.md` records the
  scanner JSON payload, bounded request/response behavior, and verdict
  semantics.
- Push notification enqueue now has a disabled-by-default async
  `push-notification-worker` over `mail.stored` with a replaceable sink and
  `slog` first adapter plus `GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webhook` for
  handing raw-token targets to an external push gateway with an optional
  bounded bearer token; webhook URLs must be HTTPS in production.
  `docs/webhook-integrations.md` records the push gateway payload and attempt
  state semantics. Target resolution drops blank, CR/LF-bearing, oversized, or
  unsupported targets before candidate recording, and the webhook sink bounds
  direct-call payload metadata before JSON serialization.
- Shared webhook/OpenSearch HTTP response cleanup drains a small bounded body
  window before close, improving connection reuse for external scanner, push,
  indexing, bootstrap, and relevance-query calls without unbounded cleanup
  reads.
- Admin push-notification attempt/stats repository filters reject
  invalid-UTF-8, CR/LF-bearing, or oversized direct-call values before SQL
  dispatch.
- User-scoped push device storage now exists for `apns`, `fcm`, and `webpush`
  tokens through the Mail API. Responses expose only a token suffix; raw tokens
  remain write-only, and create/update rejects invalid-UTF-8, unsafe, or
  oversized user/token metadata before repository upsert.
- The worker can resolve active user devices from Postgres with
  `GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT`, then pass those targets to the
  sink without coupling SMTP or storage writes to vendor delivery. Malformed
  resolved targets with blank or CR/LF-bearing device IDs/tokens, or
  unsupported platforms, are dropped before sink handoff.
- The worker records per-device candidate attempts to
  `push_notification_attempts` before sink handoff, then marks those attempts
  `queued` only after the sink succeeds. Failed sink handoffs are marked
  `failed` with the sink error before the handler returns an error for stream
  retry.
- Admin API exposes `GET /admin/v1/push-notification-attempts` with bounded
  message/status/user/platform/device/provider-status/provider-message/since
  filters for inspecting candidate fan-out and vendor outcomes.
- Admin API exposes `GET /admin/v1/push-notification-attempts/{id}` for
  single-attempt troubleshooting.
- Admin API exposes
  `PATCH /admin/v1/push-notification-attempts/{id}/outcome` so authenticated
  operators or external push gateways can record queued/delivered/failed/
  invalid-token outcomes with bounded provider diagnostics.
- Admin API exposes `GET /admin/v1/push-notification-stats` for active-device
  and status-count summaries, with optional `message_id`, `user_id`, and
  `platform`/`device_id`/`since` scoping for per-message, per-user,
  provider-platform, per-device, and recent-window troubleshooting.
- Candidate recording returns an attempt id to the worker sink, giving future
  vendor adapters a stable row to update with delivered/failed/invalid-token
  outcomes.
- Candidate and provider-outcome diagnostics are capped at UTF-8 boundaries
  before storage so internationalized subjects and vendor messages remain valid
  in Admin API views.
- Push notification candidate recording rejects invalid-UTF-8, CR/LF-bearing,
  or oversized message/user/device/company/domain IDs before SQL insert
  dispatch, and rejects unsupported platforms at the recorder boundary.
- Existing attempts can be updated with queued, delivered, failed, or
  invalid-token outcomes through the internal recorder or the Admin API.
- Internal push worker outcome updates and authenticated Admin outcome updates
  share the same `maildb` storage path, reducing drift before vendor gateway
  callbacks are wired more deeply.
- Push notification outcome recording rejects invalid-UTF-8, CR/LF-bearing, or
  oversized attempt IDs before SQL update dispatch.
- Invalid-token outcomes soft-delete the matching user push device in the same
  Postgres transaction as the attempt update.
- `mail.stored` event payloads carry an explicit schema version; preserve this
  contract when adding fields for audit, search, push, IMAP, or future fan-out
  workers.
- Audit, search, and push consumers enforce known explicit schema versions; add
  a new accepted version before introducing incompatible event payload changes.
- Spam and vendor FCM/APNs delivery are not wired.

Next:

- Add first-party FCM/APNs/Web Push sink adapters behind `internal/pushnotify`
  when provider credentials and deployment expectations are decided.
- Use the authenticated Admin outcome endpoint for external push gateway
  callbacks until first-party provider adapters are added.
- Keep hooks disabled by default and wired only in `app/run.go`.

### 5. Attachment upload API

Current state:

- Attachment table and storage model exist.
- Attachment endpoints exist in the Mail API.
- Domain outbound policy can enforce `max_attachment_bytes` for attachment
  metadata reservation and direct multipart upload before storage writes.
- Mail API can cancel a pending user-scoped upload immediately, releasing
  reserved quota and deleting any stored upload object.
- Mail API exposes attachment upload capabilities so clients can discover
  current limits and supported modes without hard-coding them.
- ADR 0007 defines the resumable/chunked attachment upload boundary: explicit
  upload sessions, quota reservation at session creation, adapter-owned staged
  chunks, normal attachment rows after finalization, and bounded cleanup.
- `attachment_upload_sessions` migration defines the future resumable upload
  session state table, including declared/received byte counts, lifecycle
  status, expiry, storage adapter metadata, and cleanup-oriented indexes.
- `maildb` can create a resumable upload session record and reserve the
  declared size in the shared quota ledger in one transaction.
- `maildb` can cancel pending/uploading/failed upload sessions, marking them
  `canceled` and releasing the declared size without allowing duplicate quota
  release on repeated cancellation.
- `maildb` can expire stale pending/uploading/failed upload sessions in bounded
  batches, marking them `expired` and releasing declared quota reservations.
- `maildb` can count stale pending/uploading/failed upload sessions under the
  same normalized cleanup cap, enabling non-destructive operator previews.
- `maildb` can list stale upload-session candidates in the same bounded order
  used for expiry, giving Admin previews row-level visibility.
- `mailservice` exposes resumable upload session create/cancel/expire methods
  over the repository boundary, reusing attachment metadata validation,
  max-size checks, CR/LF/size-bounded user/session identifiers, and domain
  outbound attachment policy enforcement.
- Stale upload cleanup can run as `attachment-cleanup-worker` with configurable
  interval, stale age, batch size, and optional run-once mode for CronJob-style
  deployments, and now expires stale resumable upload sessions in the same
  bounded sweep.
- Mail API exposes upload session create/read/cancel endpoints, reserving declared
  quota for future resumable workflows without yet advertising chunk support.
- Upload session creation rejects already-expired or overlong `expires_at`
  values before quota reservation.
- Attachment upload capabilities advertise session create/cancel support
  separately from `resumable_chunked_uploads` and include the max session TTL.
- Upload session body storage can persist a complete body and checksum without
  finalizing it into an attachment row.
- Upload session body replacement preserves the previously recorded staged body
  if repository metadata recording fails, and removes the previous body after a
  successful replacement on a best-effort basis.
- Upload session body storage can reject checksum mismatches when clients send
  `X-Content-SHA256`.
- Attachment upload capabilities advertise checksum precondition support for
  upload session body storage.
- Upload session finalization can convert a stored session body into the normal
  pending attachment row while preserving the original quota reservation.
- Upload session finalization verifies staged object size and SHA-256 before
  creating the attachment row.
- Upload session cancellation deletes any staged session body after the
  repository marks the session canceled and releases quota.
- Upload session expiry deletes staged session bodies for expired sessions,
  keeping worker-driven cleanup aligned with quota release.
- Admin API can preview counts, list bounded candidates, and run stale upload
  cleanup on demand with an explicit non-future cutoff for operator-controlled
  maintenance. Cleanup run/dry-run responses include stale upload-session
  candidate and expired counts, and candidate previews include bounded
  upload-session rows, matching the background worker's full cleanup scope.

Next:

- Decide whether to split body storage into explicit range-aware chunk commits,
  then flip `resumable_chunked_uploads` only after the retry/range semantics are
  complete.

### 6. OpenAPI/client readiness

Current state:

- Route, request body, response envelope, operationId, and component reference
  drift tests all pass.
- All schemas synchronized with Go types after platform hardening sprint.

Next:

- Keep `docs/openapi.yaml` synchronized with every HTTP route change.
- Consider generating a TypeScript client from the OpenAPI spec for future
  frontend use.
- Mail API now exposes `GET /api/v1/webmail/capabilities` as a production
  webmail bootstrap surface for contract version, module status, list limits,
  supported flags/actions, compose/search limits, attachment upload modes, and
  push-device platforms. Future webmail and Drive module APIs should extend
  this discovery shape instead of forcing frontend hard-coded constants.
- Mail API now exposes `GET /api/v1/mailbox/overview` as a lightweight
  production webmail chrome bootstrap read for aggregate total/unread/starred
  counts, stored-size totals, and system-folder ID shortcuts.
- Mail API message list pagination now accepts optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters for fast
  unread/read/starred/attachment webmail views without forcing clients through
  full-text search.
- Mail API thread list pagination now accepts optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters, with
  `read=false` representing conversations that still contain unread messages.
- Mail API thread list pagination now also accepts `folder_id`, enabling
  folder-scoped conversation views for system and custom folders.
- Mail API message and thread list pagination now accept
  `sort=newest|oldest`, enabling explicit newest-first and oldest-first
  production webmail list controls while retaining opaque cursor pagination.
- Mail API message and thread summaries now expose a required bounded
  `preview` string from the asynchronous search-document read model, so
  production webmail lists can render body context without reading stored EML
  objects during list pagination.
- Mail API now supports bounded `PATCH /api/v1/threads/bulk/flags` for
  conversation-list read/starred/answered/forwarded actions with best-effort
  IMAP flag notifications for the updated messages.
- Mail API now supports bounded `PATCH /api/v1/threads/bulk/folder` for
  conversation-list archive/move workflows with destination-folder validation,
  transactional IMAP UID invalidation, and best-effort expunge notifications.
- Admin API now exposes `GET /admin/v1/console/capabilities` as the operator
  console companion bootstrap surface for module status, common list and
  cleanup/retention limits, tenant/domain/user management, operational triage,
  API usage/export, IMAP UID backfill, admin auth/no-store behavior, and the
  redacted storage backend profile operators need before flipping between
  local/NFS, MinIO, and AWS S3-compatible storage. Next storage work should
  focus on runbook-grade backend migration verification rather than exposing
  secrets or host-local filesystem roots.

### 7. Frontend planning

Before creating or substantially implementing frontend apps, explicitly ask the
user for frontend-specific guidance.

### 8. API metering

Current state:

- Product direction is agreed: collect API usage from the beginning, keep
  billing/rate-limit enforcement policy-driven and off by default.
- A disabled-by-default API metering middleware boundary exists with async,
  fail-open event capture, a `slog` sink, and a durable outbox sink.
- A disabled-by-default `api-metering-worker` can consume `api.usage` events
  from `api.event`, write Postgres daily/monthly aggregates, and serve them
  through the Admin API.
- API usage events include an explicit schema version and deterministic
  `event_id`, preparing future idempotent accounting without making aggregates
  billing-grade yet.
- `api_usage_events` records claimed event IDs before aggregate upserts, so
  replayed usage events do not double-count daily/monthly operational totals.
- New `2026-05-04.api-usage.v2` usage events carry
  tenant/company/domain/user/API-key/principal/auth-source dimensions. The
  idempotency ledger persists those dimensions, and daily/monthly aggregate
  primary keys include them so cross-tenant or cross-principal usage does not
  merge.
- Mail API metering can enrich identity from JWT claims, while Admin API
  metering classifies configured admin-token access through trimmed SHA-256
  digest comparison without coupling `internal/apimeter` to `internal/auth`.
- The worker records immutable `api_usage_ledger` rows before updating
  aggregate read models. Admin API exposes bounded ledger list, NDJSON export,
  and stats endpoints for export sanity checks without making request handling
  synchronous.
- Admin API exposes read-only API usage ledger retention readiness. Operators
  provide a non-future cutoff and optional tenant/principal filters, then
  receive candidate counts plus the covering completed export batch, artifact,
  digest, signature, and late-recorded-row evidence before any future
  archive/delete job can safely run.
- Optional PostgreSQL integration coverage verifies bounded retention runs
  preserve blocked candidates, keep dry-runs read-only, persist run audit rows,
  and delete only the requested ready batch.
- Admin API can list and fetch persisted API usage ledger retention-run audit
  rows after blocked, dry-run, or destructive attempts.
- `api-usage-retention-worker` can run the same bounded retention path on an
  interval or once-and-exit. It is dry-run by default and requires explicit
  `confirm_ready` plus a `remote-ed25519` export manifest signer before
  destructive runs.
- API usage export capabilities advertise retention-run and retention-worker
  support plus the destructive worker remote-key requirement for generated
  operator clients.
- API usage ledger retention rejects future cutoffs at the repository boundary,
  keeping worker/direct-call behavior aligned with the Admin API.
- Admin API can create and list API usage export batch manifests, fetch a saved
  manifest by ID, and replay that manifest window as NDJSON. Batch manifests fix
  the filtered ledger totals used for downstream billing/warehouse jobs.
- Admin API can register and list external export artifacts for each batch,
  including object key, SHA-256, byte count, event count, and metadata. Artifact
  rows are deduplicated per batch by object key and SHA-256.
- Admin API can write a full API usage export batch to local object storage,
  register the resulting artifact metadata idempotently, clean up failed writes
  when the store supports delete, and download or verify stored NDJSON artifacts.
- Admin API can create/list/get canonical export manifest digests and verify a
  stored manifest digest. This gives operators a vendor-neutral integrity check
  over the saved batch plus registered artifact metadata before external
  signing, billing, or warehouse handoff.
- Admin API can create/list/get local-HMAC, local-Ed25519, or remote-Ed25519
  signatures for manifest digests and verify persisted signatures. The signer is
  disabled by default. HMAC uses
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=local-hmac`,
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID`, and
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_SECRET`; Ed25519 uses
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=local-ed25519` plus
  base64 raw Ed25519 private/public key env vars. Remote Ed25519 uses
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=remote-ed25519`,
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_URL`, optional
  `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_TOKEN`, and the base64 raw public
  key. All signers sign the lowercase 64-character manifest digest hex string,
  and remote signatures are verified locally before they are stored.
- Admin API can report API usage export handoff readiness for a saved batch,
  summarizing artifact event coverage, latest manifest digest, latest digest
  signature, operational readiness, and separate billing readiness. Locally
  signed batches remain `billing_ready=false` with
  `production_manifest_signer_required`.
- Passing `deep=true` to the handoff readiness endpoint streams all registered
  artifacts from object storage, verifies their byte/SHA metadata, verifies the
  latest manifest digest, checks that the digest still covers current artifact
  metadata, and verifies the latest signature when a verifier is available.
  Deep mode returns `verified_billing_ready` separately so `billing_ready`
  remains a stable metadata/signer-eligibility signal.
- Manifest signature verification now goes through an
  `ExportManifestSignatureVerifier` interface. Local-HMAC and Ed25519 verifiers
  are wired today; remote Ed25519 lets an external KMS-backed signing service
  plug in without coupling gogomail to a specific vendor SDK. Remote signer
  HTTP responses use the shared bounded drain-and-close cleanup path.
- Admin API exposes API usage export capabilities, including signer backend,
  signer key ID, verifier availability, production signature readiness, and
  billing/verified-billing support flags.
- Mail API now supports bounded `POST /api/v1/threads/bulk/delete` for
  conversation-list delete workflows, soft-deleting every active message in
  selected threads while invalidating IMAP UID rows, decrementing quota
  transactionally, and publishing best-effort expunge events from the
  pre-delete UID snapshot.
- Shared storage now supports `Stat` across local/NFS and S3-compatible
  backends, giving future Drive, lifecycle, and verification paths a portable
  way to inspect object size/metadata without streaming object bodies.
- S3-compatible object metadata returned by `Stat` and `List` is bounded to
  safe single-line UTF-8 values before it crosses the storage adapter boundary,
  keeping downstream Drive, lifecycle, logging, and reconciliation consumers
  from inheriting malformed provider metadata.
- Shared storage now supports `Copy` across local/NFS and S3-compatible
  backends, giving future Drive and lifecycle workflows a portable object
  duplication primitive without forcing caller-side read/write loops.
- Shared storage now supports bounded prefix `List` across local/NFS and
  S3-compatible backends, giving future Drive, lifecycle, and reconciliation
  workflows a portable cursor-paginated object metadata scan.
- Mail API now supports single-message and bounded bulk message restore for
  soft-deleted messages, preserving hierarchical quota checks before restored
  messages become active again.
- Mail API now supports bounded thread-level restore so selected soft-deleted
  conversations can be recovered through the same quota-protected restore path.
- Restore actions now best-effort assign IMAP UIDs and publish `EXISTS` events
  for restored active messages, keeping connected IMAP clients closer to
  webmail recovery state without a separate backfill pass.
- Mail API attachment downloads now expose a bodyless `HEAD` metadata probe,
  returning safe download headers and storage-object size before production
  webmail clients decide to stream attachment bytes.
- Drive copy now supports active files and bounded active folder trees via
  `POST /api/v1/drive/nodes/{id}/copy`, using storage `Copy` for local/NFS,
  MinIO, and S3-compatible backends while advertising a `max_copy_nodes` cap.
- Drive copy cleanup failures are now written to the existing cleanup-failure
  queue when a copied object cannot be deleted after metadata creation fails.
- Copied Drive files keep the destination object path ID and `drive_nodes.id`
  synchronized by passing a preallocated node UUID into the metadata insert.
- Drive file write APIs now return HTTP 507 `insufficient_storage` for quota
  exhaustion on finalize/copy paths.
- Drive node listing now supports webmail/admin `sort=name|updated|created|size`
  controls with folder-first ordering, giving future Drive screens predictable
  production browsing controls without frontend-specific assumptions.
- Drive node listing now supports webmail/admin `node_type=folder|file` filters
  and advertises supported node types through webmail capabilities.
- Webmail Drive node listing now accepts `all_parents=true` for whole-user
  Drive search/list views, and webmail capabilities advertise the mode so
  production compose file pickers can search user Drive inventory without
  crawling folders client-side.
- Drive share-link metadata now exists behind `drive_share_links` plus
  authenticated Mail API create/list/revoke routes. Raw share tokens are
  create-response-only, with persisted hashes/suffixes preparing future public
  resolution and compose-side Drive file insertion.
- Drive share-link public resolution/download is now implemented under the Mail
  API: token paths use SHA-256 hash lookup, revoked/expired/inactive owner or
  node state is rejected, metadata responses omit storage internals, and
  `download`-permission links reuse the Drive no-store, checksum, HEAD, and
  single-range download contract.
- Drive public share-link download OpenAPI now documents `HEAD`, full-body
  `200`, and byte-range `206` as non-JSON binary/header responses, and drift
  tests include the public share routes alongside authenticated Drive
  downloads.
- Drive public share-link metadata and download operations now explicitly opt
  out of global bearer auth in OpenAPI, keeping generated external-recipient
  clients aligned with the unauthenticated runtime public-share boundary.
- Drive public share-link abuse controls now have a configurable Redis
  fixed-window limiter for anonymous metadata/download routes. The limiter
  buckets normalized remote address plus a share-token SHA-256 digest, returns
  429/`Retry-After` when the per-minute quota is exhausted, and keeps limiter
  runtime errors fail-open so storage availability does not become a hidden
  public-download dependency.
- Drive public share-link successful metadata/download accesses, denied
  token/permission checks, and rate-limited requests now write best-effort
  hash-chain audit rows with sanitized link/node/request metadata when
  available plus token suffix, result, status, and remote request metadata.
  This gives Admin audit-log filters immediate visibility into public-link
  access attempts without blocking downloads on audit persistence.
- CalDAV module work has started: ADR 0010 records the standards-first gateway
  boundary, `gogomail --mode=caldav` is a runtime scaffold, and
  `internal/caldavgw` owns RFC/WebDAV method tokens plus principal, calendar
  home, collection, and `.ics` object path parsing.
- CalDAV storage groundwork now includes `caldav_calendars` and
  `caldav_calendar_objects`, with gateway validation for names, colors,
  descriptions, component types, UIDs, strong ETags, sync-token derivation, and
  bounded `.ics` object bodies.
- CalDAV WebDAV XML groundwork now includes bounded namespace-aware PROPFIND
  parsing, safe `Depth` header parsing, `allprop` `include` support, and core
  REPORT root classification for `calendar-query`, `calendar-multiget`,
  `free-busy-query`, and `sync-collection`.
- CalDAV storage tables now have a repository boundary for calendar
  create/list/get and calendar-object upsert/list/get/soft-delete, including
  `.ics` object-name validation, UID/component/ETag checks, optional observed
  ETag guards, and transactional calendar sync-token bumps.
- CalDAV `.ics` validation now wraps `github.com/emersion/go-ical` so object
  writes decode RFC 5545 iCalendar bodies, derive or verify UID/component
  metadata, and reject missing/duplicate `VERSION`/`PRODID` root properties,
  non-`2.0` versions, RFC 4791-forbidden stored `METHOD` properties, multiple
  supported top-level components,
  missing/duplicate UIDs, excessive component/property counts, and RFC-invalid
  `VEVENT`/`VTODO` duration/end combinations before storage. Supported calendar
  components now also reject duplicated singleton time/status properties before
  malformed `.ics` resources can enter repository state.
- CalDAV REPORT `calendar-data` requests now reject unsupported `content-type`
  and non-`2.0` `version` attributes, keeping projection requests aligned with
  advertised `supported-calendar-data`.
- CalDAV WebDAV response groundwork now has a reusable `multistatus` builder
  with per-property `propstat` statuses and discovery properties for
  principals, calendar homes, calendar collections, and calendar objects.
- CalDAV now has an internal `OPTIONS`/`PROPFIND` discovery handler boundary
  with user/path scope enforcement, safe depth handling, DAV capability headers,
  and multistatus responses over a pluggable discovery store.
- CalDAV PostgreSQL repository methods now satisfy that discovery store
  boundary, including active principal lookup and calendar/object adapters for
  the internal `PROPFIND` handler.
- CalDAV Basic authentication groundwork now reuses the Submission
  authenticator, requires TLS/HTTPS-forwarded requests by default, and returns
  authenticated user IDs for future native client-compatible runtime wiring.
- CalDAV runtime configuration now includes `GOGOMAIL_CALDAV_ADDR` and
  `GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH`, with production validation rejecting
  insecure Basic-auth operation.
- `gogomail --mode=caldav` now starts a dedicated HTTP listener backed by the
  CalDAV repository and Basic-auth resolver, with discovery, object I/O, and
  initial REPORT handlers in place.
- CalDAV REPORT parsing now validates `calendar-query`, `calendar-multiget`,
  `free-busy-query`, and `sync-collection` shapes more strictly, including
  nested time-range extraction, required href/filter/range/level fields, and
  bounded sync limits. It also preserves nested RFC 4791 `calendar-data`
  projection requests for `VCALENDAR` and child component property selection.
- CalDAV now handles `REPORT calendar-multiget` for authenticated calendar
  collections, returning requested ETags and `calendar-data` through WebDAV
  multistatus responses. `calendar-multiget`, `calendar-query`, and
  `sync-collection` now project returned iCalendar bodies to requested
  `calendar-data` properties while retaining required RFC 5545 structure
  fields so encoded objects stay valid for clients.
- CalDAV now handles authenticated calendar object `GET`, `HEAD`, `PUT`, and
  `DELETE` with strong ETag headers, bounded iCalendar writes, and
  `If-Match`/`If-None-Match` precondition handling.
- CalDAV now handles `REPORT calendar-query` for authenticated calendar
  collections, returning requested ETags and `calendar-data` while filtering
  VEVENT resources against CalDAV time ranges through the RFC 5545 parser.
  Calendar-query object scans now use bounded `limit/nresults` handling with a
  one-extra-row truncation probe, rejecting partial result sets until
  continuation semantics exist. Unsupported CalDAV query filter elements now
  return a `CALDAV:supported-filter` precondition instead of being silently
  skipped, keeping unimplemented predicates from broadening result sets.
- CalDAV now handles conservative `REPORT sync-collection` requests for
  authenticated calendar collections: initial sync returns active objects plus
  a top-level sync token, current tokens return no resource changes,
  stale-but-known tokens return deltas/tombstones, unknown or expired tokens
  return a DAV `valid-sync-token` error, and truncating limits are rejected
  until continuation semantics are implemented.
- CalDAV now handles `REPORT free-busy-query` on authenticated calendar
  collections, returning RFC-shaped `text/calendar` `VFREEBUSY` responses for
  `Depth: 1` child VEVENTs while bounding child object scans with
  `limit/nresults` and a one-extra-row truncation probe. It clips to the
  requested UTC time range, skips transparent/cancelled events, maps tentative
  events to `BUSY-TENTATIVE`, ingests stored VFREEBUSY `FREEBUSY` source
  periods, and coalesces same-type overlaps.
- CalDAV now handles `MKCALENDAR` on authenticated calendar collection paths
  with UUID Request-URI segments. Creation XML is bounded and namespace-aware
  for display name, description, and CalendarServer/Apple calendar color, and
  successful creates return `201 Created` plus `Location`; slug-style path
  aliases remain future compatibility work.
- CalDAV now handles `DELETE` on authenticated calendar collection paths,
  deleting the collection and active child objects through one repository
  transaction while keeping calendar-home and cross-user deletes forbidden.
- CalDAV now records durable sync-change rows for calendar creation and object
  upsert/delete paths, allowing `REPORT sync-collection` to answer
  stale-but-known tokens with object updates and response-level 404 tombstones
  instead of always forcing a full resync. Collection-deleted tokens can now
  return a final top-level sync token even after the calendar row is gone.
- CalDAV now supports RFC 6764-style service discovery: `/.well-known/caldav`
  redirects to `/caldav/`, and authenticated root `PROPFIND` exposes the
  service root as a read-only collection discovery anchor with
  `current-user-principal` and `principal-collection-set`. Principal-only
  properties such as `calendar-home-set` remain on the principal resource so
  clients do not mistake the service root for an authenticated user principal.
- CalDAV `OPTIONS` and unsupported-method responses now use one implemented
  method list for `Allow`, keeping future-only method names such as `COPY` and
  `MOVE` hidden until their WebDAV behavior is actually implemented. `OPTIONS`
  discovery now also emits `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`, and 405 method-probe responses carry the
  same safety headers for native-client capability probing.
- CalDAV `PROPFIND /caldav/principals/` now resolves the advertised principal
  collection path, returning collection metadata at `Depth: 0` and the
  authenticated principal as a `Depth: 1` child without exposing other users.
- CalDAV discovery converts shared Directory principals into CalDAV principals
  only when the Directory kind is `user`; organization, group, and resource
  principals stay gated behind future delegation/resource-booking semantics
  instead of being modeled as personal calendar homes.
- CalDAV calendar-home `PROPFIND` now reports WebDAV `current-user-principal`
  and `owner` as the canonical principal URL, preserving a clean boundary for
  future delegated/shared calendar access instead of treating the home
  collection as the principal.
- CalDAV `PROPFIND` now exposes RFC 3744-shaped current-user privilege sets for
  the operations already implemented: read-only principals, calendar-home
  calendar bind/unbind, collection object bind/unbind plus `PROPPATCH`
  metadata writes, and object content writes. ACL and broader delegation
  privileges remain unadvertised until their semantics exist.
- Directory/Identity now includes a bounded `SearchPrincipals` repository
  boundary over users, organizations, groups, and resources. It validates
  tenant/domain/organization scope, permitted principal kinds, query length,
  and result limits, and escapes `LIKE` wildcards before querying. Future
  CalDAV attendee/resource resolution, Contacts/CardDAV autocomplete, shared
  inbox targeting, and admin consoles should use this boundary instead of
  product-local principal lookup.
- CalDAV now handles WebDAV `PROPPATCH` on authenticated calendar collections
  for display name, description, and CalendarServer/Apple calendar color.
  The parser is bounded and namespace-aware, optional properties can be
  removed, `displayname` cannot be removed, and the repository records a
  transactional `collection-updated` sync marker instead of hiding metadata
  changes from WebDAV sync state.
- CalDAV collection discovery now returns WebDAV `supported-report-set` for
  implemented REPORT handlers only: `calendar-query`, `calendar-multiget`,
  `free-busy-query`, and `sync-collection`. Scheduling and other future reports
  remain unadvertised until their semantics exist. Calendar collection
  `PROPFIND Depth: 1` child object discovery now uses the shared bounded
  one-extra-row probe and rejects truncating listings instead of silently
  returning partial metadata.
- CalDAV `calendar-query` now honors simple top-level component filters such as
  `VEVENT` and `VTODO` using stored `component_type` metadata, avoiding
  unrelated object types and avoiding a full iCalendar reparse before component
  filtering.
- CalDAV `calendar-multiget` now respects request-resource scope: collection
  requests cannot fetch sibling collection objects, while calendar-home
  requests remain able to resolve authenticated same-user object hrefs.
- CalDAV `PROPFIND` now exposes WebDAV `owner`, `creationdate`, and
  `getlastmodified` for calendar collections and objects when backed by stored
  metadata, improving native-client discovery without inventing ACL/delegation
  semantics.
- CalDAV object `GET`/`HEAD` now support `If-None-Match` cache revalidation
  against strong object ETags, returning `304 Not Modified` without streaming
  `.ics` bodies when possible.
- CalDAV object `PUT` now rejects explicit non-`text/calendar` media types,
  repeated `Content-Type` headers, and non-`2.0` `text/calendar` version
  parameters before iCalendar parsing, while still allowing clients that omit
  `Content-Type`.
- CalDAV object `PUT` now enforces `If-Match: *` as an existing-resource
  precondition, preventing accidental object creation through conditional
  overwrite requests.
- CalDAV object `PUT` now rejects stale specific `If-Match` values and matching
  specific `If-None-Match` values before reading/parsing the request body.
- CalDAV object `GET`/`HEAD` now reject stale `If-Match` preconditions before
  cache revalidation, and object `DELETE` now accepts comma-listed strong
  `If-Match` ETags for better WebDAV client compatibility.
- CalDAV object `DELETE` now enforces `If-Match: *` as an existing-resource
  precondition, returning HTTP 412 for missing resources.
- CalDAV object `GET`/`HEAD` now emit `Last-Modified` and honor
  `If-Modified-Since` revalidation from stored object update timestamps.
- CalDAV object `PUT`/`DELETE` now honor `If-Unmodified-Since` before body
  reads or repository mutation, returning HTTP 412 for stale timestamp
  preconditions.
- S3-compatible `GetRange` now bounds returned readers to the validated
  requested length even when a provider sends an oversized `206 Partial
  Content` body, matching local/NFS range-read behavior.
- CalDAV object `GET`/`HEAD` now honor `If-Unmodified-Since` before cache
  revalidation, returning HTTP 412 for stale timestamp read preconditions.
- S3-compatible `GetRange` now validates that `Content-Range` matches the
  requested byte window before returning the bounded response reader.
- S3-compatible `GetRange` now reports `io.ErrUnexpectedEOF` when a matching
  partial response body ends before the requested byte count.
- S3-compatible `GetRange` now drains a small bounded remainder on successful
  range-reader close so oversized partial responses can still reuse HTTP
  connections without exposing extra bytes to callers.
- S3-compatible `GetRange` now also drains a small bounded remainder when
  callers close before consuming the requested range, helping preview/cancel
  paths reuse HTTP connections.
- IMAP `STATUS`/LIST-STATUS parsing now rejects duplicate status data items
  and duplicated LIST-STATUS `STATUS` return options before mailbox metadata
  lookup, and LIST-STATUS now preserves specific status-return diagnostics
  instead of collapsing malformed status return options into generic LIST arity
  errors.
- CalDAV `MKCALENDAR` now rejects non-UUID creation path IDs before reading
  the XML request body when no active collection already exists at that path.
- CalDAV collection `DELETE` now honors `If-Unmodified-Since` and
  `If-Match: *` preconditions before deleting a calendar collection and its
  children, and strong collection ETags derived from sync state are advertised
  through discovery so specific `If-Match` values can protect stale clients.
- CalDAV collection `PROPPATCH` now shares that precondition gate, rejecting
  stale metadata updates and mismatched collection ETag conditions before XML
  request bodies are read or parsed.
- CalDAV `REPORT` now rejects malformed Depth headers and `Depth: infinity`
  before reading XML request bodies, keeping unsupported WebDAV traversal
  semantics out of calendar-query, calendar-multiget, sync-collection, and
  free-busy-query work.
- CalDAV `REPORT` and `PROPFIND` also reject repeated HTTP `Depth` headers
  before XML body parsing, keeping traversal scope deterministic before
  discovery, sync, and query execution.
- CalDAV object and collection preconditions evaluate repeated `If-Match` and
  `If-None-Match` headers as a single ETag list, aligning cache validation and
  write guards with HTTP field-combination semantics.
- CalDAV date-based conditional headers now fail closed when
  `If-Modified-Since` or `If-Unmodified-Since` is repeated on object reads,
  object writes, object deletes, or collection precondition checks, avoiding
  ambiguous timestamp guards across clients and intermediaries.
- CalDAV object `DELETE` now passes a matched strong `If-Match` ETag through
  `DeleteObjectRequest` and rechecks it inside the repository transaction,
  matching the existing `PUT` observed-ETag guard and reducing stale-delete
  races.
- CalDAV object `PUT`/`DELETE` now also pass the looked-up strong ETag through
  repository observed-ETag guards for `If-Match: *`, so existence-only
  preconditions still protect the exact object version seen by the handler.
- CalDAV `calendar-multiget` now accepts HTTP(S) absolute URI hrefs by
  normalizing the URI path through the existing CalDAV path parser and
  preserving same-user / same-collection scope checks; userinfo-bearing
  authorities, query, fragment, opaque, non-HTTP(S), and unsafe hrefs stay
  rejected as per-resource misses.
- CalDAV `REPORT sync-collection` now enforces HTTP `Depth: 0` before sync
  lookup or change-log work, matching the RFC 6578 request-scope model and the
  existing CardDAV behavior while leaving child traversal to the required
  request-body `sync-level`.
- CalDAV `REPORT sync-collection` now requires the request body to carry an
  explicit `DAV:sync-token` element, accepting an empty element for initial
  sync but rejecting omitted sync-token anchors before repository work.
- CalDAV stale-token `sync-collection` delta reads now fetch one extra
  change-log row behind bounded `limit/nresults`, allowing exact-limit deltas
  to succeed while still rejecting responses that would genuinely truncate.
- CalDAV sync-change retention now has repository groundwork:
  `PruneCalendarSyncChanges` can dry-run or delete bounded old change-log rows
  while preserving the newest marker per calendar, backed by a prune-order
  migration index. Next, wire an operator/worker path with an explicit
  retention-age policy before treating token expiry as production-ready.
- CalDAV sync-change writes now also enqueue transactional `dav.event`
  outbox rows with `calendar.changed` payloads. Those payloads now preserve
  owner user, actor user, and delegated-vs-direct context for delegated
  writes/deletes, so future Notification & Sync, audit, reminder, device,
  mobile delta, and search/index consumers can reason about who changed whose
  calendar without coupling push delivery into the CalDAV protocol gateway.
- `event-worker` now registers DAV change audit handlers. Operators can run a
  dedicated worker instance with `GOGOMAIL_EVENT_STREAM=dav.event` to validate
  `calendar.changed`/`contacts.changed` payloads and persist audit rows while
  later Notification & Sync consumers are developed.
- CalDAV initial `sync-collection` snapshots now also fetch one extra calendar
  object through a sync-specific repository list path, so omitted or exact
  `limit/nresults` requests cannot silently return a partial snapshot with the
  current collection sync token.
- CalDAV `REPORT calendar-query` now honors HTTP `Depth: 0` by returning no
  child calendar-object matches for collection-scoped queries unless clients
  explicitly send `Depth: 1`, keeping WebDAV request scope from silently
  widening during event searches.
- CalDAV `calendar-query` filter parsing now enforces RFC 4791-shaped
  component filter grammar by requiring `comp-filter name` and a top-level
  `VCALENDAR` filter, and by rejecting missing or repeated top-level component
  filters, preventing malformed native-client queries from being treated as
  broad whole-calendar searches. It also rejects `time-range` elements placed
  directly under `filter` and duplicate `time-range` elements inside the same
  component filter, keeping range matching semantics explicit.
- CalDAV `calendar-query` and `free-busy-query` now evaluate bounded VEVENT
  recurrence sets through the RFC 5545 parser, including `RRULE`, `EXDATE`,
  and `RDATE` support from the shared iCalendar library. Dense or unbounded
  rules are capped per object so native-client time-range scans cannot turn
  one stored event into unbounded gateway work.
- CalDAV `calendar-query` now evaluates VTODO time-range filters with RFC 4791
  `DTSTART`, `DUE`, `DURATION`, `COMPLETED`, and `CREATED` overlap rules,
  including effective `DUE = DTSTART + DURATION`, so advertised VTODO support
  no longer silently drops matching tasks from native-client sync results.
- CalDAV iCalendar object validation now accepts one VEVENT master plus
  same-UID `RECURRENCE-ID` detached override VEVENTs in a single stored object.
  `calendar-query` and `free-busy-query` scan all VEVENTs in the object and
  suppress the replaced master occurrence when an override is present, matching
  common RFC 5545 recurring-event client output more closely.
- Admin Drive node listing now accepts `all_parents=true` for whole-user Drive
  search/list views while rejecting ambiguous `parent_id` combinations.
- Drive file finalize, upload-session cleanup/retry-body replacement,
  permanent-delete cleanup, cleanup-failure retry, download, and copy paths
  enforce the owning user's
  `drive/users/{user_id}/...` object prefix before storage adapter access.

Next:

- Keep CalDAV in an experimental/backend-only release tier until client-ready
  gates are closed: broader recurrence edge cases (complete), production sync-token
  retention-age policy (complete - dav-sync-retention-worker with
  GOGOMAIL_DAV_SYNC_RETENTION_CUTOFF_AGE=2160h default), ADR 0014 slug/alias
  implementation   (complete), ADR 0015 timezone time-range interpretation
  (complete), VTODO recurrence expansion (RRULE on VTODO, complete), scheduling
  semantics (complete - RFC 6047 iMIP with ATTENDEE/ORGANIZER extraction, iTIP
  message building, DeliveryQueue wiring to mail.outbound.general, and attendee
  resolution via Directory + CardDAV with internal-user/directory-alias/carddav-contact/external
  classification),
  and broader Apple/Android/Windows/macOS compatibility tests.
- Keep frontend implementation behind the explicit start gate, but preserve
  the product target in backend/API planning: future Next.js TypeScript +
  shadcn/ui webmail, Drive, calendar, contacts, admin console, and shared inbox
  surfaces should follow `DESIGN.md` and a Notion Mail-like workflow model,
  with mobile mail/Drive/calendar/contacts/push/offline sync and desktop
  power-user flows planned as first-class clients.
- Before public shared/delegated calendar or resource-booking features,
  extend the initial `internal/directory` user/organization/group/resource
  principal resolver plus alias lookup and group/alias/resource schema into the
  platform boundaries CalDAV depends on: Directory/Identity for delegated
  relationships, resource booking policy, and principal resolution;
  Contacts/CardDAV for personal/external
  people and address books;
  Notification & Sync for reminders, devices, quiet hours, and delta fan-out;
  Search for unified event/person/resource lookup; and Policy/Audit for
  retention, admin controls, and traceable calendar access.
- Directory/Identity now has the first company-scoped delegation table and
  repository check boundary for owner/delegate principals, product scopes, and
  `read`/`write`/`manage` role hierarchy. CalDAV runtime authorization now has
  a first integration through explicit Directory/accesspolicy decisions and
  shared audit insertion, allowing cross-user calendar paths to be resolved
  against the owner store only when the delegated role check allows it.
  Delegated CalDAV `PROPFIND` now derives WebDAV
  `current-user-privilege-set` from that same decision path so discovery stays
  consistent with enforced access. Delegated CalDAV REPORT and sync responses
  now use the same privilege shaping for calendar-object properties, and
  missing Directory principals fail closed as authorization denial. The policy
  boundary also rejects resolved non-user owner/actor principals before
  delegation checks or audit insertion. Next CalDAV sharing work should add
  native-client compatibility coverage and write/manage UX semantics before
  public shared calendars are advertised.
- Effective delegation now has a bounded group-expansion read boundary. Next
  product-module integration should still remain deliberate: CalDAV/CardDAV,
  Drive, mailbox sharing, and admin APIs should consume it through explicit
  policy/audit adapters and WebDAV privilege semantics instead of directly
  branching on directory rows in protocol handlers.
- Direct and effective delegation checks now both require active owner and
  delegate principals when `ActiveOnly` is set, keeping policy callers
  fail-closed after a shared calendar, contacts, Drive, or mailbox endpoint
  principal is suspended or deleted.
- CardDAV now has that first deliberate integration for delegated contacts
  access. The gateway distinguishes actor and address-book owner, resolves
  allowed cross-user read/write/manage requests against the owner store through
  the `contacts` Directory delegation scope, records delegated access through
  the shared accesspolicy/audit path, and derives delegated PROPFIND
  `current-user-privilege-set` from the same decision. Resolved non-user
  owner/actor principals are rejected before delegation checks or audit
  insertion. Keep this experimental: next Contacts/CardDAV sharing work should
  add native-client shared address-book coverage, admin/product sharing
  semantics, and autocomplete linkage before public CardDAV sharing is
  advertised.
- Directory/Identity now also has a bounded delegation listing boundary for
  owner/delegate/scope/role-filtered inspection. This was prioritized before
  deeper CalDAV sharing semantics because admin consoles, shared calendar
  management, Drive shares, shared inboxes, and Contacts/CardDAV delegation all
  need the same observable relationship read model. Next API work should expose
  it through contract-first admin endpoints rather than letting products query
  `directory_delegations` directly.
- That first contract-first admin endpoint now exists as
  `GET /admin/v1/directory/delegations`.
- Audited delegation creation now exists as
  `POST /admin/v1/directory/delegations`, backed by
  `CreateDelegationWithAudit`. It validates active same-company owner/delegate
  principals, scope, role, and self-delegation before inserting the grant and
  `directory_delegation.create` audit row in one transaction.
- Audited delegation deletion now exists as
  `DELETE /admin/v1/directory/delegations/{id}`, backed by
  `DeleteDelegationWithAudit`, so admins can revoke grants with a
  transaction-coupled `directory_delegation.delete` audit row.
- Audited delegation role updates now exist as
  `PATCH /admin/v1/directory/delegations/{id}/role`, backed by
  `UpdateDelegationRoleWithAudit`. The endpoint changes active grants under
  active companies while committing `directory_delegation.role_update` with
  previous/new role detail.
- Audited delegation reassignment now exists as
  `PATCH /admin/v1/directory/delegations/{id}/assignment`, backed by
  `ReassignDelegationWithAudit`. The endpoint moves an active grant to a new
  owner/delegate/scope while preserving its role, validates active same-company
  new principals, and commits `directory_delegation.reassign`. Next delegation
  work should connect these admin lifecycle boundaries into product-facing
  CalDAV, Drive, Contacts/CardDAV, and shared inbox policy checks only after
  each module has its protocol-specific semantics and UX gate.
- Audited group membership creation now exists as
  `POST /admin/v1/directory/group-memberships`, backed by
  `CreateGroupMembershipWithAudit`. It validates active same-company group and
  member principals, role, self-membership, and nested group cycles before
  inserting the membership and `directory_group_membership.create` audit row in
  one transaction.
- Directory group membership listing now exists as
  `GET /admin/v1/directory/group-memberships`, backed by
  `ListGroupMemberships`, so operators and the future admin console can inspect
  company-scoped group-backed access without querying Directory tables
  directly.
- Audited group membership deletion now exists as
  `DELETE /admin/v1/directory/group-memberships/{id}`, backed by
  `DeleteGroupMembershipWithAudit`, so group-backed delegation can be revoked
  with a transaction-coupled `directory_group_membership.delete` audit row.
- Audited group membership role updates now exist as
  `PATCH /admin/v1/directory/group-memberships/{id}/role`, backed by
  `UpdateGroupMembershipRoleWithAudit`, so operators can promote or demote
  group-backed access without delete/recreate churn.
- Audited group membership reassignment now exists as
  `PATCH /admin/v1/directory/group-memberships/{id}/assignment`, backed by
  `ReassignGroupMembershipWithAudit`, so operators can move active memberships
  between group/member assignments while preserving role and audit continuity.
  Next group-membership work should focus on product-facing policy integration,
  not raw membership table access.
- Directory principal search is also exposed through
  `GET /admin/v1/directory/principals`. Future attendee/resource lookup,
  Contacts/CardDAV autocomplete, Drive sharing, shared inbox targeting, and
  admin console screens should reuse this contract or the underlying
  `SearchPrincipals` boundary rather than adding product-local principal search
  semantics.
- Directory alias resolution is exposed through
  `GET /admin/v1/directory/aliases/resolve`. Future mail routing diagnostics,
  attendee resolution, shared inbox targeting, and admin console screens should
  use this address-to-principal contract instead of re-parsing addresses or
  querying `directory_aliases` directly.
- Directory alias listing now has a bounded repository boundary and admin API
  read path, but product modules should keep using `ListAliases`/`ResolveAlias`
  instead of reaching into `directory_aliases` directly.
- That admin API read path now exists as `GET /admin/v1/directory/aliases`.
- Directory alias creation now has an audited repository mutation boundary and
  `POST /admin/v1/directory/aliases` admin API. It normalizes addresses,
  requires active domain scope, enforces alias-domain alignment, verifies an
  active same-company target principal, returns a predictable duplicate-alias
  error on the active-address unique index, and records
  `directory_alias.create` in the same transaction.
- Directory alias deletion now exists as
  `DELETE /admin/v1/directory/aliases/{id}` and records
  `directory_alias.delete` in the same transaction as the soft delete. Next
  alias work should add update/reassign flows only with the same
  transaction-audited policy shape and without turning this into a product-local
  shared-inbox CRUD model.
- The first `internal/accesspolicy` adapter wraps Directory effective
  delegation into a normalized allow/deny decision. Next integrations should
  add product-specific policy/audit adapters around it before exposing shared
  calendars, delegated address books, Drive shares, or shared inbox actions.
  For WebDAV protocols, use its RFC 4918 privilege mapper instead of inventing
  per-module role-to-privilege tables. Audit integrations should use its
  delegated-access audit detail builder so logs carry normalized principal,
  role, decision, and privilege fields without free-form reason cardinality,
  or its delegated-access audit log builder when they need the full standard
  audit envelope. Admin audit-log queries now support `actor_id` and
  `target_id` filters; future delegated-sharing diagnostics should use those
  filters instead of adding product-local audit tables. When product modules
  start authorizing delegated decisions, prefer `DelegatedAccessAuthorizer` so
  the effective-delegation check and audit insertion remain one fail-closed
  operation; use `DelegationAuditRecorder` only when a product boundary has
  already made and preserved the decision separately.
- CalDAV principal discovery now exposes Directory primary email addresses and
  active user-targeted Directory aliases via RFC 4791
  `calendar-user-address-set` normalized `mailto:` hrefs when present. Keep the
  next scheduling work on this standards-shaped principal/address boundary:
  attendee resolution should connect through Directory plus Contacts/CardDAV,
  and public scheduling/resource booking should wait for explicit policy and
  audit decisions.
- Delegated CalDAV/CardDAV discovery must keep actor and owner identity
  separate: `DAV:current-user-principal` is the authenticated actor, while
  resource hrefs, `DAV:owner`, and storage lookup remain owner-scoped. Future
  shared calendar/address-book, resource booking, and delegated UI/API work
  should preserve that boundary instead of rewriting the client identity to the
  owner.
- Continue Contacts/CardDAV as a standards-first module: the current
  `internal/carddavgw` path/href, storage metadata, address-book/contact
  repository, bounded vCard 3.0/4.0 semantic validation, REPORT parsing,
  multistatus rendering, and internal `OPTIONS`/`PROPFIND` discovery handler
  now includes internal `addressbook-query`, `addressbook-multiget`, and
  `sync-collection` execution plus contact-object `GET`, `HEAD`, `PUT`, and
  `DELETE` semantics, and `gogomail --mode=carddav` now exposes an
  experimental Basic-auth runtime listener. The vCard parser now recognizes
  unquoted value separators, preserving quoted parameter values that contain
  colons. Contact-object `PUT` rejects repeated `Content-Type` headers before
  vCard media parsing. It also evaluates
  `/carddav/principals/` as the advertised principal collection, returning the
  authenticated principal at `Depth: 1` without listing unrelated users, and
  `addressbook-query` filters over parsed unfolded vCard property values,
  including RFC 6352 match-type, `negate-condition`, default
  `i;unicode-casemap`, nested `param-filter`, and `test=anyof|allof`
  composition for top-level filters and prop-filters. Unsupported vCard
  property or parameter filters now fail with the RFC 6352
  `CARDDAV:supported-filter` precondition instead of misleading empty success
  responses, including `Depth: 0` requests that otherwise return no child
  objects. Unsupported CardDAV filter child elements now use the same
  `CARDDAV:supported-filter` precondition. Duplicate `text-match` elements
  inside one `prop-filter` or `param-filter` are rejected at parse time,
  preserving RFC 6352's singular text-match grammar. REPORT `address-data` can also
  project returned vCards to
  requested property names and rejects unsupported requested address-data
  content types or versions with the RFC 6352
  `CARDDAV:supported-address-data` precondition. Unsupported text-match
  collations now fail with `CARDDAV:supported-collation`; address-book
  collections advertise `CARDDAV:supported-collation-set` with working
  `i;ascii-casemap` and `i;unicode-casemap` matching. Capability properties
  that are not allprop-friendly remain available through explicit `prop`,
  `include`, and `propname`; returned
  address-data also carries explicit `text/vcard` attributes matching the
  stored vCard version. Contact-object `PUT` accepts explicit `text/vcard`
  version parameters only for 3.0/4.0 and requires the media-type version to
  match the body `VERSION` before mutating storage.
  `addressbook-multiget` requires an explicit `Depth` header before resolving
  requested hrefs, while accepting common Depth 0/1 client shapes.
  `addressbook-query` execution honors bounded `limit/nresults` response caps.
  CardDAV `OPTIONS` and unsupported-method responses share one implemented
  method list plus no-store/nosniff safety headers, keeping native contact
  clients from seeing or caching methods before handler semantics exist.
  Future `COPY`/`MOVE` method names are explicit constants but remain
  regression-covered as unadvertised until full WebDAV semantics are added.
  Repository-backed query execution can stream contact objects and stop once
  the response cap is satisfied, avoiding whole-address-book materialization on
  that hot path. Address-data projection failures are explicit errors rather
  than silent full-body fallbacks. RFC 6352 `addressbook-query` now requires an
  explicit `Depth` header; `Depth: 1` scans address-object children,
  `Depth: 0` remains collection-scoped without returning child objects, and
  `Depth: infinity` is rejected before XML body parsing.
  CardDAV `REPORT` and `PROPFIND` reject repeated HTTP `Depth` headers before
  XML body parsing so address-book traversal scope cannot depend on header
  merge behavior in clients or intermediaries.
  CardDAV object and collection preconditions evaluate repeated `If-Match` and
  `If-None-Match` headers as a single ETag list, preserving conditional request
  semantics for contact writes, deletes, cache validation, and collection
  metadata mutations. Date-based CardDAV conditionals now reject repeated
  `If-Modified-Since` or `If-Unmodified-Since` headers before object reads,
  object writes, object deletes, and collection precondition checks, matching
  the CalDAV fail-closed boundary for ambiguous timestamp guards.
  PROPFIND responses now expose conservative
  RFC 3744-style current-user privileges, advertising `DAV:read` broadly and
  `DAV:bind`/`DAV:unbind` only on address-book homes where extended `MKCOL`
  can create child collections and collection `DELETE` can remove them, and on
  address-book collections where contact-object `PUT`/`DELETE` can bind or
  unbind child `.vcf` members. Collections also advertise `DAV:write-properties`
  only with implemented `PROPPATCH`, plus `DAV:write-content` only on contact objects with
  implemented write paths.
  Address-book collections also expose CalendarServer-compatible `getctag`
  from the same durable sync token used for WebDAV `sync-token`, keeping
  legacy change detection and RFC 6578 sync anchored to one collection version.
  Address-book collection `PROPFIND Depth: 1` child-object discovery now uses
  the shared bounded one-extra-row probe and rejects truncating listings instead
  of silently returning partial contact metadata.
  RFC 6352 `addressbook-description` is now returned from stored address-book
  metadata. WebDAV `PROPPATCH` now updates authenticated address-book
  collection `DAV:displayname` and `addressbook-description` through a bounded
  parser and repository mutation that refreshes sync state and appends an
  `addressbook-updated` change row. Collection ETags are derived from the
  durable sync token, exposed through PROPFIND `getetag`, and used with
  `If-Match`/`If-Unmodified-Since` to reject stale `PROPPATCH` requests before
  body reads. RFC 6352-style extended `MKCOL` now creates authenticated
  address-book collections at UUID request-URI paths after validating
  `DAV:resourcetype`, `DAV:displayname`, and `addressbook-description`, while
  rejecting existing collections, cross-user paths, missing homes, and unsafe
  path ids before body reads where possible.
  Address-book collection `DELETE` soft-deletes the collection and active child
  contact objects transactionally, honors collection preconditions, and records
  an `addressbook-deleted` change row. `sync-collection` can now return the
  latest deletion sync token for stale-token requests even after the collection
  is no longer active, and enforces RFC 6578 Depth behavior by rejecting
  `Depth: 1` sync requests before sync work. It also distinguishes empty
  initial `DAV:sync-token` elements from missing token elements and rejects the
  latter before sync work. Stale-token delta reads fetch one extra change-log
  row behind bounded `limit/nresults`, matching the CalDAV exact-limit
  behavior while still rejecting genuinely truncating deltas. Initial
  `sync-collection` snapshots use the same one-extra-object probe through a
  sync-specific repository list path, avoiding silent partial address-book
  snapshots when the generic list default would otherwise cap results.
  CardDAV sync-change retention now has repository groundwork:
  `PruneAddressBookChanges` can dry-run or delete bounded old change rows while
  preserving the newest marker per address book, backed by a prune-order
  migration index. The `dav-sync-retention-worker` now runs the CalDAV and
  CardDAV prune paths together on an interval or once-and-exit, dry-run by
  default and guarded by explicit confirmation before destructive runs. Worker
  executions now persist `dav_sync_retention_runs` rows with cutoff, bounded
  counts, dry-run/confirmation flags, status, and sanitized failure text, so
  partial CalDAV/CardDAV retention failures are auditable. The retention
  repository can list bounded run history by status/created-at window and fetch
  one run by bounded ID, and Admin API now exposes that history at
  `/admin/v1/dav-sync/retention-runs` plus detail reads by run ID. Admin API now
  also exposes `/admin/v1/dav-sync/retention-readiness` as a non-destructive
  dry-run preview over CalDAV/CardDAV sync-change candidates with non-future
  cutoff validation, bounded per-backend probe limits, truncation signaling,
  and a `destructive_guarded` flag. Admin API now has an explicitly confirmed
  run boundary at `/admin/v1/dav-sync/retention-runs`: dry-run calls persist
  preview audit rows, and destructive calls require `confirm_ready=true` plus a
  non-truncated readiness preview before CalDAV/CardDAV prune calls are made.
  CardDAV address-book change writes now also enqueue transactional
  `dav.event` outbox rows with `contacts.changed` payloads, and those payloads
  preserve owner user, actor user, and delegated-vs-direct context for
  address-book and contact-object mutations. Future Contacts, autocomplete,
  mobile sync, and notification workers should consume that event boundary
  rather than querying CardDAV change tables from product code.
  Next, choose a production retention-age policy and run native-client expiry
  compatibility tests before treating DAV token expiry as client-ready.
  CalDAV calendar-object `PUT` now rejects duplicate active iCalendar UIDs
  within the same calendar before the SQL upsert path, keeping repository
  errors predictable while the PostgreSQL partial unique index remains the
  final concurrency guard. Final unique-index races are mapped back to stable
  duplicate UID/name repository errors instead of surfacing raw driver messages.
  CalDAV `DELETE` now uses the same default authenticated user resolver
  fallback as object `GET`/`PUT` and WebDAV discovery/report methods, so
  manually assembled gateway handlers stay predictable and do not panic when a
  custom resolver is not injected.
  Contact-object `PUT` now rejects duplicate active vCard UIDs within the same
  address book before the SQL upsert path, keeping
  repository errors predictable while the PostgreSQL partial unique index
  remains the final concurrency guard. Final unique-index races are mapped back
  to stable duplicate UID/name repository errors instead of surfacing raw driver
  messages. Contact-object `DELETE` now passes observed strong ETags into the
  repository transaction so `If-Match` deletes are rechecked under the
  address-book lock before row removal.
  Contact-object `PUT`/`DELETE` now also carry observed strong ETags for
  successful `If-Match: *` preconditions, keeping existence-only contact
  mutations guarded against stale handler preflight.
  Delegated contacts access now uses the Directory/accesspolicy/audit boundary
  instead of a CardDAV-local sharing model: cross-user `GET`, `PUT`, `DELETE`,
  `MKCOL`, `PROPPATCH`, `REPORT`, and `PROPFIND` requests require the matching
  contacts read/write/manage role, run against the owner address-book store,
  keep `DAV:current-user-principal` actor-scoped, and expose delegated WebDAV
  privileges in discovery and REPORT responses. Directory-to-CardDAV principal
  conversion now explicitly rejects non-user principals, preserving the product
  boundary that CardDAV is user-owned contacts while organization, group, and
  resource principals wait for deliberate semantics.
  It should be followed by broader vCard compatibility and native-client
  compatibility tests before any public contacts UI or API treats it as
  production-ready.
- Admin audit-log listing now supports bounded `action_prefix` filters, giving
  operators a contract-level way to inspect action families such as
  `share_link.` across successful, denied, and rate-limited public-link
  activity before a dedicated aggregate dashboard exists. Next public-link work
  should add aggregate activity views and configurable tenant policy for whether
  `view` links can preview content beyond metadata before broad public rollout.
- Add a concrete cloud KMS adapter, or deploy the remote-Ed25519 signer service,
  before invoices or hard Open API limits depend on completed export batches.
- Keep scheduled API usage retention dry-run in pre-production until production
  export storage and signer policy are settled.
- Avoid synchronous writes on hot API paths.

## Do not do yet

- Do not start frontend implementation without asking the user.
- Do not build a built-in spam engine inside SMTP core.
- Do not add vendor-specific spam/filtering behavior to protocol paths.
- Do not advertise SMTP extensions before full RFC semantics exist.

## Standard finish checklist

```bash
go test ./...
go mod tidy -diff
git status --short
git push
```

Every meaningful feature should be a reviewable commit before pushing.
