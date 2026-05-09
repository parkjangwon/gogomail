# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-051
- **제목**: Milter Protocol Adapter — `internal/milter` 풀 + 연결 관리 ✅ (2026-05-09)
- **배경**: Phase 5-A. Milter 프로토콜 구현은 완료되었으나, 연결 풀 미구현.
  TCP 연결 풀, 유휴 큐 관리, maxConns 제한이 필요.

- **구현 완료**:
  - ✅ `internal/milter/pool.go`: 연결 풀 (Get/Put/Close)
  - ✅ `internal/milter/pool_test.go`: 풀 테스트 3개 (DialConnect, MaxConns, Reuses)
  - ✅ FIFO 유휴 큐, atomic 카운터, context 기반 대기
  - ✅ maxConns 제한 + 재연결 지원
  - ✅ `go test ./...` 통과 (5378 tests)
  - ✅ commit: TBD (다음 단계)

- **다음 태스크**: TASK-052 (Milter Hook Adapter — SMTP 파이프라인 통합)

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```
