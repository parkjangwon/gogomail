# gogomail — Gemini CLI

**전체 계약은 [PROJECT_HARNESS.md](PROJECT_HARNESS.md)에 있다. 이 파일보다 PROJECT_HARNESS.md가 우선한다.**

## 빠른 참조

- 스택: Go · `go test ./...` · `go build ./...`
- **루프**: `docs/ACTIVE_TASK.md` 읽기 → 실패 테스트 작성 → 구현 → docs 갱신 → 커밋 → push
- **Pre-commit hook**: 테스트 실패 또는 docs 누락 시 커밋 차단
- **Push**: `go test ./...` 통과 커밋만 자동 push
- **RFC 준수**: 필수, 선택 아님
- **프론트엔드 게이트**: 앱 구현 전 사용자에게 알리고 대기

## 현재 태스크

→ `docs/ACTIVE_TASK.md` 를 읽어라. 다른 파일을 먼저 읽지 않는다.
