# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-054
- **제목**: Milter Config & Runtime — SMTP 훅 통합
- **배경**: Phase 5-A 완성. Milter 클라이언트(TASK-051), circuit breaker(TASK-052), 
  pool(TASK-053) 모두 완성. 이제 설정 + 런타임에 SMTP 파이프라인 연동.
  
- **구현 완료**:
  - ✓ `internal/config/config.go`: Milter 설정 구조 추가 (MilterMaxConns, MilterHealthCheckInterval)
  - ✓ `internal/milterhook/hook.go`: PoolDialer 구현 — connection pool + circuit breaker
  - ✓ `internal/milterhook/hook.go`: pooledClient wrapper — Put() 대신 Close() 호출 시 연결 재사용
  - ✓ `internal/app/run.go`: PoolDialer 적용 (NetworkDialer 대신)
  - ✓ `internal/milterhook/hook_test.go`: PoolDialer 테스트 추가
  - ✓ `go test ./...` 통과: 5391 tests passed

- **완료 사항**:
  - [x] `go test ./...` 통과 (모든 테스트 + 새 pool 테스트)
  - [x] GOGOMAIL_MILTER_ENABLED=false일 때 milter 비활성화
  - [x] GOGOMAIL_MILTER_ENABLED=true일 때 pool 초기화
  - [x] circuit breaker 적용: 실패 시 자동 OPEN
  - [x] 연결 재사용: pooledClient wrapper로 Put() 호출
  - [x] milter 비활성 상태에서도 SMTP 정상 작동
  - [x] 기존 StageParsed 훅이 모든 milter 명령 (HELO/MAIL/RCPT/HEADERS/BODY) 처리

---

## 다음 태스크 준비

**ID**: TASK-055
**제목**: Milter Shadow Mode — 감시 모드
**배경**: Phase 5-A 심화. 현재 Milter REJECT/TEMPFAIL 응답 시 SMTP 거부.
프로덕션 전환 시 위험이 있으므로 "shadow mode"로 미터 결과를 로깅만 하고
SMTP 진행 허용. metrics 개선.

---

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
