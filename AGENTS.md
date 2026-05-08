# gogomail — Codex / OpenAI Agents

**전체 계약은 [PROJECT_HARNESS.md](PROJECT_HARNESS.md)에 있다. 이 파일보다 PROJECT_HARNESS.md가 우선한다.**

## 빠른 참조

- 스택: Go · `go test ./...` · `go build ./...`
- **루프**: `docs/ACTIVE_TASK.md` 읽기 → 실패 테스트 작성 → 구현 → docs 갱신 → 커밋 → push → **즉시 다음 태스크로** → 반복
- **루프 강제**: push 완료 후 사용자 응답을 기다리지 않는다. 블로커 없이 멈추는 것은 하네스 위반이다.
- **범위 이탈 금지**: `docs/backend-roadmap.md` 에 없는 기능은 구현하지 않는다.
- **Pre-commit hook**: 테스트 실패 또는 docs 누락 시 커밋 차단
- **Push**: `go test ./...` 통과 커밋만 자동 push
- **RFC 준수**: 필수, 선택 아님
- **프론트엔드 게이트**: 앱 구현 전 사용자에게 알리고 대기

## 현재 태스크

→ `docs/ACTIVE_TASK.md` 를 읽어라. 다른 파일을 먼저 읽지 않는다.

## 이 파일과 CLAUDE.md 동기화

두 파일을 항상 같은 커밋에서 함께 수정한다.
