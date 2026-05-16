# gogomail

<img width="1456" height="720" alt="gogomail" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

Go로 구축한 표준 중심의 메일 플랫폼입니다. SMTP, IMAP, CalDAV, CardDAV — 모두 RFC를 준수하여 구현했으므로, 표준 호환 클라이언트라면 별도 플러그인 없이 작동합니다.

---

## 철학

모든 프로토콜 인터페이스가 공개 RFC에 매핑되며, 모든 설계 결정은 상호운용성을 우선합니다. 표준으로 표현할 수 없는 기능이라면 표준이 생길 때까지 기다립니다 — 그래야 MTA, 저장소, 인증 제공자 등 어떤 컴포넌트도 통합 재작업 없이 교체할 수 있습니다.

---

## 구현 현황

### 백엔드

| 컴포넌트 | 표준 | 상태 |
|---|---|---|
| SMTP 수신 (edge MTA) | RFC 5321, 5322, 2045–2049 | 프로덕션 |
| SMTP 제출 | RFC 6409, 4954 | 프로덕션 |
| SMTP 전달 + 스마트 호스트 릴레이 | RFC 5321, 7505 | 프로덕션 |
| DKIM 서명 | RFC 6376 | 프로덕션 |
| SPF / DMARC 검증 | RFC 7208, 7489 | 프로덕션 |
| DSN / 바운스 처리 | RFC 3461, 3464 | 프로덕션 |
| IMAP | RFC 9051, 3501 | 프로덕션 |
| POP3 | RFC 1939 | 프로덕션 |
| CalDAV + iCalendar | RFC 4791, 5545, 6638, 7809, 3744 | 프로덕션 |
| iMIP 스케줄링 | RFC 6047 | 프로덕션 |
| CardDAV + vCard | RFC 6352, 6350, 2426, 3744 | 프로덕션 |
| WebDAV / 드라이브 게이트웨이 | RFC 4918 | 프로덕션 |
| LDAP 디렉터리 게이트웨이 | RFC 4511, 4512, 4519 | 프로덕션 |
| 메일 + 관리자 REST API | OpenAPI, API 키 연동 | 프로덕션 |
| 드라이브 / 파일 저장소 | S3 호환 | 고급 |
| 메일 흐름 로그 + 분석 | PostgreSQL + OpenSearch | 고급 |
| API 미터링 | PostgreSQL 사용량 원장 | 프로덕션 |

상세 구현 현황: [`docs/CURRENT_STATUS.md`](docs/CURRENT_STATUS.md).

### 웹메일 (Next.js 15)

Next.js 15, Tailwind v4, shadcn/ui로 구축한 키보드 중심 웹메일 클라이언트.

- **메일** — 3-pane 레이아웃, Gmail 스타일 단축키(`g i`, `e`, `r`, `#`, …)와 한글 IME, Cmd+K 스포트라이트 검색(연산자 지원), TipTap 리치 텍스트 작성기(슬래시 명령, 인라인 이미지), 발송 지연, 스누즈, 핀, 팔로우업 알림, 받은편지함 카테고리, 구독 취소 링크 자동 감지, ICS 초대 감지.
- **필터** — AND/OR 다중 조건 규칙, 9개 조건 필드, 정규식 포함 6가지 매치 타입, 9가지 동작, 차단 발신자, 자동 응답.
- **캘린더** — 월/주/일/예약 뷰, 컬러 코드 다중 캘린더, RFC 5545 반복 이벤트, ICS 가져오기.
- **연락처** — CardDAV 기반 목록, 호버 카드, 계층적 조직도, 그룹 기반 수신자 선택.
- **드라이브** — 폴더 트리, 공유 링크, 휴지통을 갖춘 S3 기반 파일 관리자.
- **설정** — 폴더별 사서함 통계, 비동기 EML/ZIP 백업, EML/MBOX 복원, JSON 설정 가져오기/내보내기, 포커스 모드, 접근성(고대비, 모션 감소, 스크린 리더).

### 어드민 콘솔 (Next.js 15)

Next.js 15, Cloudscape Design System으로 구축한 엔터프라이즈 관리 콘솔 (port 3001).

- **Tenancy** — 회사·도메인 계층 관리, 도메인 온보딩, 변경 이력, 테넌트 헬스.
- **Organization** — SSO, SCIM 프로비저닝, 웹훅, 조직 서명, 알림 템플릿.
- **Access** — 주소 별칭, 위임, 디렉터리, 그룹 관리.
- **Mail** — 메일 흐름 로그, 메시지 추적, 배달 시도, outbox 점검, 라우팅 규칙.
- **Security** — DKIM 키, DMARC, MFA 정책, IP 접근 제어, 세션 관리, 스팸 필터, 속도 제한, API 키, 보존 정책, 인증 정책, SMTP 정책, 보안 상태 점수.
- **Storage** — 쿼터 대시보드, 좌석별 사용량, Drive 관리, 첨부파일 인벤토리, 스토리지 조정.
- **Compliance** — Legal hold, 데이터 보존 정책, 감사 로그.
- **Analytics** — API 사용량 지표, 푸시 알림 분석.
- **System** — 헬스 체크, 큐 상태, 백프레셔 모니터링.

### 제품 가이드 (VitePress)

`apps/docs`에는 공개 GoGoMail 가이드가 있습니다. 마케팅 사이트가 아니라 관리자와 사용자가 실제 운영 중에 참고하는 촘촘한 제품 가이드로 작성합니다.

- **범위** — 관리자 콘솔과 웹메일을 기능별 페이지로 나누고, DKIM, SCIM 프로비저닝, 거버넌스, 보존, 위임처럼 전문 용어가 필요한 항목은 용어 사전으로 연결합니다.
- **언어** — 영어, 한국어, 일본어, 중국어 간체를 docs i18n 계층으로 관리합니다.
- **외부 연동 API** — 그룹웨어나 사내 포털 같은 외부 시스템을 위해 API 키 인증, 이메일 기반 사서함 식별, 요청 예시, 오류 응답, 스코프, API 미터링을 설명합니다.

---

## 아키텍처

단일 바이너리, 다중 모드. 각 모드는 한 컴포넌트를 독립적으로 실행 — 별도 노드에 분산 배포하거나 개발용으로 단일 프로세스에 합칠 수 있습니다.

```
gogomail --mode=smtp-edge          # 인바운드 SMTP (port 25)
gogomail --mode=smtp-submission    # 인증된 제출 (port 587)
gogomail --mode=imap               # IMAP 서버 (port 143 / 993)
gogomail --mode=pop3               # POP3 서버 (port 110 / 995)
gogomail --mode=caldav             # CalDAV 서버
gogomail --mode=carddav            # CardDAV 서버
gogomail --mode=ldap-gateway       # 읽기 전용 LDAP v3 디렉터리 게이트웨이
gogomail --mode=webdav             # WebDAV 게이트웨이 (RFC 4918)
gogomail --mode=api                # 메일 + 관리자 REST API
gogomail --mode=delivery-worker    # 발신 SMTP 전달
gogomail --mode=outbox-relay       # outbox → 이벤트 스트림 릴레이
gogomail --mode=event-worker       # 이벤트 스트림 컨슈머
gogomail --mode=migration          # 데이터베이스 마이그레이션 실행
```

**인프라**: PostgreSQL, Redis, S3 호환 객체 저장소(로컬, MinIO, AWS S3).

---

## 외부 연동 API

GoGoMail은 포털, 그룹웨어, 결재 시스템, 내부 대시보드에서 메일 기능을 끼워 넣을 수 있도록 신뢰된 서버 간 API를 제공합니다.

- **인증** — 관리자 콘솔에서 발급한 `Authorization: Bearer gm_...` API 키를 사용합니다. 키는 스코프와 도메인 범위가 제한됩니다.
- **사서함 식별** — 외부 시스템은 보통 GoGoMail 내부 사용자 ID를 모르므로 `X-Gogomail-User-Email` 또는 `user_email` 사용을 권장합니다. `X-Gogomail-User-ID`와 `user_id`는 이미 GoGoMail 내부 사용자 ID를 저장하는 시스템을 위한 호환 옵션입니다.
- **스코프** — `mail:read`는 카운트, 폴더, 메시지 목록 조회에 사용하고, `mail:send`는 메일 작성과 발송에 사용하며, `mail:manage`는 읽음 처리나 이동처럼 사서함 상태를 바꾸는 작업에 사용합니다.
- **미터링** — 외부 API 호출은 사용량 리포트, 쿼터 분석, 고객별 과금 또는 비용 배분을 위해 기록됩니다.
- **참고 문서** — 기계가 읽는 OpenAPI 사양은 `docs/openapi.yaml`에 있고, 업체 전달용 예시는 VitePress 가이드의 `/admin-console/external-integration` 페이지에서 확인합니다.

---

## 빠른 시작

### 백엔드

요구사항: Go 1.25+, PostgreSQL 15+, Redis 7+

```bash
go build -o bin/gogomail ./cmd/gogomail

GOGOMAIL_DATABASE_URL=postgres://... bin/gogomail --mode=migration

GOGOMAIL_DATABASE_URL=postgres://... \
GOGOMAIL_REDIS_URL=redis://localhost:6379 \
GOGOMAIL_STORAGE_BACKEND=local \
GOGOMAIL_STORAGE_LOCAL_PATH=/tmp/gogomail \
bin/gogomail --mode=api
```

| 변수 | 설명 |
|---|---|
| `GOGOMAIL_DATABASE_URL` | PostgreSQL 연결 문자열 |
| `GOGOMAIL_REDIS_URL` | Redis 연결 문자열 |
| `GOGOMAIL_STORAGE_BACKEND` | `local` / `minio` / `s3` |
| `GOGOMAIL_AUTH_JWT_SECRET` | 메일 API JWT 인증용 HS256 시크릿 |
| `GOGOMAIL_ADMIN_TOKEN` | 관리자 API Bearer 토큰 |
| `GOGOMAIL_DKIM_ENABLED` | `true` 시 전달 단계에서 DKIM 서명 |
| `GOGOMAIL_DELIVERY_TLS_MODE` | `opportunistic`(기본) / `require` / `disable` |
| `GOGOMAIL_ENV` | `production` 시 더 엄격한 TLS·인증 가드 강제 |

전체 설정 참조: `internal/config/`.

### 웹메일

요구사항: Node.js 20+, pnpm 9+

```bash
cd apps/webmail
pnpm install
pnpm dev       # http://localhost:3000
pnpm build
```

### 문서 가이드

요구사항: Node.js 22+, pnpm 10+

```bash
cd apps/docs
pnpm install
pnpm dev       # http://localhost:3005
pnpm build
```

로컬 한국어 가이드는 `http://localhost:3005/ko/`에서 시작합니다. 외부 연동 API 가이드는 `http://localhost:3005/ko/admin-console/external-integration`에서 확인할 수 있습니다.

### 어드민 콘솔

요구사항: Node.js 20+, pnpm 8+

```bash
cd apps/console
pnpm install
pnpm dev       # http://localhost:3001
pnpm build
```

백엔드가 `http://localhost:8080`에서 실행 중이어야 합니다 (`GOGOMAIL_BACKEND_URL` 환경변수로 변경 가능). 관리자 자격증명으로 로그인하세요.

### 시드 데이터

```bash
docker compose -f docker/docker-compose.dev.yml up -d postgres
./scripts/seed_dev_beta.sh
```

기본 로그인: `pjw@parkjw.org` / `pass1234`.

---

## 개발

```bash
go test ./...                                # 전체 테스트
go build ./...                               # 빌드 확인
tsc --noEmit -p apps/webmail/tsconfig.json   # 웹메일 타입 확인
tsc --noEmit -p apps/console/tsconfig.json  # 어드민 콘솔 타입 확인
pnpm --dir apps/docs type-check              # 문서 타입 확인
pnpm --dir apps/docs build                   # 문서 빌드
```

Pre-commit 훅이 강제하는 규칙:

1. 커밋 전 `go test ./...` 통과.
2. `internal/` 또는 `migrations/` 변경 시 같은 커밋에 최소 1개의 `docs/` 파일 포함.

워크플로는 `docs/ACTIVE_TASK.md`가 주도합니다 — 한 번에 한 태스크, TDD, 문서·코드 동시 커밋. 전체 계약은 `PROJECT_HARNESS.md` 참고.

---

## 로드맵

| Phase | 초점 | 상태 |
|---|---|---|
| 0–1 | SMTP, IMAP, CalDAV, CardDAV, 메일/관리자 API, 전달, DKIM/SPF/DMARC | ✓ 완료 |
| 2 | 웹메일 프론트엔드 | ✓ 완료 |
| 3 | 런타임 설정 저장소 · company→domain→user 계층 · 2FA/TOTP | 예정 |
| 4 | 엔터프라이즈 인증: LDAP 디렉터리 게이트웨이 · SCIM 2.0 · SAML/OIDC | LDAP 게이트웨이 완료, SCIM/SSO 예정 |
| 5 | WebDAV 게이트웨이 · CalDAV/CardDAV 강화 | ✓ 완료 |
| 6 | 메일 보안: milter 어댑터 · DNSBL (RFC 5782) | 예정 |
| 7 | POP3 | ✓ 완료 |
| 8 | 푸시 알림: FCM / APNs / 웹 푸시 (RFC 8030) | 예정 |

전체 로드맵: [`docs/backend-roadmap.md`](docs/backend-roadmap.md).

---

## 핵심 문서

| 문서 | 내용 |
|---|---|
| `docs/ACTIVE_TASK.md` | 현재 개발 태스크 |
| `docs/backend-roadmap.md` | RFC 참고 포함 phase별 전체 로드맵 |
| `docs/CURRENT_STATUS.md` | 상세 구현 현황 |
| `docs/openapi.yaml` | 메일 + 관리자 API OpenAPI 사양 |
| `apps/docs/` | 관리자 콘솔, 웹메일, 용어 사전, 외부 연동 API를 담은 VitePress 제품 가이드 |
| `docs/adr/` | 아키텍처 결정 기록 |
| `PROJECT_HARNESS.md` | 자율 에이전트 개발 루프 계약 |

---

## 라이선스

[Elastic License 2.0](LICENSE). 내부 사용·수정은 자유이며, gogomail을 호스팅 또는 관리형 서비스로 제공하려면 명시적 허가가 필요합니다.

Copyright (c) 2026 Park Jangwon.
