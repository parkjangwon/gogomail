# gogomail Project Harness

This is the authoritative durable contract for all coding agents working on this repository.
`CLAUDE.md`, `AGENTS.md`, and `GEMINI.md` are thin pointers to this file.

---

## ⚠️ 루프 강제 실행 — 에이전트는 절대 멈추지 않는다

**이 규칙은 이 파일의 다른 모든 규칙보다 우선한다. 읽는 즉시 적용한다.**

- push 완료 후 **사용자 응답을 기다리지 않는다.** 즉시 다음 태스크로 전진한다.
- "태스크 완료"는 루프 종료 사유가 아니다.
- "할 일이 없다"는 없다 — `docs/NEXT_STEPS.md` 백로그가 항상 다음 항목을 갖는다.
- 블로커 없이 루프를 멈추는 것 자체가 **하네스 위반**이다.
- 루프가 멈추는 유일한 합법적 조건:
 1. 사용자가 명시적으로 중단을 요청할 때
 2. `go test ./...` 반복 실패 후 원인을 특정할 수 없을 때 (ACTIVE_TASK.md에 블로커 기록 후 대기)

### 🚨 COMPLETE 상태 처리 (루프 재진입 규칙)

`docs/ACTIVE_TASK.md`의 ID가 `COMPLETE`이면:

1. **절대 사용자를 기다리지 않는다.** `대기한다`는 문구를 무시한다.
2. `docs/NEXT_STEPS.md`의 `## 백로그` 섹션에서 첫 번째 미완료 태스크를 선택한다.
3. 백로그가 비어있으면 `docs/NEXT_STEPS.md`의 `Next:` 항목들에서 가장 구체적인 항목을 새 태스크로 만든다.
4. 그것도 없으면 `docs/backend-roadmap.md`에서 미구현 또는 하드닝이 필요한 항목을 선택한다.
5. 새 `ACTIVE_TASK.md`를 작성하고 **즉시 루프 1단계부터 재시작**한다.

---

## 📋 로드맵 범위 이탈 금지

- **모든 구현은 `docs/backend-roadmap.md`에 기술된 Phase/항목 내에서만 진행한다.**
- `docs/NEXT_STEPS.md` 백로그 항목은 반드시 `docs/backend-roadmap.md`에서 파생된다.
- `docs/ACTIVE_TASK.md`가 지시하지 않은 코드는 작성하지 않는다.
- 로드맵에 없는 새 기능을 임의로 추가하는 것은 금지.
- 백로그가 비어있으면 `docs/backend-roadmap.md`에서 다음 항목을 `docs/NEXT_STEPS.md`에 추가한 뒤 루프를 계속한다.

---

## 자율 개발 루프 (Autonomous Loop)

에이전트는 이 절차를 **번호 순서대로** 반복한다. 순서를 바꾸거나 건너뛰지 않는다.

```
1. docs/ACTIVE_TASK.md 읽기 — 현재 태스크와 완료 조건 확인
2. 실패하는 테스트 먼저 작성 (_test.go)
3. 테스트가 통과하도록 구현
4. go test ./... 실행 — 실패 시 3으로 돌아가기
5. 관련 docs 업데이트 (CURRENT_STATUS.md, backend-roadmap.md, openapi.yaml 등)
6. docs/ACTIVE_TASK.md 완료 체크리스트 전부 체크
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. docs/NEXT_STEPS.md 백로그에서 다음 태스크를 ACTIVE_TASK.md로 이동
10. 즉시 1로 돌아가기 — 사용자 응답을 기다리지 않는다. 루프는 영속한다.
```

### 절대 금지

- 4번(테스트 통과) 전에 커밋
- 5번(docs 업데이트) 없이 커밋
- 테스트 실패 상태에서 push
- ACTIVE_TASK.md를 건너뛰고 임의로 태스크 선택
- 로드맵/백로그에 없는 기능을 임의로 구현
- **루프를 멈추고 사용자 입력을 기다리는 것 (블로커가 없는 경우)**

### 루프 블로킹 조건

다음 상황에서만 루프를 멈추고 ACTIVE_TASK.md에 블로커를 기록한다:
- `go test ./...` 반복 실패 후 원인 불명
- 완료 조건이 모호해서 구현 방향 결정 불가

---

## ACTIVE_TASK.md 형식

```markdown
## ACTIVE_TASK

- **ID**: TASK-NNN
- **제목**: 한 줄 설명
- **구현 대상**: 어떤 파일/패키지
- **완료 조건**:
  - [ ] go test ./... 통과
  - [ ] 관련 기능 동작 확인
  - [ ] docs/CURRENT_STATUS.md 갱신
  - [ ] docs/backend-roadmap.md 해당 항목 체크
  - [ ] (API 변경 시) docs/openapi.yaml 갱신
- **다음 태스크**: NEXT_STEPS.md의 항목명 (사전 예고만, 자동 변경 금지)
```

에이전트는 이 파일만 읽는다. NEXT_STEPS.md는 완료 후 다음 태스크 선택 시에만 참조한다.

---

## Pre-commit Hook (엄격 차단)

`.git/hooks/pre-commit`이 다음 두 조건을 강제한다:

1. **`go test ./...` 통과** — 실패 시 커밋 차단
2. **프로덕션 코드 변경 시 docs/ 동시 스테이징** — `internal/*.go` (테스트 제외) 또는 `migrations/*.sql` 변경 시 `docs/` 파일이 최소 1개 스테이징돼야 함

---

## Push 정책

`go test ./...`가 통과한 커밋만 `git push origin main`한다.
실패 커밋은 로컬에서 수정 후 재커밋한다.

---

## 커밋 규칙

- **코드 + docs 반드시 같은 커밋** — 기능은 구현과 문서가 함께 있을 때 완료
- **docs 단독 커밋 금지** — docs만 push된 경우 fixup 커밋으로 합칠 것
- 커밋 메시지: `feat:`, `fix:`, `refactor:`, `test:`, `docs:` 컨벤셔널 형식

---

## 컨텍스트 파일 읽기 순서

작업 시작 전 반드시 읽어야 할 파일:
1. `PROJECT_HARNESS.md` (이 파일)
2. `docs/ACTIVE_TASK.md`
3. `docs/backend-roadmap.md`
4. `git log --oneline -10`

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
1. `go test ./...` 통과 (강제, pre-commit hook)
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
| `go test` 실행 생략 | 숨은 버그, 5375개 안전망 무효화 |
| 완료 조건 미확인 | 불완전한 구현, 다음 태스크 블로킹 |
| 실패 테스트 스킵 | TDD 원칙 위반, 회귀 불가능 |
| 문서 없이 커밋 | pre-commit hook 차단, push 실패 |
| RFC 무시하고 구현 | 호환성 깨짐, 나중에 다시 짜기 |

---

*Managed by make-harness. Last updated: 2026-05-08*
