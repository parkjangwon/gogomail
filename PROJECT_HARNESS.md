# gogomail Project Harness

This is the authoritative durable contract for all coding agents working on this repository.
`CLAUDE.md` and `AGENTS.md` are thin pointers to this file.

---

## 운영 모드 — 사용자 주도 자율 실행

기본 모드는 **사용자 요청 단위 실행**이다. 에이전트는 요청받은 작업을
끝까지 수행하고 검증하지만, 완료 후 백로그를 임의로 물고 늘어지지 않는다.

- 현재 사용자 프롬프트가 항상 최우선 작업 범위다.
- `docs/ACTIVE_TASK.md`는 하네스 루프나 장기 작업의 durable 상태로 사용한다.
- push 완료 후에는 결과를 보고하고 멈춘다. 단, 사용자가 명시적으로
  "다음 태스크 계속", "하네스 루프", "autopilot"처럼 연속 실행을 요청한
  경우에만 다음 백로그로 전진한다.
- 로컬에서 되돌릴 수 있는 조사/수정/검증은 묻지 말고 진행한다.
- 파괴적 작업, 외부 운영환경 변경, 권한/비밀이 필요한 작업, 범위가 크게
  갈리는 결정은 멈추고 사용자 지시를 받는다.

### COMPLETE 상태 처리

`docs/ACTIVE_TASK.md`의 ID가 `COMPLETE`이면:

1. 활성 하네스 태스크가 없다는 뜻으로 해석한다.
2. 현재 사용자 프롬프트를 작업 범위로 삼는다.
3. 사용자가 명시적으로 연속 하네스 실행을 요청한 경우에만
   `docs/NEXT_STEPS.md`의 다음 미완료 백로그를 `ACTIVE_TASK.md`로 승격한다.
4. 일반 대화형 요청에서는 `ACTIVE_TASK.md`를 새 태스크로 덮어쓰지 않는다.

---

## 📋 로드맵 범위 이탈 금지

- 제품 기능 구현은 `docs/backend-roadmap.md` 또는 사용자의 명시 요청에서
  출발해야 한다.
- `docs/NEXT_STEPS.md` 백로그 항목은 가능한 한 `docs/backend-roadmap.md`에서
  파생한다.
- 로드맵에 없는 새 기능을 임의로 추가하지 않는다.
- 문서/하네스/운영 런북 정비는 사용자 요청이 있으면 로드맵 외 작업으로
  허용한다. 이때 동작 변경 없이 문서 정확성과 에이전트 효율을 개선한다.
- 백로그가 비어 있더라도 사용자가 연속 실행을 요청하지 않았다면 새 작업을
  자동 생성하지 않는다.

---

## 요청 단위 실행 루프

에이전트는 요청받은 범위 안에서 다음 절차를 따른다.

```
1. docs/ACTIVE_TASK.md 읽기 — non-COMPLETE이면 현재 사용자 요청과 충돌 여부 확인
2. 완료 조건과 검증 방법을 정한다
3. 코드 동작 변경이면 실패 테스트 또는 가장 좁은 회귀 검증을 먼저 준비한다
4. 구현/문서 수정을 수행한다
5. 변경 범위에 맞는 검증을 실행한다
   - docs/json only: `git diff --check` + JSON/YAML 검증
   - Go backend: targeted test 후 필요 시 `go test ./...`
   - frontend: 해당 앱의 test/type-check
6. 관련 docs 업데이트 (CURRENT_STATUS.md, backend-roadmap.md, openapi.yaml 등)
7. 요청받았거나 하네스 소유 작업이면 git add, Lore 커밋, push
8. 결과와 검증 근거를 보고한다. 명시적 연속 실행이 아니면 여기서 멈춘다.
```

### 절대 금지

- 관련 검증 전에 커밋
- 프로덕션 코드/API/운영 동작 변경 후 docs 업데이트 없이 커밋
- 테스트 실패 상태에서 push
- 사용자가 요청하지 않은 백로그 항목을 자동 구현
- 로드맵/사용자 요청에 없는 기능을 임의로 구현
- 비밀/권한/운영환경 변경이 필요한 작업을 확인 없이 수행

### 루프 블로킹 조건

다음 상황에서만 루프를 멈추고 ACTIVE_TASK.md에 블로커를 기록한다:
- 관련 테스트 반복 실패 후 원인 불명
- 완료 조건이 모호해서 구현 방향 결정 불가
- 권한, 비밀, 외부 운영환경 접근이 필요함

---

## ACTIVE_TASK.md 형식

```markdown
## ACTIVE_TASK

- **ID**: TASK-NNN
- **제목**: 한 줄 설명
- **구현 대상**: 어떤 파일/패키지
- **완료 조건**:
  - [ ] 관련 검증 통과 (`go test ./...`, frontend type-check, docs 검증 등)
  - [ ] 관련 기능 동작 확인
  - [ ] docs/CURRENT_STATUS.md 갱신
  - [ ] docs/backend-roadmap.md 해당 항목 체크
  - [ ] (API 변경 시) docs/openapi.yaml 갱신
- **다음 태스크**: NEXT_STEPS.md의 항목명 (사전 예고만, 자동 변경 금지)
```

하네스 루프에서는 `ACTIVE_TASK.md`를 durable source of truth로 사용한다.
일반 대화형 요청에서는 사용자 프롬프트가 우선이며, `NEXT_STEPS.md`는
연속 실행을 명시적으로 요청받았을 때만 다음 태스크 선택에 사용한다.

---

## Pre-commit Hook (엄격 차단)

`.git/hooks/pre-commit`이 다음 두 조건을 강제한다:

1. **`go test -short ./...` 통과** — 실패 시 커밋 차단
2. **프로덕션 코드 변경 시 docs/ 동시 스테이징** — `internal/*.go`, `cmd/*.go` (테스트 제외) 또는 `migrations/*.sql` 변경 시 `docs/` 파일이 최소 1개 스테이징돼야 함

---

## Push 정책

pre-commit의 `go test -short ./...`가 통과한 커밋만 push한다.
백엔드/프로토콜/공유 로직 변경은 push 전에 `go test ./...`를 추가로 실행한다.
프론트엔드 변경은 해당 앱의 test/type-check를 실행한다.

---

## 커밋 규칙

- **코드 + docs 반드시 같은 커밋** — 기능은 구현과 문서가 함께 있을 때 완료
- **docs 단독 커밋 허용** — 문서/하네스/런북 정비가 사용자 요청이거나 실제
  상태 불일치를 고치는 경우 허용한다
- 커밋 메시지: AGENTS.md의 Lore Commit Protocol을 따른다. 컨벤셔널 prefix는
  선택 사항이며 Lore trailer를 대체하지 않는다.

---

## 컨텍스트 파일 읽기 순서

작업 시작 전 반드시 읽어야 할 파일:
1. `docs/ACTIVE_TASK.md`
2. `PROJECT_HARNESS.md` (하네스/워크플로 판단이 필요한 경우)
3. `docs/backend-roadmap.md` (제품 기능 범위 판단이 필요한 경우)
4. `git log --oneline -10` (커밋/히스토리 판단이 필요한 경우)

필요 시 추가로 읽는 파일:
- `docs/CURRENT_STATUS.md`
- `docs/openapi.yaml`
- `docs/adr/` 관련 항목

---

## RFC 표준 준수 (필수, 선택 아님)

gogomail은 실제 클라이언트와의 상호운용을 위해 RFC 표준 준수가 핵심 요구사항이다.

SMTP, 메시지 파싱, 헤더, MIME, 인증, DNS, 전송, 바운스 처리, IMAP 관련 구현 시:

| RFC | 표준 |
|-----|------|
| RFC 5321 | SMTP |
| RFC 5322 | Internet Message Format |
| RFC 2045–2049 | MIME |
| RFC 2047 | 비ASCII 헤더 인코딩 |
| RFC 6531/6532 | SMTPUTF8 |
| RFC 6376 | DKIM |
| RFC 7208 | SPF |
| RFC 7489 | DMARC |
| RFC 3461/3464 | DSN |
| RFC 3501+ | IMAP |

**Ad-hoc string parsing 금지** — 프로토콜 파싱/직렬화는 검증된 라이브러리를 사용한다.

---

## EML 파서 성능 요구사항

EML 파서는 SMTP 수신, Mail API, IMAP, 검색 인덱싱의 핫 패스에 있다.

- 전체 메시지를 메모리에 로드하지 않는다 — 스트리밍 우선
- 불필요한 본문 복사 금지
- 메시지/파트/헤더 최대 크기 강제
- 파서 변경 시 `go test -bench` 및 할당량 검사 실행

---

## SMTP 파이프라인 확장성

스팸 처리는 별도 모듈로 구현 예정이다. SMTP 코어에 스팸 로직을 넣지 않는다.

SMTP 코어가 정의할 수 있는 것: 인터페이스, 훅 스테이지, 봉투, 트레이스 헤더, 결과 캐리어
SMTP 코어 밖에 있어야 할 것: SPF/DKIM/DMARC 검사 구현, 스팸 스코어링, 격리 결정, 외부 스팸 릴레이

파이프라인 단계: 이미지 변환, FCM 푸시, 감사 로깅, 스팸 릴레이 핸드오프, 첨부파일 스캔, 인덱싱, 테넌트 정책

---

## 아키텍처 원칙

- 작고 구성 가능한 인터페이스 — 모놀리식 컴포넌트 금지
- 핫 패스는 효율적이고 할당 인식적으로
- 프로토콜 엔진과 제품 기능 분리
- 동작을 인터페이스/훅/어댑터로 교체 가능하게
- 벤더별 서비스를 SMTP/메시지 코어에 하드코딩 금지

---

## 제품 개념

gogomail은 극도의 성능, 확장성, 가용성을 갖춘 개발자 친화적 메일 플랫폼이다.
국가급 규모 배포를 장기 목표로 설계한다.

의사결정 필터:
- 실제 클라이언트 상호운용성을 위한 프로토콜 정확성 우선
- 백프레셔, 큐, 재시도, 멱등성, 관찰 가능성 우선
- 운영 탄력성, 명확한 실패 모드, 우아한 저하를 핵심 제품 기능으로

---

## 프론트엔드 작업 규칙

백엔드 계약, API 준비, 프론트엔드 계획은 자율적으로 진행 가능.

`apps/shell`, `apps/webmail`, `apps/console`, 공유 UI 패키지, 실제 Next.js 화면을 구현할 때는 다음 규칙을 따른다:
- **`docs/DESIGN.md`를 반드시 먼저 읽는다** — 컬러 토큰, 레이아웃, 컴포넌트 패턴, 금지사항 전부 이 파일에 있다.
- 디자인 시스템 토큰을 임의로 추가하거나 변경하지 않는다.
- DESIGN.md에 없는 컴포넌트 패턴이 필요하면 이 파일을 먼저 업데이트하고 구현한다.

---

## ADR 작성 기준

다음 변경 시 `docs/adr/` 아래 ADR을 작성한다:
테넌트/도메인 경계, SMTP 코어 경계, 큐/스토리지/인증/정책 아키텍처, 전송 라우팅,
프론트엔드 시작 게이트, 스팸 통합 전략, API 계약 버전 관리, 릴리즈/배포 아키텍처

---

## 토큰 절약 원칙 (품질 유지 필수)

gogomail의 품질은 다음 세 가지에 의존한다:
1. `go test -short ./...` 통과 (강제, pre-commit hook)
2. 문서 동시 커밋 (강제, pre-commit hook)
3. RFC 표준 준수 (아키텍처)

**이 세 가지는 절대 줄일 수 없다.**

### 안전한 절약 방법 (품질 무영향)

- **NEXT_STEPS.md**: 전체 읽지 말 것 → 다음 3-5개 태스크만 읽기
- **git log**: 기본 `-20`, 필요하면 `-50`까지만 (전체 히스토리 X)
- **docs/adr/**: 아키텍처 결정 변경할 때만 읽기 (매 태스크마다 X)
- **docs/openapi.yaml**: API 변경할 때만 읽기
- **병렬 읽기 우선**: Read 여러 개를 한 번에 수행
- **읽은 파일 캐싱**: 같은 세션 내 재읽기 금지

**효과**: 토큰 15-20% 절약, 품질 100% 유지

### 절대 금지 (품질 저하)

| 금지 항목 | 이유 |
|----------|------|
| 필요한 테스트 실행 생략 | 숨은 버그, 테스트 안전망 무효화 |
| 완료 조건 미확인 | 불완전한 구현, 다음 태스크 블로킹 |
| 실패 테스트 스킵 | TDD 원칙 위반, 회귀 불가능 |
| 문서 없이 커밋 | pre-commit hook 차단, push 실패 |
| RFC 무시하고 구현 | 호환성 깨짐, 나중에 다시 짜기 |

---

*Managed by make-harness. Reviewed and pruned: 2026-05-31*
