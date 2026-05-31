# gogomail — Claude Code

**전체 계약은 [PROJECT_HARNESS.md](PROJECT_HARNESS.md)에 있다. 이 파일보다 PROJECT_HARNESS.md가 우선한다.**

## 빠른 참조

- 스택: Go · commit gate `go test -short ./...` · full backend `go test ./...` · `go build ./...`
- **실행 단위**: 현재 사용자 요청을 우선한다. `docs/ACTIVE_TASK.md`가 non-COMPLETE이면 충돌 여부를 확인한다.
- **연속 루프**: 사용자가 명시적으로 하네스 루프/다음 태스크 진행을 요청한 경우에만 push 후 백로그로 전진한다.
- **범위 이탈 금지**: 제품 기능은 `docs/backend-roadmap.md` 또는 사용자 명시 요청에서 출발한다.
- **Pre-commit hook**: `go test -short ./...` 실패 또는 production 코드 docs 누락 시 커밋 차단
- **Push**: 검증 통과 커밋만 push한다. 백엔드 변경은 필요 시 `go test ./...`까지 확인한다.
- **RFC 준수**: 필수, 선택 아님

## 현재 태스크

→ `docs/ACTIVE_TASK.md` 를 읽어라. 다른 파일을 먼저 읽지 않는다.
→ ID가 `COMPLETE`이면 활성 하네스 태스크가 없는 상태다. 현재 사용자 요청을 작업 범위로 삼고, 명시적 연속 실행 요청 없이 다음 백로그를 자동 생성하지 않는다.

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
