# GoGoMail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

Go로 작성된 자체 호스팅 멀티테넌트 메일 · 협업 플랫폼입니다. 단일 정적 바이너리가 SMTP, IMAP, POP3, JMAP, CalDAV, CardDAV, WebDAV, LDAP, REST API, 이벤트 워커 역할을 모두 수행하며, 시작 시 모드를 선택합니다. PostgreSQL · Redis · S3 호환 스토리지만 있으면 단일 호스트 데모부터 다중 DC 엔터프라이즈 배포까지 코드 변경 없이 동일한 바이너리로 운영할 수 있습니다.

English / 영어: [README.md](README.md)

---

## GoGoMail이란

GoGoMail은 자체 커뮤니케이션 인프라를 직접 운영하려는 조직을 위한 프로덕션급 오픈소스 메일 플랫폼입니다. 주요 이메일·협업 프로토콜을 모두 단일 바이너리로 구현하고, 현대적인 웹메일 UI와 완전한 관리자 콘솔을 기본 제공하며, AI 에이전트가 서비스를 관리하거나 개별 사용자의 메일함을 자동화할 수 있는 MCP 서버를 내장합니다.

**대상:**
- SaaS 벤더 대신 메일 스택을 직접 운영하고 싶은 팀
- 회사 → 도메인 → 사용자 단위로 엄격한 멀티테넌트 격리와 완전한 감사 추적이 필요한 조직
- AI 네이티브 메일 애플리케이션을 개발하거나 메일함 워크플로를 자동화하는 개발자
- 수십 개의 마이크로서비스 대신 단일 바이너리와 의존성 세 개로 운영하고 싶은 운영자

**대상이 아닌 경우:**
- 개인용 단일 사용자 메일 서버 (GoGoMail은 처음부터 테넌트 인식 구조)
- Postfix/Dovecot 드롭인 대체품 (배송·스토리지·접근을 하나의 프로세스에서 소유)
- 호스팅 서비스 (직접 운영하는 소프트웨어)

GoGoMail은 24개 실행 모드를 선택할 수 있는 단일 정적 바이너리로 제공됩니다. 개발 환경에서는 모든 역할을 하나의 프로세스로 실행하고, 부하가 늘어나면 Docker Compose 프로필과 환경 변수, 레플리카 수만 바꿔 각 모드를 개별 컨테이너로 분리할 수 있습니다. 코드 변경은 필요하지 않습니다.

---

## 빠른 시작

```bash
# 전체 개발 스택: Postgres, Redis, MinIO, ClamAV, 백엔드, 워커, 모니터링
docker compose -f docker/docker-compose.dev.yml up -d
```

기동 후:

| 서비스 | URL |
|---|---|
| Backend API | `http://localhost:8080/` |
| Readiness probe | `http://localhost:8080/health/ready` |
| Grafana | `http://localhost:3000/` (admin / admin) |
| Postgres | `localhost:15432` |
| Redis | `localhost:16379` |
| MinIO 콘솔 | `http://localhost:19001` |
| 웹 매뉴얼 | `http://localhost:3005/` (별도 실행 — 아래 참고) |

UI 작업 시 frontend 앱은 별도로 실행합니다.

```bash
pnpm -C apps/webmail install && pnpm -C apps/webmail dev
pnpm -C apps/console install && pnpm -C apps/console dev
pnpm -C apps/docs install && pnpm -C apps/docs dev       # 웹 매뉴얼 (port 3005)
```

### 개발 데이터 시드

언어에 따라 두 가지 시드 데이터를 선택할 수 있습니다.

```bash
bash scripts/seed_dev_beta.sh       # 한국어 버전 (기본)
bash scripts/seed_dev_beta_en.sh    # 영문 버전
```

**한국어 시드** (`parkjw.org` 테넌트 — 한국어 이름, 폴더, 메일 내용):

| 계정 | 이메일 | 비밀번호 | 역할 |
|---|---|---|---|
| 관리자 | `admin@gogomail.io` | `admin1234` | admin |
| 데모 사용자 | `user@parkjw.org` | `pass1234` | user |

**영문 시드** (`acme.io` 테넌트 — 영문 이름, 폴더, 메일 내용):

| 계정 | 이메일 | 비밀번호 | 역할 |
|---|---|---|---|
| 관리자 | `admin@gogomail.io` | `admin1234` | admin |
| 데모 사용자 | `user@acme.io` | `pass1234` | user |

두 시드 모두 수신함 메일, 커스텀 폴더, 연락처 22개, CalDAV 캘린더 2개가 포함됩니다. 동료 계정 13개의 비밀번호는 모두 `pass1234`입니다. 두 테넌트는 같은 데이터베이스에 공존 가능합니다.

```bash
bash scripts/reset_dev_data.sh --yes   # 데이터 초기화 후 재시드
```

운영과 유사한 split-mode 배포:

```bash
cd docker
cp env.scale.example .env
docker compose -f docker-compose.scale.yml --profile local-infra --profile protocols --profile workers up -d
docker compose -f docker-compose.scale.yml --profile ops run --rm migrate
```

운영 배포는 [`docker/DEPLOYMENT.ko.md`](docker/DEPLOYMENT.ko.md)와 [`docs/SCALING.md`](docs/SCALING.md)를 참고하세요.

### 배포 토폴로지

동일한 바이너리, 동일한 이미지, 동일한 설정 형식 — 모든 규모에서 그대로 동작합니다.

| 토폴로지 | 사용 시기 |
|---|---|
| `docker-compose.dev.yml` (all-in-one) | 로컬 개발 — 모든 역할을 단일 프로세스로 |
| `docker-compose.scale.yml` + 프로필 | 단일 사이트 운영 — 역할별 컨테이너 분리 |
| Kubernetes (`k8s/` 또는 `helm/gogomail`) | 멀티 노드, HPA 오토스케일링, 롤링 배포, PodDisruptionBudget |

24개 실행 모드는 각각 독립적으로 수평 확장됩니다. 싱글톤 워커는 PostgreSQL Advisory Lock과 Redis 리스로 조율하며, 별도의 코디네이션 서비스가 필요하지 않습니다.

---

## 프로토콜과 모듈

각 백엔드 모듈은 정해진 공개 표준을 구현합니다. 독점 확장이나 벤더 종속 없음.

### 메일 전송

| 모듈 | 표준 |
|---|---|
| SMTP 수신 (edge MTA) | RFC 5321, RFC 5322, RFC 2045–2049, RFC 6531/6532 (SMTPUTF8) |
| SMTP 발송 | RFC 6409 (587/465), RFC 4954 (AUTH) |
| SMTP 배달 (outbound transport) | RFC 5321, RFC 7672 (DANE), RFC 7505 (null MX), RFC 3461/3464 (DSN/bounce) |
| SMTP relay / smarthost | RFC 5321 |

### 이메일 보안

| 표준 | RFC |
|---|---|
| DKIM 서명 및 검증 | RFC 6376 |
| SPF | RFC 7208 |
| DMARC | RFC 7489 |
| ARC (Authenticated Received Chain) | RFC 8617 |
| MTA-STS | RFC 8461 |
| TLS-RPT | RFC 8460 |
| DNSBL | RFC 5782 |
| Milter (외부 스팸 훅) | sendmail milter v2/v6 |

### 메일박스 접근

| 모듈 | 표준 |
|---|---|
| IMAP4rev2 | RFC 9051 (+ RFC 3501 폴백), IDLE, CONDSTORE, QRESYNC |
| POP3 | RFC 1939, RFC 2449 (CAPA), RFC 2595 (STLS), RFC 1734 (AUTH) |
| JMAP Core + Mail | RFC 8620, RFC 8621 — 20개 메서드, EventSource SSE 푸시 |

### 협업

| 모듈 | 표준 |
|---|---|
| CalDAV | RFC 4791, RFC 7809 (timezone), RFC 6638 (scheduling), RFC 5545 (iCalendar) |
| iMIP 일정 조율 | RFC 6047 |
| CardDAV | RFC 6352, RFC 6350 (vCard 4.0), RFC 2426 (vCard 3.0) |
| Drive (WebDAV) | RFC 4918, RFC 3744 (ACL), RFC 4331 (quota) |
| LDAP 게이트웨이 | RFC 4511, RFC 4512, RFC 4519 |

### 신원 및 프로비저닝

| 모듈 | 표준 |
|---|---|
| SCIM 2.0 | RFC 7642, RFC 7643, RFC 7644 |
| SAML 2.0 SSO | OASIS SAML 2.0 Core |
| OpenID Connect SSO | OpenID Connect Core 1.0, RFC 7636 (PKCE) |
| JWT 인증 | RFC 7519 |
| TOTP / HOTP MFA | RFC 6238 (TOTP), RFC 4226 (HOTP) |

### 인프라

| 모듈 | 표준 |
|---|---|
| DNS 자동 감지 | RFC 6764 (Well-Known URIs, DNS SRV) |
| Web Push | RFC 8030 |
| TLS | RFC 8446 (TLS 1.3), RFC 5246 (TLS 1.2 최소) |
| 실시간 설정 푸시 | Server-Sent Events (HTML5 EventSource) |

---

## 기능

| 영역 | 포함 내용 |
|---|---|
| 웹메일 | 메일 목록/상세, 작성, 초안, 폴더 작업, 첨부파일, 검색, 스팸/허용 발신자 설정, 프로필 사진, 주소록, 드라이브, 일정, 암호화 DM, 알림 센터, Web Push, MFA, 영어/한국어/일본어/중국어 간체 UI |
| 암호화 DM | 참여자 전용 1:1·그룹 룸, 룸별 암호화 스토리지, 읽음/안읽음, 그룹 소유자·초대, 텍스트/파일/Drive 메시지, 반응, 검색, 미디어/링크 뷰 |
| 관리자 콘솔 | 회사/도메인/사용자 관리, RBAC·커스텀 역할, 감사 로그, 메일 플로우 로그, 배달 시도, suppression·라우팅 규칙, quota/스토리지, 스팸 필터 정책, SCIM/SSO/LDAP/RDBMS 아이덴티티 설정, 보안 상태, 알림, 리포트, 분석 |
| 메일 파이프라인 | inbound/submission SMTP, 로컬 도메인 배달, outbound delivery worker, DSN/bounce, DKIM/SPF/DMARC 경계, 스팸 scoring 훅, retry 스케줄링, throttling, 이벤트 fan-out |
| 남용 방지 | 내장 스팸 필터 (SPF/DKIM/DMARC 점수 계산, RBL/DNSBL, 첨부 확장자 점수, 문구 팩, 대량 수신자 제한), IP·계정별 brute-force 추적기, ClamAV AV 스캔, milter 훅 |
| 인증·보안 | PBKDF2 비밀번호 해시 + 레거시 자동 업그레이드, TOTP MFA, refresh 토큰 회전·재사용 감지, rate limiting, 모든 admin 핸들러 IDOR 격리, 내부 헤더 stripping |
| 관찰 가능성 | Prometheus 메트릭, 구조화된 slog JSON, X-Request-ID 추적, 정리/롤백 경고 로그, SCIM 동기화 경고, Grafana 대시보드, Loki 로그 집계 |
| 스토리지 | PostgreSQL 16+, Redis 7+ (단일/Sentinel/Cluster), S3/MinIO/로컬 FS |
| 신뢰성 | Outbox 패턴 (PG → Redis Streams), 도메인별 throttling, 서킷 브레이커, 30초 graceful drain, remote-signer 타임아웃/종료 하드닝, 크래시 안전 재시작 |
| 배포 | 단일 Go 바이너리, 24개 실행 모드, Docker Compose dev/scale 프로필, Helm 차트, Kubernetes 매니페스트 (HPA, PDB, ingress) |

---

## AI 네이티브 기능

GoGoMail은 AI 에이전트 시대를 위해 설계되었습니다. 관리자 권한과 사용자 데이터를 혼용하지 않고 플랫폼의 모든 기능에 구조적으로 접근할 수 있는 두 개의 MCP (Model Context Protocol) 서버를 내장합니다.

| 서버 | 대상 | 툴 수 |
|---|---|---|
| 관리 MCP (`apps/gogomail-manage-mcp`) | 운영자, 지원팀, 관리자 | **50개** — 사용자/도메인 변경, 배달·큐 진단, 조직 소속/직책 관리, 보안·스팸 필터 정책, Admin API bridge |
| 사용자 MCP (`apps/gogomail-user-mcp`) | 개별 웹메일 사용자 | **123개** — 메일 발송/검색/bulk 작업, DM 룸·메시지·반응, 주소록, 일정, 드라이브 업로드/다운로드/공유, 알림·Web Push, 스팸 UX, 프로필/아바타 |

관리 MCP는 서비스를 운영하기 위한 서버이고, 사용자 MCP는 사용자가 Claude Desktop, Codex, 또는 다른 MCP 호환 에이전트를 자신의 메일함과 협업 데이터에 연결하기 위한 서버입니다. 관리 영역에는 접근할 수 없습니다.

```
운영자 에이전트       →  gogomail-manage-mcp  →  /admin/v1/...
개별 사용자 에이전트  →  gogomail-user-mcp    →  /api/v1/... 및 /api/mail/...
```

모든 GoGoMail 쓰기 작업은 사람이 읽을 수 있는 `reason`이 필요합니다. 파괴적 작업은 정확한 확인 문자열도 요구합니다. 민감한 사용자 작업은 `basic` 모드에서 명시 확인을 요구하며, `bypass` 모드는 사용자 설정과 도메인 정책이 모두 허용할 때만 사용 가능합니다.

→ 관리 MCP: [apps/gogomail-manage-mcp/README.ko.md](apps/gogomail-manage-mcp/README.ko.md)
→ 사용자 MCP: [apps/gogomail-user-mcp/README.ko.md](apps/gogomail-user-mcp/README.ko.md)

---

## 아키텍처 원칙

- **하나의 바이너리, 다양한 형태** — 모듈식 모놀리스. 개발에서는 24개 모드를 한 프로세스에서, 운영에서는 각 모드를 독립 배포로 분리합니다. 토폴로지 간 코드 변경 없음.
- **Outbox 패턴으로 이벤트 손실 없음** — Redis 장애 시에도 outbox에 누적되어 복구 후 자동 배출.
- **최소 의존성** — Postgres + Redis + S3. Kafka, ZooKeeper, service mesh 불필요.
- **워크로드별 수평 확장** — 모드별 독립 확장 가능. 싱글톤 워커는 PG advisory lock / Redis lease로 리더 선출.
- **프로덕션 검증기** — `internal/config/validate.go`가 시작 시 안전하지 않은 설정(insecure auth, HTTP S3, JWT 시크릿 < 32바이트, localhost HELO, `sslmode=disable`, CHANGEME 플레이스홀더)을 거부.
- **처음부터 멀티테넌트** — company → domain → user 경계가 모든 쿼리에 적용. 테넌트 간 상태 누출 없음.
- **Compose/env 배포 계약** — 리포를 클론하고, 동일한 이미지를 유지한 채, Compose 프로필·환경변수·레플리카 수만 바꾸면 단일 호스트에서 split-mode SaaS로 성장 가능.

---

## 문서

| 주제 | 파일 |
|---|---|
| 배포 가이드 | [docker/DEPLOYMENT.ko.md](docker/DEPLOYMENT.ko.md) |
| 코드 변경 없는 확장 | [docs/SCALING.md](docs/SCALING.md) |
| 백엔드 모드 (24개) | [docs/MODES.md](docs/MODES.md) |
| 아키텍처 개요 | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| 보안 모델 | [docs/SECURITY.md](docs/SECURITY.md) |
| 보안 리뷰 | [docs/SECURITY_REVIEW.md](docs/SECURITY_REVIEW.md) |
| 운영 / 런북 | [docs/OPERATIONS.md](docs/OPERATIONS.md) |
| 토폴로지 패턴 | [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) |
| OpenAPI 계약 | [docs/openapi.yaml](docs/openapi.yaml) |
| 로드맵 | [docs/backend-roadmap.md](docs/backend-roadmap.md) |
| 사용자 MCP 보안·정책 | [docs/USER_MCP.md](docs/USER_MCP.md) |
| 웹 매뉴얼 (VitePress, en/ko/ja/zh-CN) | [apps/docs/](apps/docs/) — `pnpm -C apps/docs dev` |

---

## 소스 빌드

```bash
go build -o gogomail ./cmd/gogomail
./gogomail -mode all-in-one
```

Go 1.25 이상 필요. 테스트: `go test ./...`.

## 라이선스

[LICENSE](LICENSE) 참고.
