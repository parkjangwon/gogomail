# gogomail Project Harness

This is the authoritative durable contract for all coding agents working on this repository.
`CLAUDE.md`, `AGENTS.md`, and `GEMINI.md` are thin pointers to this file.

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
10. 1로 돌아가기
```

### 절대 금지

- 4번(테스트 통과) 전에 커밋
- 5번(docs 업데이트) 없이 커밋
- 테스트 실패 상태에서 push
- ACTIVE_TASK.md를 건너뛰고 임의로 태스크 선택
- frontend 앱 구현 시작 (아래 프론트엔드 게이트 참고)

### 루프 블로킹 조건

다음 상황에서는 루프를 멈추고 ACTIVE_TASK.md에 블로커를 기록한다:
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
  - [ ] (API 변경 시) docs/backend-api-contracts.md, openapi.yaml 갱신
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
- `docs/backend-api-contracts.md`
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

## 프론트엔드 게이트 (필수 일시정지)

백엔드 계약, API 준비, 프론트엔드 계획은 자율적으로 진행 가능.

그러나 `apps/shell`, `apps/webmail`, `apps/admin`, 공유 UI 패키지, 실제 Next.js 화면 구현 시작 전에:
**반드시 사용자에게 알리고 프론트엔드 관련 지시를 기다린다.**

---

## ADR 작성 기준

다음 변경 시 `docs/adr/` 아래 ADR을 작성한다:
테넌트/도메인 경계, SMTP 코어 경계, 큐/스토리지/인증/정책 아키텍처, 전송 라우팅,
프론트엔드 시작 게이트, 스팸 통합 전략, API 계약 버전 관리, 릴리즈/배포 아키텍처

---

*Managed by make-harness. Last updated: 2026-05-08*
