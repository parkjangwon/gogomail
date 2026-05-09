# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-052
- **제목**: Milter Circuit Breaker — State Machine + Failure Recovery ✅ (2026-05-09)
- **배경**: Phase 5-A 항목 9. 연결 풀(TASK-051) 다음 단계.
  외부 milter 서버 다운 시 빠르게 실패하고 복구하는 circuit breaker 필요.

- **구현 완료**:
  - ✅ `internal/milter/circuit.go`: State machine (CLOSED → OPEN → HALF_OPEN → CLOSED)
  - ✅ `internal/milter/circuit_test.go`: 6개 테스트
    - ClosedState, OpensAfterFailures, TransitionsToHalfOpen
    - ClosesAfterSuccess, ReopensAfterFailureInHalfOpen, Metrics
  - ✅ failureThreshold + resetTimeout 설정 가능
  - ✅ AllowRequest() 상태별 제어 (CLOSED/OPEN/HALF_OPEN)
  - ✅ Atomic 메트릭 (success/failure count)
  - ✅ `go test ./...` 통과 (5385 tests)
  - ✅ commit: TBD

- **다음 태스크**: TASK-053 (Milter Pool Integration — Circuit breaker 연동)

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
