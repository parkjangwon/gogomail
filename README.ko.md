# gogomail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

Go로 작성된 자체 호스팅 멀티테넌트 메일 · 협업 플랫폼입니다. 단일
정적 바이너리가 SMTP, IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP, REST
API, 이벤트 워커 역할을 모두 수행하며, 시작 시 모드를 선택합니다.
PostgreSQL · Redis · S3 호환 스토리지만 있으면 단일 호스트 데모부터
다중 DC 엔터프라이즈 배포까지 **코드 변경 없이** 동일한 바이너리로
운영할 수 있습니다.

English / 영어: [README.md](README.md)

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

## 빠른 시작

```bash
# All-in-one 데모 (Postgres + Redis + MinIO + gogomail)
cd docker
cp .env.example .env   # 시크릿 수정
docker compose -f docker-compose.small.yml up -d
```

기동 후:

- 웹메일 / API: `https://localhost/` (번들된 nginx 경유)
- 관리자 콘솔: 동일 nginx `/admin` 경로
- 메트릭: `:9090/metrics` (`GOGOMAIL_METRICS_BACKEND=prometheus` 일 때)

운영 배포는 에이전트 친화 가이드인
[`docker/DEPLOYMENT.md`](docker/DEPLOYMENT.md) (한국어:
[`docker/DEPLOYMENT.ko.md`](docker/DEPLOYMENT.ko.md))를 참고하세요.

## 문서

| 주제 | 파일 |
|---|---|
| 배포 가이드 (에이전트 친화) | [docker/DEPLOYMENT.ko.md](docker/DEPLOYMENT.ko.md) |
| 백엔드 모드 (24개) | [docs/MODES.md](docs/MODES.md) |
| 아키텍처 개요 | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| 보안 모델 | [docs/SECURITY.md](docs/SECURITY.md) |
| 운영 / 런북 | [docs/OPERATIONS.md](docs/OPERATIONS.md) |
| 토폴로지 패턴 | [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) |
| OpenAPI 계약 | [docs/openapi.yaml](docs/openapi.yaml) |
| 로드맵 | [docs/backend-roadmap.md](docs/backend-roadmap.md) |

## 소스 빌드

```bash
go build -o gogomail ./cmd/gogomail
./gogomail -mode all-in-one
```

Go 1.25 이상 필요. 테스트: `go test ./...`.

## 라이선스

[LICENSE](LICENSE) 참고.
