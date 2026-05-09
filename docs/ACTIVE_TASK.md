# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-054
- **제목**: Milter Config & Runtime — SMTP 훅 통합
- **배경**: Phase 5-A 완성. Milter 클라이언트(TASK-051), circuit breaker(TASK-052), 
  pool(TASK-053) 모두 완성. 이제 설정 + 런타임에 SMTP 파이프라인 연동.
  
- **구현 대상**:
  - `internal/config/config.go`: Milter 설정 구조 추가
    - `MilterEnabled bool`
    - `MilterAddress string` (TCP 주소)
    - `MilterTimeout time.Duration` (기본 5초)
    - `MilterMaxConns int` (기본 10)
    - `MilterHealthCheckInterval time.Duration` (기본 30초)
  - `internal/app/run.go`: Milter 풀 초기화 + 종료
  - `internal/mailservice/smtp_adapter.go` 또는 SMTP 파이프라인:
    - `authentication_checked` 단계 후 RCPT TO 진행 전 milter RCPT 호출
    - `parsed` 단계 (MAIL FROM, HELO, HEADERS, BODY) 단계별 milter 호출
  - `internal/milterhook/milterhook.go`: Milter 훅 구현 (SMTP 파이프라인 통합)
    - `OnHelo()`, `OnMailFrom()`, `OnRcptTo()`, `OnHeaderEnd()`, `OnBodyChunk()`, `OnEndOfMessage()`
    - 각 메서드: pool에서 클라이언트 가져오기, 명령 보내기, Action 응답 처리
    - Action 처리: ACCEPT(통과), REJECT(거부), TEMPFAIL(임시 실패), DISCARD(폐기)
  - 환경 변수:
    - `GOGOMAIL_MILTER_ENABLED` (기본 false)
    - `GOGOMAIL_MILTER_ADDRESS` (예: "127.0.0.1:11332")
    - `GOGOMAIL_MILTER_TIMEOUT` (기본 5s)

- **완료 조건**:
  - [ ] `go test ./...` 통과 (기존 + 새 milter hook 테스트)
  - [ ] GOGOMAIL_MILTER_ENABLED=false일 때 milter 비활성화
  - [ ] GOGOMAIL_MILTER_ENABLED=true + valid address일 때 풀 초기화
  - [ ] HELO/MAIL/RCPT/HEADERS/BODY 단계에서 milter 호출
  - [ ] milter REJECT 응답 시 SMTP 거부 (error 반환)
  - [ ] milter ACCEPT 응답 시 SMTP 진행
  - [ ] circuit breaker 적용: 실패 시 자동 OPEN
  - [ ] milter 비활성 상태에서도 SMTP 정상 작동
  - [ ] docs 업데이트

- **다음 태스크**: TASK-055 (Milter Shadow Mode — 감시 모드)

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
