# gogomail — Claude Code

**전체 계약은 [PROJECT_HARNESS.md](PROJECT_HARNESS.md)에 있다. 이 파일보다 PROJECT_HARNESS.md가 우선한다.**

## 빠른 참조

- 스택: Go · `go test ./...` · `go build ./...`
- **루프**: `docs/ACTIVE_TASK.md` 읽기 → 실패 테스트 작성 → 구현 → docs 갱신 → 커밋 → push → **즉시 다음 태스크로** → 반복
- **루프 강제**: push 완료 후 사용자 응답을 기다리지 않는다. 블로커 없이 멈추는 것은 하네스 위반이다.
- **범위 이탈 금지**: `docs/backend-roadmap.md` 에 없는 기능은 구현하지 않는다.
- **Pre-commit hook**: 테스트 실패 또는 docs 누락 시 커밋 차단
- **Push**: `go test ./...` 통과 커밋만 자동 push
- **RFC 준수**: 필수, 선택 아님

## 현재 태스크

→ `docs/ACTIVE_TASK.md` 를 읽어라. 다른 파일을 먼저 읽지 않는다.
→ ID가 `COMPLETE`이면 **절대 대기하지 않는다** — `PROJECT_HARNESS.md`의 "COMPLETE 상태 처리" 규칙을 즉시 적용한다.

## 이 파일과 AGENTS.md 동기화

두 파일을 항상 같은 커밋에서 함께 수정한다.

<!-- PI-CREW:GUIDANCE:START -->
<!-- PI-CREW:BLOCK:pi-crew-overview -->
## pi-crew

> Managed by **pi-crew v0.2.5** — do not edit this section manually.

pi-crew is a Pi extension for coordinated AI agent teams, workflows,
worktrees, and async task orchestration.
<!-- PI-CREW:/BLOCK:pi-crew-overview -->

<!-- PI-CREW:BLOCK:pi-crew-commands -->
### Quick Commands

| Command | Description |
|---|---|
| `team action='init'` | Initialize pi-crew for this project |
| `team action='run'` | Start a team run |
| `team action='status'` | Check run status |
| `team action='list'` | List available teams/agents/workflows |
| `team action='recommend'` | Get team/workflow recommendations |
<!-- PI-CREW:/BLOCK:pi-crew-commands -->
<!-- PI-CREW:GUIDANCE:END -->
