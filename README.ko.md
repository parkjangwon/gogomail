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
- 회사/도메인/사용자 설정 경계와 보안 거버넌스 정책

### 웹메일

- 메일 목록, 읽기 패널, 리치 텍스트 작성, 폴더, 검색, 스누즈, 라벨, 알림, 첨부파일, Drive 선택 흐름
- 키보드 중심 UX: 전역 단축어, 앱 전환, 스팟라이트 검색, 행 포커스, 읽기창 이동, 메시지 작업
- HTML 메일, 외부 이미지, 원격 콘텐츠 프록시의 안전 렌더링

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

검증 명령:

```bash
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
pnpm --dir apps/webmail type-check
pnpm --dir apps/webmail test:security-helpers
pnpm --dir apps/webmail audit --prod
pnpm --dir apps/console type-check
pnpm --dir apps/console exec vitest run src/lib/__tests__/adminProxy.test.ts
pnpm --dir apps/console audit --prod
pnpm --dir apps/docs type-check
pnpm --dir apps/docs build
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

bin/gogomail --mode=api
bin/gogomail --mode=smtp-edge
bin/gogomail --mode=smtp-submission
bin/gogomail --mode=delivery-worker
bin/gogomail --mode=imap
bin/gogomail --mode=pop3
bin/gogomail --mode=caldav
bin/gogomail --mode=carddav
bin/gogomail --mode=webdav
bin/gogomail --mode=ldap-gateway
bin/gogomail --mode=migration
```

핵심 런타임 의존성:

- Go module은 `go 1.25.7`을 선언하고 toolchain `go1.26.3`을 고정합니다.
- PostgreSQL 15+
- Redis 7+
- local, MinIO, 또는 S3 호환 객체 저장소

주요 환경 변수:

| 변수 | 용도 |
|---|---|
| `GOGOMAIL_ENV` | `production`에서 더 엄격한 인증/TLS/보안 기본값 적용 |
| `GOGOMAIL_DATABASE_URL` | PostgreSQL 연결 문자열 |
| `GOGOMAIL_REDIS_URL` / `REDIS_ADDR` | Redis 연결 |
| `GOGOMAIL_STORAGE_BACKEND` | `local`, `minio`, `s3` |
| `GOGOMAIL_AUTH_JWT_SECRET` | Mail API JWT 서명 secret |
| `GOGOMAIL_ADMIN_TOKEN` | token 기반 관리자 API 접근용 bearer token |
| `GOGOMAIL_BACKEND_URL` | Next.js 서버 route가 사용할 백엔드 URL |
| `NEXT_PUBLIC_GOGOMAIL_PUBLIC_BASE_URL` | 브라우저에 표시해야 하는 public origin |

전체 설정은 `internal/config/`와 `configs/`를 참고하세요.

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
pnpm --dir apps/webmail type-check
pnpm --dir apps/console type-check
pnpm --dir apps/docs type-check
pnpm --dir apps/docs build
```

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
