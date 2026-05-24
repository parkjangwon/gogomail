# gogomail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

Go로 작성된 자체 호스팅 멀티테넌트 메일 · 협업 플랫폼입니다. 단일
정적 바이너리가 SMTP, IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP, REST
API, 이벤트 워커 역할을 모두 수행하며, 시작 시 모드를 선택합니다.
PostgreSQL · Redis · S3 호환 스토리지만 있으면 단일 호스트 데모부터
다중 DC 엔터프라이즈 배포까지 **코드 변경 없이** 동일한 바이너리로
운영할 수 있습니다.

English / 영어: [README.md](README.md)

현재 기준: **2026-05-25**. 이 저장소는 SaaS 출시 전 하드닝에 집중하고
있습니다. 웹메일 사용성, 알림 안전성, 로컬 도메인 배송, 스팸 제어,
MCP 자동화, split-mode 배포 준비가 핵심 축입니다.

## 무엇인가

- 자체 호스팅 메일 플랫폼: SMTP 수신/발송/배달 + IMAP + POP3
- 내장 웹메일(Next.js 16) 및 관리자 콘솔
- CalDAV · CardDAV · WebDAV 기반 일정/주소록/드라이브
- LDAP 디렉터리 게이트웨이 + SCIM 2.0 프로비저닝
- 멀티테넌시: 모든 쿼리에 **company → domain → user** 경계 적용
- 단일 Go 바이너리, 24개 실행 모드 (자세히는 [`docs/MODES.md`](docs/MODES.md))

## 기능

| 영역 | 기능 |
|---|---|
| 메일 서버 | RFC 5321/5322 SMTP, RFC 6409 submission (587/465), RFC 5321/7672 DANE 지원 송신 |
| 메일박스 프로토콜 | IMAP4rev2 (RFC 9051) IDLE/CONDSTORE/QRESYNC, POP3 (RFC 1939) |
| 협업 | CalDAV (RFC 4791), CardDAV (RFC 6352), WebDAV (RFC 4918), LDAP (RFC 4511) |
| API | Mail API, Admin API, Auth 서버 (JWT + refresh + MFA), SCIM 2.0 |
| 웹메일 / 관리자 | Next.js 16 웹메일 SPA 및 관리자 콘솔 (`apps/webmail`, `apps/console`) |
| 이메일 보안 | SPF (RFC 7208), DKIM (RFC 6376), DMARC (RFC 7489), ARC (RFC 8617), MTA-STS (RFC 8461), TLS-RPT (RFC 8460) |
| 인증 | JWT (HS256, 32바이트 이상 시크릿), TOTP MFA, refresh 토큰 회전 + 재사용 감지, PBKDF2 비밀번호 해시 |
| 남용 방지 | IP·계정별 brute-force 추적기, rate limit, DNSBL, milter, ClamAV 연동 |
| 관찰 가능성 | Prometheus 메트릭, slog JSON 로그(시크릿 마스킹) |
| 스토리지 | PostgreSQL 16+, Redis 7+ (단일 / Sentinel / Cluster), S3 / MinIO / 로컬 FS |
| 신뢰성 | Outbox 패턴 (PG → Redis Streams), 도메인별 throttling, 서킷 브레이커, 30초 graceful drain |

## 현재 제품 표면

- **웹메일** — 메일 목록/상세, 작성, 초안, 폴더 작업, 첨부파일, 검색,
  모든 편지함, 스팸/차단/허용 발신자 설정, 프로필 사진, 주소록,
  드라이브, 일정, 알림 센터, Web Push, MFA, refresh-token 세션,
  영어/한국어/일본어/중국어 간체 설정을 제공합니다.
- **관리자 콘솔** — 회사/도메인/사용자 관리, 감사 로그, 배송 시도,
  suppression 및 라우팅 제어, quota/storage 화면, 보안 상태,
  SCIM/SSO/조직 설정, 출시 준비 UI를 위한 광범위한 mock 기반 E2E
  커버리지를 제공합니다.
- **메일 파이프라인** — inbound/submission SMTP, MX fallback 없는 로컬
  도메인 배송, outbound delivery worker, DSN/bounce 생성, DKIM/SPF/DMARC
  경계, spam relay hook, retry scheduling, throttling, event fan-out을
  포함합니다.
- **에이전트 자동화** — 관리 MCP와 사용자 MCP를 분리해 운영자는
  서비스를 관리하고, 사용자는 자신의 메일함/주소록/드라이브/일정/
  환경설정을 안전하게 자동화할 수 있습니다.

## 강점

- **하나의 바이너리, 다양한 형태** — modular monolith. 개발에서는 24개
  모드를 한 프로세스에서, 운영에서는 각 모드를 독립 배포로 분리할 수
  있습니다.
- **Outbox 패턴으로 이벤트 손실 없음** — Redis가 장애여도 outbox에
  누적되어 복구 시 자동으로 배출됩니다.
- **RFC 우선 프로토콜** — `5321`, `5322`, `9051`, `1939`, `4791`,
  `6352`, `4918`, `4511`, DKIM/SPF/DMARC/ARC/MTA-STS.
- **프로덕션 검증기** — `internal/config/validate.go`가 시작 시
  안전하지 않은 설정(insecure auth, HTTP S3, JWT 시크릿 < 32바이트,
  localhost HELO, sslmode=disable 등)을 거부합니다.
- **최소 의존성** — Postgres + Redis + S3. Kafka · ZooKeeper · service
  mesh 불필요.
- **워크로드별 수평 확장** — 모드별로 독립적 확장 가능. 싱글톤 워커는
  PG advisory lock / Redis lease로 리더 선출.
- **단일 진실의 출처** — 테넌트 · 메일박스 · outbox 상태가 모두
  Postgres에. 로컬 스풀 없음, 크래시 복구 안전.
- **로컬 우선 smoke 경로** — 개발 Compose 스택이 HTTP backend와
  event/outbox relay/delivery worker를 함께 실행하므로 웹메일
  send/receive 경로를 수동 worker 기동 없이 확인할 수 있습니다.

## 빠른 시작

```bash
# 로컬 개발 스택 (Postgres + Redis + MinIO + ClamAV + backend + workers)
cd docker
docker compose -f docker-compose.dev.yml up -d --build \
  backend event-worker outbox-relay delivery-worker
```

기동 후:

- Backend API: `http://localhost:8080/`
- Readiness: `http://localhost:8080/health/ready`
- Postgres / Redis / MinIO: `localhost:15432`, `localhost:16379`,
  `localhost:19000` (`localhost:19001` console)

UI 작업 시 frontend 앱은 별도로 실행합니다.

```bash
pnpm -C apps/webmail install
pnpm -C apps/webmail dev

pnpm -C apps/console install
pnpm -C apps/console dev
```

운영과 유사한 split-mode 배포는 no-code scaling 템플릿에서 시작합니다.

```bash
cd docker
cp env.scale.example .env
docker compose -f docker-compose.scale.yml --profile local-infra --profile protocols --profile workers up -d
docker compose -f docker-compose.scale.yml --profile ops run --rm migrate
```

운영 배포는 [`docker/DEPLOYMENT.md`](docker/DEPLOYMENT.md) (한국어:
[`docker/DEPLOYMENT.ko.md`](docker/DEPLOYMENT.ko.md))와
[`docs/SCALING.md`](docs/SCALING.md)를 참고하세요.

## AI 에이전트 자동화 (MCP 서버)

GoGoMail은 관리자 권한과 사용자 데이터 접근 권한을 섞지 않기 위해 [Model Context Protocol](https://modelcontextprotocol.io/) 서버를 둘로 분리합니다.

| 서버 | 디렉터리 | 대상 | 범위 |
|---|---|---|---|
| 관리 MCP | `apps/gogomail-manage-mcp` | 운영자, 지원팀, 관리자 | Admin API, 큐/헬스 확인, 사용자/도메인 운영, 조직 소속/직책 메타데이터, 보안/스팸 정책, 선택적 Suppo/GitHub 연동을 위한 49개 관리 툴 |
| 사용자 MCP | `apps/gogomail-user-mcp` | 개별 웹메일 사용자 | 사용자 스코프 `gmu_` 키를 통한 메일, 초안, 폴더, 스레드, 주소록, 디렉터리, 드라이브, 일정, 환경설정, spam UX, 프로필/아바타, 계정 컨텍스트용 96개 사용자 툴 |

관리 MCP는 서비스를 운영하기 위한 서버이고, 사용자 MCP는 사용자가 Codex, Claude Desktop 같은 에이전트를 자신의 메일함과 협업 데이터에 연결하기 위한 서버입니다.

```
운영자 요청
    → AI 에이전트
        → gogomail-manage-mcp
            → /admin/v1/...

사용자 요청
    → AI 에이전트
        → gogomail-user-mcp
            → /api/v1/... 및 /api/mail/...
```

`gogomail-manage-mcp`는 현재 **49개 관리 툴**을 제공합니다. 감사 로그가 붙는 사용자/도메인 변경, 배송·큐 진단, 조직 소속/직책 관리, 보안 및 스팸 필터 정책 헬퍼, 전용 wrapper가 없는 문서화된 어드민 콘솔 라우트를 위한 제한된 `gogomail_admin_api_request` bridge가 포함됩니다. 모든 GoGoMail 쓰기 작업은 사람이 읽을 수 있는 `reason`이 필요하고, 파괴적 작업은 정확한 확인 문자열도 요구합니다.

`gogomail-user-mcp`는 현재 **96개 사용자 툴**을 제공합니다. 메일 발송/초안/검색, 메시지·스레드 bulk 작업, spam 신고/not-spam 및 발신자 허용/차단 헬퍼, 프로필/아바타 헬퍼, 주소록·일정 CRUD 편의 툴, 드라이브 업로드/다운로드/공유 링크, 문서화된 사용자 API만 허용하는 exact-manifest `gogomail_api_request` bridge가 포함됩니다. 민감한 동작은 `basic` 모드에서 명시 확인을 요구하며, `bypass` 모드는 사용자 설정과 도메인 정책이 모두 허용할 때만 사용할 수 있습니다.

→ 관리 MCP 문서: [apps/gogomail-manage-mcp/README.md](apps/gogomail-manage-mcp/README.md) / [한국어](apps/gogomail-manage-mcp/README.ko.md)
→ 사용자 MCP 문서: [apps/gogomail-user-mcp/README.md](apps/gogomail-user-mcp/README.md) / [한국어](apps/gogomail-user-mcp/README.ko.md)

## 문서

| 주제 | 파일 |
|---|---|
| 배포 가이드 (에이전트 친화) | [docker/DEPLOYMENT.ko.md](docker/DEPLOYMENT.ko.md) |
| 코드 변경 없는 확장 | [docs/SCALING.md](docs/SCALING.md) |
| 백엔드 모드 (24개) | [docs/MODES.md](docs/MODES.md) |
| 현재 구현 상태 | [docs/CURRENT_STATUS.md](docs/CURRENT_STATUS.md) |
| 아키텍처 개요 | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| 보안 모델 | [docs/SECURITY.md](docs/SECURITY.md) |
| 운영 / 런북 | [docs/OPERATIONS.md](docs/OPERATIONS.md) |
| 토폴로지 패턴 | [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) |
| OpenAPI 계약 | [docs/openapi.yaml](docs/openapi.yaml) |
| 로드맵 | [docs/backend-roadmap.md](docs/backend-roadmap.md) |
| AI 에이전트 관리 MCP 서버 | [apps/gogomail-manage-mcp/README.ko.md](apps/gogomail-manage-mcp/README.ko.md) |
| AI 에이전트 사용자 MCP 서버 | [apps/gogomail-user-mcp/README.ko.md](apps/gogomail-user-mcp/README.ko.md) |
| 사용자 MCP 보안/정책 메모 | [docs/USER_MCP.md](docs/USER_MCP.md) |

## 소스 빌드

```bash
go build -o gogomail ./cmd/gogomail
./gogomail -mode all-in-one
```

Go 1.25 이상 필요. 테스트: `go test ./...`.

## 라이선스

[LICENSE](LICENSE) 참고.
