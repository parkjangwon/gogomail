# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-027
- **제목**: DNSBL/RBL 통합 (Phase 5-B) — RFC 5782 DNS 기반 차단 목록
- **배경**: Phase 5-B 항목. RCPT TO 단계 이전에 송신 IP를 DNS 블랙리스트(DNSBL)에 조회하여
  스팸/악성 메일 서버를 차단한다. RFC 5782 §2.2 반환 코드 해석 포함.
  기본 비활성화 (`GOGOMAIL_DNSBL_ZONES=""`) — 운영자가 zone 목록을 설정해야 활성화.
- **구현 대상**:
  - `internal/dnsbl/dnsbl.go` — DNSBL 조회 패키지
    - `Checker` struct: zone 목록 + net.Resolver
    - `Check(ctx, ip) (Result, error)` — reversed-IP A 레코드 조회
    - `Result`: Listed bool, Zone, ReturnCode
  - `internal/dnsbl/dnsbl_test.go` — 테스트 (fake resolver 사용)
  - `internal/dnsbl/hook.go` — smtpd.Hook 어댑터 (StageAuthenticationChecked에서 실행)
  - `internal/config/config.go` — `DNSBLZones`, `DNSBLTimeout`, `DNSBLPolicy` 추가
  - `internal/app/run.go` — SMTP 수신 파이프라인에 DNSBL 훅 조건부 등록
- **완료 조건**:
  - [x] `go test ./internal/dnsbl/...` 통과
  - [x] listed IP → SMTP 거부, unlisted IP → 통과
  - [x] `go test ./...` 통과
- **다음 태스크**: TASK-028

---

## 완료됨

- **ID**: TASK-023
- **제목**: Well-Known URIs (RFC 6764) — CalDAV/CardDAV 자동발견
- **배경**: Phase 4-B 항목. Apple Mail, iOS, macOS, Thunderbird는 `/.well-known/caldav`와
  `/.well-known/carddav` URI로 CalDAV/CardDAV 서버를 자동 발견한다. 현재 미구현.
- **구현 대상**: `internal/httpapi/wellknown.go`
  - `GET /.well-known/caldav` → `301` to `/caldav/`
  - `GET /.well-known/carddav` → `301` to `/carddav/`
  - `PROPFIND /.well-known/{caldav,carddav}` → `301` (WebDAV 클라이언트 지원)
  - HTTP mux에 등록 (`internal/httpapi/server.go` 또는 라우터)
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] `/.well-known/caldav` → 301 리다이렉트
  - [x] `/.well-known/carddav` → 301 리다이렉트
- **다음 태스크**: TASK-024

---

## 완료됨

- **TASK-024**: WebDAV Quota (RFC 4331) — quota-used-bytes / quota-available-bytes ✅ (2026-05-09)
  - `internal/webdavgw/webdavgw.go`: Resource에 QuotaUsedBytes/QuotaAvailableBytes 추가, MarshalPropfindResponse에 quota 속성 반영
  - `internal/httpapi/webdav.go`: WebDAVService에 GetUsageSummary 추가, PROPFIND root collection에 RFC 4331 quota 속성 삽입
  - `go test ./...` 5141개 통과
- **TASK-023**: Well-Known URIs (RFC 6764) — CalDAV/CardDAV 자동발견 ✅ (2026-05-09)
  - `internal/httpapi/wellknown.go`에 RegisterWellKnownRoutes 구현
  - GET/PROPFIND/OPTIONS 모든 메서드 301 리다이렉트 지원
  - `GOGOMAIL_WELL_KNOWN_CALDAV_URL` / `GOGOMAIL_WELL_KNOWN_CARDDAV_URL` 설정 가능
  - `go test ./...` 5141개 통과
- **TASK-022**: POP3 게이트웨이 런타임 통합 ✅ (2026-05-09)
  - `internal/pop3d/pop3d.go`에 AUTH PLAIN/LOGIN 추가 + CAPA에 SASL 광고
  - `internal/mailservice/pop3_adapter.go`에 POP3StoreAdapter + pop3Mailbox 구현 (lazy content load, CommitDeletes on QUIT)
  - `internal/config/config.go`에 POP3Addr/TLS/MaxConnections/IdleTimeout 필드
  - `internal/app/mode.go`에 ModePOP3, `run.go`에 runPOP3Gateway
  - `go test ./...` 5131개 통과
- **TASK-021**: WebDAV Gateway — Drive RFC 4918 지원 ✅ (2026-05-09)
  - `internal/httpapi/webdav.go`에 RFC 4918 WebDAV 핸들러 구현
  - PROPFIND (목록), MKCOL (폴더 생성), GET (다운로드), DELETE (휴지통), MOVE/COPY (이동/복사) 지원
  - `internal/httpapi/webdav_test.go`에 11개 테스트 (모두 통과)
  - `go test ./...` 통과
- **TASK-020**: OpenAPI → TypeScript 클라이언트 생성 ✅ (2026-05-09)
  - `Makefile`에 `gen-ts-client` 타겟 추가
  - `openapi-typescript` v7.13.0으로 `docs/openapi.yaml` → `clients/typescript/index.ts` 생성 (383KB, 11986줄)
  - `go test ./...` 통과
- **TASK-019**: Drive 파일 공유 — Directory delegation 통합 ✅ (2026-05-09)
  - `internal/httpapi/drive.go`에 `DelegatedAccessAuthorizer` 적용
  - `checkDriveDelegatedAccess` 헬퍼로 owner/actor/role 기반 권한 체크
  - 역할별 권한: read(List/Get/download), write(Trash/Restore/Rename/Move/Copy/Share), manage(PermanentDelete)
  - `go test ./...` 통과

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