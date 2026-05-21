# gogomail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

GoGoMail은 Go로 만든 표준 중심 메일 플랫폼입니다. 백엔드 메일 프로토콜,
웹메일, 엔터프라이즈 관리자 콘솔, VitePress 제품 가이드를 한 저장소에서
관리합니다.

백엔드는 특정 전용 클라이언트를 전제로 하지 않고 SMTP, IMAP, POP3, CalDAV,
CardDAV, WebDAV, LDAP, DKIM, SPF, DMARC, DSN, OpenAPI 기반 REST API처럼
상호운용 가능한 표준 경계를 중심으로 구현합니다.

[English README](README.md)

---

## 저장소 구성

| 영역 | 경로 | 설명 |
|---|---|---|
| Go 백엔드 | `cmd/`, `internal/`, `migrations/` | 메일 프로토콜, REST API, 전송 워커, 저장소, 보안 정책, 마이그레이션 |
| 웹메일 | `apps/webmail` | Next.js 16 웹메일, 포트 `3003` |
| 관리자 콘솔 | `apps/console` | Next.js 16 + Cloudscape 콘솔, 포트 `3001` |
| 제품 가이드 | `apps/docs` | VitePress 가이드, 포트 `3005`, 영어/한국어/일본어/중국어 간체 |
| API 클라이언트 | `clients/` | 생성 또는 공유 API 타입 |
| 운영 문서 | `docs/` | 현재 상태, OpenAPI, 보안 리뷰, 로드맵, ADR |
| 로컬 인프라 | `docker/` | 개발용 Docker Compose와 배포 규모별 예시 |

---

## 현재 기능

### 백엔드

- SMTP 수신, 제출, 아웃바운드 전송, 스마트 호스트 라우팅, DSN/바운스 처리
- IMAP 및 POP3 메일함 접근
- CalDAV, CardDAV, iMIP, WebDAV/Drive, 읽기 전용 LDAP 게이트웨이
- OpenAPI로 문서화된 Mail/Admin REST API
- PostgreSQL 메타데이터, Redis 조정, local/MinIO/S3 호환 객체 저장소
- DKIM 서명, SPF/DMARC 검증, DNS 점검, 큐/백프레셔, 감사 로그, API 미터링
- 내장 스팸 정책, DNSBL/RBL 점검, 테넌트 스팸 필터 팩, 선택적 ClamAV 첨부파일 스캔
- 대량 전송 배치, delivery attempt/outbox 상태 기록 배치화, 전송 route 관찰성, 조정 가능한 parsed message body 캐시
- Request-ID 전파, PostgreSQL pool sizing 환경변수, quota alert 이메일 발송 job, 선택적 retention AutoPurge job
- scheduled `pg_dump` 백업용 스크립트와 Compose cron profile
- 회사/도메인/사용자 설정 경계와 보안 거버넌스 정책

### 웹메일

- 메일 목록, 읽기 패널, 리치 텍스트 작성, 폴더, 검색, 스누즈, 라벨, 알림, 첨부파일, Drive 선택 흐름
- 비밀번호 재설정 UI, refresh-token 기반 세션 갱신, 서버 동기화 이메일 서명/필터 규칙/빠른 답장 템플릿, Web Push service worker 등록, 캘린더 편집/삭제 흐름, 클라이언트 생성 레코드용 crypto 기반 브라우저 ID 생성
- 키보드 중심 UX: 전역 단축어, 앱 전환, 스팟라이트 검색, 행 포커스, 읽기창 이동, 메시지 작업
- HTML 메일, 외부 이미지, 원격 콘텐츠 프록시의 안전 렌더링
- TOTP MFA 로그인 (비밀번호 → 인증 코드 2단계) 및 설정 화면에서 QR 코드 등록, 복구 코드 발급, 비활성화 흐름

### 관리자 콘솔

- 회사, 도메인, 사용자, 관리자, 역할, 온보딩 흐름
- SSO/SCIM, 웹훅, 알림 템플릿, 서명, 조직 설정
- 메시지 추적, 메일 흐름 로그, 전송 시도, 아웃박스, 라우팅, 릴레이, 큐, 시스템 상태
- 보안 현황, MFA, 인증/세션/IP/발신 제한 정책, DKIM, DMARC/SPF, 스팸 필터, API 키, 보존, 감사, 법적 보관
- 회사/도메인 보안 거버넌스로 보안 수준 프리셋과 private-network 웹훅 예외 관리

### 제품 가이드

`apps/docs`는 관리자 콘솔, 웹메일, 외부 연동 API, 용어 사전, 단축어, 보안
거버넌스, 사용자/운영자 작업 흐름을 문서화합니다.

---

## 보안 상태

최근 보안 강화 내역은 [`docs/SECURITY_REVIEW.md`](docs/SECURITY_REVIEW.md)에
정리되어 있습니다.

적용된 주요 제어:

- production 기본 관리자 부트스트랩 로그인 차단
- 백엔드 outbound URL guard로 DNS 해석 후 localhost, private/link-local, multicast, unspecified, metadata-service 대상 차단
- outbound 클라이언트의 redirect 재검증 및 redirect chain 제한
- 웹훅 secret 목록 응답 redaction
- 웹메일 HTML 렌더링에서 active content와 unsafe URL scheme 제거
- 이미지 프록시에서 SVG, oversized response, private destination, private redirect 차단
- 웹메일/콘솔 API 프록시에서 client-supplied credential 제거 및 allowlist header만 전달
- 쿠키 기반 mutating route에 same-origin `Origin` 또는 `Referer` 요구
- production 인증 쿠키는 `__Host-`, `HttpOnly`, `SameSite=Strict`, `Secure` 적용
- production CSP에서 `unsafe-eval` 제거, `nosniff`, frame denial, COOP/CORP, HSTS, no-store 적용
- Go toolchain `go1.26.3` 고정, 프론트엔드 PostCSS patched line override
- 회사/도메인 `/security/governance`로 typed operational exception을 관리하되 platform invariant는 전역 고정
- 웹메일 로그인에서 회사/도메인 `auth_policy`에 MFA가 필요하면 TOTP를 강제합니다. 이미 등록한 사용자는 정책과 관계없이 항상 TOTP 챌린지를 받습니다. `mfa_exempt_cidrs`로 IP 기반 면제를 설정할 수 있습니다.
- 관리자 콘솔 로그인에서 TOTP MFA를 강제할 수 있습니다. `company_admin`의 MFA 여부는 테넌트별 `auth_policy` 설정 키로 제어하고, `system_admin` 강제 등록은 `GOGOMAIL_ADMIN_MFA_REQUIRED`로 제어합니다.
- 관리자 MFA는 로그인 챌린지, `/admin/v1/auth/mfa/*` API, 설정 화면의 등록/인증/해제 플로우, 그리고 `console_mfa_setup_required` 강제 등록 가드를 모두 포함한 end-to-end로 구성되어 있습니다. 잠금 해제(break-glass): `bin/gogomail admin mfa-reset --email <주소>` (`DATABASE_URL` 환경 변수 필요).

검증 명령:

```bash
./scripts/verify-backend-release.sh
GOGOMAIL_SECURITY_VERIFY=1 ./scripts/verify-backend-release.sh
pnpm --dir apps/webmail type-check
pnpm --dir apps/webmail test:security-helpers
pnpm --dir apps/webmail audit --prod
pnpm --dir apps/console type-check
pnpm --dir apps/console exec vitest run src/lib/__tests__/adminProxy.test.ts
pnpm --dir apps/console audit --prod
pnpm --dir apps/docs type-check
pnpm --dir apps/docs build
TASK_090_DATABASE_URL='<pg_url>' scripts/verify-task-090-message-explain.sh
```

---

## 빠른 시작

### 1. 로컬 백엔드 인프라 실행

```bash
docker compose -f docker/docker-compose.dev.yml up -d
```

PostgreSQL, Redis, MinIO, MinIO bucket 초기화, `air` hot reload가 적용된 Go
백엔드를 실행합니다.

주요 로컬 엔드포인트:

| 서비스 | URL |
|---|---|
| Backend API | `http://localhost:8080` |
| PostgreSQL | `localhost:15432` |
| Redis | `localhost:16379` |
| MinIO API | `http://localhost:19000` |
| MinIO Console | `http://localhost:19001` |

전체 중지:

```bash
docker compose -f docker/docker-compose.dev.yml down
```

### 2. 개발 데이터 시드

```bash
./scripts/seed_dev_beta.sh
```

기본 개발 로그인:

```text
pjw@parkjw.org / pass1234
```

### 3. 프론트엔드 실행

```bash
pnpm --dir apps/webmail install
pnpm --dir apps/webmail dev
# http://localhost:3003
```

```bash
pnpm --dir apps/console install
pnpm --dir apps/console dev
# http://localhost:3001
```

```bash
pnpm --dir apps/docs install
pnpm --dir apps/docs dev
# http://localhost:3005
```

---

## 백엔드 바이너리

백엔드는 하나의 Go 바이너리와 여러 실행 모드로 구성됩니다.

```bash
go build -o bin/gogomail ./cmd/gogomail

bin/gogomail --mode=all-in-one
bin/gogomail --mode=mail-api
bin/gogomail --mode=admin-api
bin/gogomail --mode=auth-server
bin/gogomail --mode=edge-mta
bin/gogomail --mode=inbound-mta
bin/gogomail --mode=outbound-mta
bin/gogomail --mode=delivery-worker
bin/gogomail --mode=outbox-relay
bin/gogomail --mode=event-worker
bin/gogomail --mode=imap
bin/gogomail --mode=pop3
bin/gogomail --mode=caldav
bin/gogomail --mode=carddav
bin/gogomail --mode=webdav
bin/gogomail --mode=ldap-gateway
bin/gogomail --migrate --mode=mail-api
```

`--mode`가 기본 실행 모드 선택자입니다. `--mode`를 넘기지 않은 경우에는
`APP_MODE`도 읽으므로 Docker Compose 환경 파일과 직접 바이너리 실행이 같은
계약을 사용합니다. 둘 다 설정하면 `--mode`가 우선합니다. 알 수 없는 모드는
시작 시 즉시 실패하며, 허용 모드 목록은 `internal/app/mode.go`가 기준입니다.

핵심 런타임 의존성:

- Go module은 `go 1.25.7`을 선언하고 toolchain `go1.26.3`을 고정합니다.
- PostgreSQL 15+
- Redis 7+
- local, MinIO, 또는 S3 호환 객체 저장소

주요 환경 변수:

| 변수 | 용도 |
|---|---|
| `GOGOMAIL_ENV` | `production`에서 더 엄격한 인증/TLS/보안 기본값 적용 |
| `APP_MODE` | `--mode`를 제공하지 않았을 때 사용할 백엔드 컴포넌트 모드 fallback. `--mode`가 우선 |
| `GOGOMAIL_DATABASE_URL` | PostgreSQL 연결 문자열 |
| `GOGOMAIL_DB_MAX_OPEN_CONNS` | PostgreSQL 최대 open connection, 기본 `20` |
| `GOGOMAIL_DB_MAX_IDLE_CONNS` | PostgreSQL 최대 idle connection, 기본 `5` |
| `GOGOMAIL_DB_CONN_MAX_LIFETIME` | PostgreSQL connection max lifetime, 기본 `30m` |
| `GOGOMAIL_DB_CONN_MAX_IDLE_TIME` | PostgreSQL connection max idle time, 기본 `5m` |
| `GOGOMAIL_REDIS_ADDR` | Redis host와 port |
| `GOGOMAIL_REDIS_PASSWORD` | Redis 비밀번호, medium/large Docker profile에서 필요 |
| `GOGOMAIL_REDIS_SENTINEL_ADDRS` / `GOGOMAIL_REDIS_MASTER_NAME` | 선택적 Redis Sentinel failover 설정 |
| `GOGOMAIL_FARM_COORDINATOR_BACKEND` | SMTP farm coordinator backend, `noop` 또는 `redis`; production에서는 `redis` 필수 |
| `GOGOMAIL_FARM_COORDINATOR_NODE_ID` / `GOGOMAIL_FARM_COORDINATOR_HEARTBEAT_TTL` / `GOGOMAIL_FARM_COORDINATOR_JOB_VISIBILITY_TIMEOUT` | 분산 SMTP farm node 식별자, heartbeat TTL, delivery job visibility timeout |
| `GOGOMAIL_STORAGE_BACKEND` | `local`, `nfs`, `minio`, `s3` |
| `GOGOMAIL_MAILSTORE_ROOT` / `GOGOMAIL_STORAGE_ROOT` | local/NFS 객체 저장소 root. `GOGOMAIL_MAILSTORE_ROOT`가 우선이고 `GOGOMAIL_STORAGE_ROOT`는 deprecated fallback alias |
| `GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS` | 저장소 migration 중 capability에 노출할 compatibility label |
| `GOGOMAIL_STORAGE_S3_*` | S3/MinIO endpoint, region, bucket, prefix, credentials, path-style, TLS CA 옵션 |
| `GOGOMAIL_AUTH_JWT_SECRET` | Mail API JWT 서명 secret |
| `GOGOMAIL_ADMIN_TOKEN` | token 기반 관리자 API 접근용 bearer token |
| `GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL` / `GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD` | 개발 전용 bootstrap admin credential, production에서는 차단 |
| `GOGOMAIL_SYSTEM_EMAIL_FROM` / `GOGOMAIL_SYSTEM_SMTP_*` | 초대, welcome, quota alert, 비밀번호 재설정용 시스템 메일 발신자 |
| `GOGOMAIL_APNS_*` / `GOGOMAIL_WEBPUSH_*` | APNs 및 Web Push 알림 credential |
| `GOGOMAIL_WEBHOOK_DISPATCH_ENABLED` | tenant webhook dispatch 활성화, 기본 `true` |
| `GOGOMAIL_CORS_ALLOWED_ORIGINS` | admin/mail API가 허용할 browser origin 목록 |
| `GOGOMAIL_METRICS_BACKEND` / `GOGOMAIL_METRICS_ADDR` | metrics backend와 Prometheus scrape 주소 |
| `GOGOMAIL_PUBLIC_BASE_URL` | 시스템 메일 링크와 오픈 추적 픽셀에 쓰는 public HTTPS origin. 기본값은 없고 production에서는 localhost 계열을 사용할 수 없음 |
| `GOGOMAIL_OUTBOX_RELAY_*` | outbox relay batch size, polling interval, retry 제어 |
| `GOGOMAIL_DELIVERY_*` | delivery worker stream, consumer, retry, TLS, smart-host, route, throttle, timeout 제어 |
| `GOGOMAIL_DELIVERY_RECIPIENT_BATCH_SIZE` | 같은 도메인 SMTP 전송 배치의 최대 수신자 수, 기본 `100` |
| `GOGOMAIL_MESSAGE_BODY_CACHE_ENTRIES` | parsed message body 캐시 용량, 기본 `256`, `0`이면 비활성화 |
| `GOGOMAIL_MESSAGE_BODY_CACHE_TTL` | parsed message body 캐시 TTL, 기본 `5m` |
| `GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL` | 릴리즈 검증에서 백업/복구 리허설에 사용할 선택 DB URL |
| `GOGOMAIL_RESTORE_REHEARSAL_DB_URL` / `GOGOMAIL_RESTORE_REHEARSAL_KEEP_DB` | `scripts/backup-restore-rehearsal.sh`용 scratch DB override와 보존 flag |
| `GOGOMAIL_AUTO_PURGE_ENABLED` | 회사 retention policy의 `auto_purge_enabled`가 켜진 테넌트에 대해 scheduled AutoPurge 실행 |
| `GOGOMAIL_AUTO_PURGE_INTERVAL` | retention AutoPurge scheduler interval, 기본 `24h` |
| `GOGOMAIL_AUTO_PURGE_BATCH_SIZE` | 회사별 1회 실행에서 삭제할 messages/audit rows 최대치, 기본 `1000` |
| `GOGOMAIL_API_METERING_*` / `GOGOMAIL_API_USAGE_*` | API metering stream, aggregation, retention, export signer 제어 |
| `GOGOMAIL_ATTACHMENT_SCAN_*` / `GOGOMAIL_ATTACHMENT_CLEANUP_*` | 첨부파일 scanning backend, ClamAV/webhook 옵션, stale upload cleanup 제어 |
| `GOGOMAIL_PUSH_NOTIFICATION_*` | push notification backend, webhook, consumer, device limit 제어 |
| `GOGOMAIL_BACKUP_DIR` | `scripts/backup.sh`가 사용할 백업 디렉터리, 기본 `./backups` |
| `GOGOMAIL_BACKUP_RETENTION_DAYS` | local backup retention 기간, 기본 `7` |
| `GOGOMAIL_BACKUP_S3_BUCKET` / `GOGOMAIL_BACKUP_S3_PREFIX` | 설정 시 백업 파일을 업로드할 S3 bucket과 key prefix |
| `GOGOMAIL_SECURITY_VERIFY` | `1`이면 backend release verification에 `go vet`과 `govulncheck` 추가 |
| `GOGOMAIL_BACKEND_URL` | Next.js 서버 route가 사용할 백엔드 URL |
| `NEXT_PUBLIC_GOGOMAIL_PUBLIC_BASE_URL` | 브라우저에 표시해야 하는 public origin |
| `NEXT_PUBLIC_VAPID_PUBLIC_KEY` | 웹메일 push subscription 등록에 쓰는 browser-visible Web Push VAPID public key |
| `GOGOMAIL_ADMIN_MFA_REQUIRED` | `system_admin` 로그인에 TOTP MFA 등록 강제 여부, 기본 `false` |

전체 설정은 `internal/config/`, `internal/config/validate.go`, `configs/`, [`docker/.env.example`](docker/.env.example), [`apps/webmail/.env.example`](apps/webmail/.env.example), [`apps/console/.env.example`](apps/console/.env.example)를 참고하세요.

---

## 외부 연동 API

신뢰된 외부 시스템은 서버 간 API로 포털, 그룹웨어, 결재 시스템, 내부
대시보드에 메일 기능을 붙일 수 있습니다.

- 관리자 콘솔에서 발급한 `Authorization: Bearer gm_...` API 키를 사용합니다.
- 사서함 식별자는 `X-Gogomail-User-Email` 또는 `user_email`을 권장합니다.
- 권한 범위는 `mail:read`, `mail:send`, `mail:manage`처럼 좁게 부여합니다.
- API 호출은 사용량 리포트와 쿼터 분석을 위해 미터링됩니다.

참고:

- [`docs/openapi.yaml`](docs/openapi.yaml)
- `apps/docs`의 `/admin-console/external-integration` 페이지

---

## 개발 메모

```bash
go test ./...
go vet ./...
go build ./...
./scripts/verify-backend-release.sh
./scripts/verify-frontend-release.sh
pnpm --dir apps/webmail type-check
pnpm --dir apps/console type-check
pnpm --dir apps/docs type-check
pnpm --dir apps/docs build
```

릴리즈 검증 진입점:

- `./scripts/verify-backend-release.sh`는 Go 테스트, module tidy diff, 선택 PostgreSQL/OpenSearch 통합 테스트, 선택 백업/복구 리허설, 선택 보안 검증, clean-worktree gate를 실행합니다.
- `./scripts/verify-frontend-release.sh`는 기본적으로 webmail/console type-check와 helper test를 실행합니다. 더 무거운 브라우저/build gate는 `GOGOMAIL_FRONTEND_E2E=1`, `GOGOMAIL_FRONTEND_BUILD=1`로 켭니다.

이 저장소는 엄격한 프로젝트 하네스를 사용합니다.

- 구현 작업 전 `docs/ACTIVE_TASK.md`를 읽습니다.
- 동작 변경 시 코드, 테스트, 문서를 같은 커밋에 둡니다.
- pre-commit hook은 `go test ./...`를 실행합니다.
- 백엔드 또는 migration 변경 시 staged `docs/` 업데이트가 필요합니다.

자세한 내용은 [`PROJECT_HARNESS.md`](PROJECT_HARNESS.md)를 참고하세요.

---

## 핵심 문서

| 문서 | 내용 |
|---|---|
| [`docs/ACTIVE_TASK.md`](docs/ACTIVE_TASK.md) | 현재 개발 태스크 |
| [`docs/CURRENT_STATUS.md`](docs/CURRENT_STATUS.md) | 상세 구현 현황 |
| [`docs/SECURITY_REVIEW.md`](docs/SECURITY_REVIEW.md) | 보안 강화 요약과 검증 명령 |
| [`docs/backend-release-readiness.md`](docs/backend-release-readiness.md) | 릴리즈 준비 체크, 선택 gate, 운영 검증 메모 |
| [`docs/openapi.yaml`](docs/openapi.yaml) | Mail/Admin API 계약 |
| [`docs/backend-roadmap.md`](docs/backend-roadmap.md) | 장기 백엔드 로드맵과 완료된 하드닝 항목 |
| [`apps/docs/`](apps/docs/) | 관리자 콘솔, 웹메일, 용어 사전, 연동 API 제품 가이드 |
| [`docker/`](docker/) | Docker Compose 파일과 배포 메모 |
| [`docs/adr/`](docs/adr/) | 아키텍처 결정 기록 |

---

## 라이선스

[Elastic License 2.0](LICENSE). 내부 사용과 수정은 자유이며, GoGoMail을
호스팅 또는 관리형 서비스로 제공하려면 명시적 허가가 필요합니다.

Copyright (c) 2026 Park Jangwon.
