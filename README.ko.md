# gogomail

<img width="1456" height="720" alt="1777874812592" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

Go로 구축한 표준 중심의 메일 플랫폼입니다. SMTP, IMAP, CalDAV, CardDAV — 모든 기능이 RFC를 준수하여 구현되어 있어, 어떤 표준 호환 클라이언트에서도 기본 설정으로 작동합니다. 독점 플러그인이나 벤더별 확장이 필요하지 않습니다.

---

## 철학

대부분의 웹메일 플랫폼은 수년에 걸쳐 독점 API와 벤더 락인을 축적합니다. gogomail은 반대 접근을 취합니다. 모든 프로토콜 인터페이스가 공개 RFC에 매핑되며, 모든 설계 결정이 상호운용성을 먼저 고려합니다. 표준으로 표현할 수 없는 기능이라면 표준으로 표현할 수 있을 때까지 기다립니다.

이는 실무에서 의미가 있습니다. 메일 클라이언트, 캘린더 앱, 연락처 동기화가 모두 공개 표준을 통해 작동할 때, MTA, 저장소 백엔드, 인증 제공자 등 모든 컴포넌트를 — 통합 재작업 없이 — 교체할 수 있습니다.

---

## 구현 현황

### 백엔드

| 컴포넌트 | 표준 | 상태 |
|---|---|---|
| SMTP 수신 (edge MTA) | RFC 5321, 5322, 2045–2049 | 프로덕션 준비 완료 |
| SMTP 제출 (발신 MTA) | RFC 6409, AUTH RFC 4954 | 프로덕션 준비 완료 |
| SMTP 전달 + 스마트 호스트 릴레이 | RFC 5321, RFC 7505 (null MX) | 프로덕션 준비 완료 |
| DKIM 서명 | RFC 6376 | 프로덕션 준비 완료 |
| SPF / DMARC 검증 | RFC 7208, RFC 7489 | 프로덕션 준비 완료 |
| DSN / 바운스 처리 | RFC 3461, RFC 3464 | 프로덕션 준비 완료 |
| IMAP | RFC 9051 (IMAP4rev2), RFC 3501 | 프로덕션 준비 완료 |
| CalDAV + iCalendar | RFC 4791, RFC 5545, RFC 6638 | 고급 |
| iMIP 스케줄링 | RFC 6047 | 완료 |
| 타임존 지원 | RFC 7809 | 완료 |
| CardDAV + vCard | RFC 6352, RFC 6350, RFC 2426 | 고급 |
| 메일 API (REST) | OpenAPI | 프로덕션 준비 완료 |
| 관리자 API | OpenAPI | 프로덕션 준비 완료 |
| POP3 | RFC 1939 | 프로덕션 준비 완료 |
| 드라이브 / 파일 저장소 | S3 호환 백엔드 | 고급 |
| WebDAV / 드라이브 게이트웨이 | RFC 4918 | 고급 |
| 메일 흐름 로그 + 분석 | PostgreSQL + OpenSearch | 고급 |

### 웹메일 (Next.js 15)

Next.js 15, Tailwind v4, shadcn/ui로 구축한 키보드 중심의 웹메일 클라이언트입니다. Notion Mail / Superhuman UX 원칙에서 영감을 얻었습니다.

**메일**
- 3-pane 레이아웃 — 사이드바, 메시지 목록, 읽기 창
- 키보드 단축키: Gmail 스타일 (`g i`, `g s`, `e`, `r`, `a`, `f`, `#`, …) + 한글 IME 지원
- 스포트라이트 검색 (Cmd+K) with Gmail 스타일 연산자 (`from:`, `to:`, `subject:`, `has:attachment`)
- 인라인 답장 편집기 (TipTap v2) with 리치 텍스트, 슬래시 명령, 인라인 이미지
- TipTap을 이용한 작성 — 슬래시 명령, 인라인 이미지 붙여넣기, 첨부파일 업로드
- 발송 지연 (실행 취소 발송 카운트다운)
- 스누즈, 핀, 발송된 메일에 대한 팔로우업 알림
- 받은편지함 카테고리 탭 + 스마트 자동분류 칩
- 컴팩트 보기 토글
- 받은편지함 비우기 축하 상태
- 구독 취소 링크 자동 감지
- ICS 캘린더 초대 감지 with 캘린더에 추가

**필터 & 자동화**
- 다중 조건 필터 규칙 with AND/OR 로직
- 9개 조건 필드: from, to, cc, subject, body, has attachment, is unread, size larger/smaller
- 6가지 매치 타입: contains, not contains, equals, starts with, ends with, regex
- 9가지 동작: label color, move to folder, mark read/unread, star, mark important, skip inbox, delete, forward
- 규칙별 활성화 토글 + 처리 중단 플래그
- 차단된 발신자, 자동 응답

**가상 폴더**
- 읽지 않음, 스누즈됨, 핀, 중요, 작업 — 모두 사이드바 단축키

**캘린더**
- 월/주/일/예약 뷰
- 컬러 코드 적용된 여러 캘린더 — 추가, 편집, 삭제
- 반복 이벤트 (RFC 5545 RRULE: daily/weekly/monthly/yearly, interval, day selection, end by count or date)
- 이메일을 통한 ICS 가져오기

**연락처 & 조직도**
- CardDAV 기반 연락처 목록 with 검색
- 메시지 헤더의 연락처 호버 카드
- 계층적 네비게이션을 이용한 조직도 (조직도)
- 작성 모달에서 그룹 기반 수신자 선택
- 3-pane 수신자 피커 (조직 트리, 구성원/연락처, 선택된 수신자)

**드라이브**
- 폴더 트리, 업로드, 다운로드, 공유 링크, 휴지통을 이용한 S3 기반 파일 관리자

**설정**
- 일반, 모양, 알림, 읽기, 작성, 서명/자동 응답, 필터, 차단된 발신자, 자동 응답, 템플릿, 개인정보, 접근성, 엔터프라이즈 보안, 저장소/백업, 정보
- 폴더별 사서함 통계 (메시지 수, 읽지 않음, 별표, 예상 크기)
- 폴더별 비동기 EML / ZIP 백업 (논블로킹, 진행률 추적)
- EML/MBOX 파일에서 사서함 복원
- 설정 가져오기 / 내보내기 (JSON)
- 포커스 모드, DND 인식 브라우저 알림
- 고대비, 감소된 모션, 글꼴 가족, 스크린 리더 모드

---

## 아키텍처

단일 바이너리, 다중 모드. 각 모드는 한 컴포넌트를 독립적으로 실행하므로, 별도 노드에 배포하거나 개발을 위해 단일 프로세스로 구성할 수 있습니다.

```
gogomail --mode=smtp-edge          # 인바운드 SMTP (port 25)
gogomail --mode=smtp-submission    # 인증된 제출 (port 587)
gogomail --mode=imap               # IMAP 서버 (port 143 / 993)
gogomail --mode=pop3               # POP3 서버 (port 110 / 995)
gogomail --mode=caldav             # CalDAV 서버
gogomail --mode=carddav            # CardDAV 서버
gogomail --mode=webdav             # WebDAV 게이트웨이 (RFC 4918)
gogomail --mode=api                # 메일 + 관리자 REST API
gogomail --mode=delivery-worker    # 발신 SMTP 전달
gogomail --mode=outbox-relay       # outbox → 이벤트 스트림 릴레이
gogomail --mode=event-worker       # 이벤트 스트림 컨슈머
gogomail --mode=migration          # 데이터베이스 마이그레이션 실행
```

**인프라**: PostgreSQL, Redis, S3 호환 객체 저장소 (로컬, MinIO, 또는 AWS S3).

---

## 최근 업데이트 (2026-05-14)

### WebDAV 게이트웨이 인증 활성화 (2026-05-14)
- ✅ 외부 클라이언트 접근을 위한 Bearer 토큰 및 HTTPS Basic 인증 지원
- ✅ 외부 클라이언트 (Mac Finder, Windows Explorer, Linux) `/dav/` 엔드포인트를 통한 드라이브 마운트
- ✅ RWMutex와 만료된 Lock 자동 정리로 Lock 관리 최적화
- ✅ 모든 RFC 4918 WebDAV 작업 지원: OPTIONS, PROPFIND, MKCOL, GET, PUT, DELETE, MOVE, COPY, PROPPATCH, LOCK, UNLOCK

### 웹메일 Phase 3 완료 (2026-05-12)
- ✅ E2E 테스트 인프라 (Playwright, 25+ 테스트 케이스)
- ✅ 계층적 네비게이션을 이용한 조직도 수신자 피커
- ✅ ComposeModal 통합: 발송 지연, 초안 자동저장, 이모지 피커
- ✅ ReadingPane 개선: 별표 토글, 읽음/읽지 않음, 캘린더 초대 감지
- ✅ 설정 모달: 프로필 사진, 보안, 엔터프라이즈 기능
- ✅ 드라이브 파일 피커, 메시지 검색 with 연산자, 받은편지함 카테고리

### API & 인프라
- ✅ 백엔드 API 경로 정렬: `/api/v1/` → `/api/mail/` (971개 테스트 통과)
- ✅ 계층적 조직 데이터 구조 로드됨 (depth 기반 parent_id 관계)
- ✅ 멤버 해석을 포함한 CardDAV 디렉토리 org-tree 엔드포인트
- ✅ 관리자 콘솔: 사용자 관리, 조직 구조, 감사 로그
- ✅ 개발/프로덕션 배포를 위한 Docker Compose 설정

### 알려진 이슈
- 웹메일 조직도 UI 렌더링: DB의 계층적 데이터가 프론트엔드 표시 통합 대기 중

---

## 빠른 시작

### 백엔드

**요구사항**: Go 1.25+, PostgreSQL 15+, Redis 7+

```bash
# 빌드
go build -o bin/gogomail ./cmd/gogomail

# 마이그레이션 실행
GOGOMAIL_DATABASE_URL=postgres://... bin/gogomail --mode=migration

# API 서버 시작 (개발)
GOGOMAIL_DATABASE_URL=postgres://... \
GOGOMAIL_REDIS_URL=redis://localhost:6379 \
GOGOMAIL_STORAGE_BACKEND=local \
GOGOMAIL_STORAGE_LOCAL_PATH=/tmp/gogomail \
bin/gogomail --mode=api
```

주요 환경 변수:

| 변수 | 설명 |
|---|---|
| `GOGOMAIL_DATABASE_URL` | PostgreSQL 연결 문자열 |
| `GOGOMAIL_REDIS_URL` | Redis 연결 문자열 |
| `GOGOMAIL_STORAGE_BACKEND` | `local` / `minio` / `s3` |
| `GOGOMAIL_AUTH_JWT_SECRET` | 메일 API JWT 인증을 위한 HS256 시크릿 |
| `GOGOMAIL_ADMIN_TOKEN` | 관리자 API용 Bearer 토큰 |
| `GOGOMAIL_DKIM_ENABLED` | `true` 로 설정하여 전달 시 DKIM 서명 활성화 |
| `GOGOMAIL_DELIVERY_TLS_MODE` | `opportunistic` (기본값) / `require` / `disable` |
| `GOGOMAIL_ENV` | `production` 으로 설정 시 더 엄격한 TLS 및 인증 가드 강제 |

전체 설정 참고: `internal/config/`.

### 웹메일

**요구사항**: Node.js 20+, pnpm 9+

```bash
cd apps/webmail
pnpm install
pnpm dev       # http://localhost:3000
pnpm build     # 프로덕션 빌드
```

---

## 개발

```bash
# 모든 테스트 실행
go test ./...

# 특정 패키지의 테스트 실행
go test ./internal/scheduling/...

# 빌드 확인
go build ./...

# 웹메일 타입 확인
tsc --noEmit -p apps/webmail/tsconfig.json
```

Pre-commit 훅은 자동으로 두 가지 규칙을 강제합니다:

1. 커밋 전에 `go test ./...`이 통과해야 합니다.
2. 프로덕션 코드 변경 (`internal/`, `migrations/`)은 같은 커밋에서 최소 1개의 `docs/` 파일을 스테이징해야 합니다.

개발 워크플로는 `docs/ACTIVE_TASK.md`에 의해 주도됩니다 — 한 번에 하나의 태스크, TDD, 문서와 코드 함께 커밋. 전체 계약은 `PROJECT_HARNESS.md`를 참고하십시오.

---

## 로드맵

| Phase | 초점 | 상태 |
|---|---|---|
| 0–1 | SMTP, IMAP, CalDAV, CardDAV, 메일/관리자 API, 전달, DKIM/SPF/DMARC | ✓ 완료 |
| 2 | 웹메일 프론트엔드 — 키보드 중심, 설정, 필터, 캘린더, 연락처, 드라이브 | ✓ 완료 |
| 3 | 런타임 설정 저장소 · company→domain→user 설정 계층 · 2FA/TOTP | 예정 |
| 4 | 엔터프라이즈 인증: LDAP (RFC 4511) · SCIM 2.0 · SAML/OIDC | 예정 |
| 5 | 드라이브 WebDAV 게이트웨이 (RFC 4918) · CalDAV/CardDAV 프로덕션 강화 | ✓ WebDAV 완료 |
| 6 | 메일 보안: milter 어댑터 · DNSBL (RFC 5782) | 예정 |
| 7 | POP3 (RFC 1939) | ✓ 완료 |
| 8 | 푸시 알림: FCM / APNs / 웹 푸시 (RFC 8030) | 예정 |

전체 로드맵: `docs/backend-roadmap.md`.

---

## 핵심 문서

| 문서 | 내용 |
|---|---|
| `docs/ACTIVE_TASK.md` | 현재 개발 태스크 |
| `docs/backend-roadmap.md` | RFC 참고가 포함된 전체 phase별 로드맵 |
| `docs/CURRENT_STATUS.md` | 상세한 현재 구현 상태 |
| `docs/DESIGN.md` | 프론트엔드 디자인 언어 및 컴포넌트 패턴 |
| `docs/openapi.yaml` | 메일 + 관리자 API OpenAPI 사양 |
| `docs/backend-api-contracts.md` | API 계약 문서 |
| `docs/adr/` | 아키텍처 결정 기록 |
| `PROJECT_HARNESS.md` | 자율 에이전트 개발 루프 계약 |

---

## 라이선스

[Elastic License 2.0](LICENSE) — 내부적으로 무료로 사용 가능 (상업 또는 비상업), 포크 및 커스터마이징 가능. gogomail을 제품 또는 관리 서비스로 판매 또는 호스팅하려면 저작권 소유자의 명시적 허가가 필요합니다.

Copyright (c) 2026 Park Jangwon.
