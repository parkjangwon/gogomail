# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-018
- **제목**: IMAP FETCH BODY 실제 클라이언트 픽스처 확장
- **배경**: `internal/imapgw` MIME literal fetch가 기본 케이스만 커버.
  Apple Mail, Thunderbird, K-9 Mail 형태의 `BODY[TEXT]`, `BODY[HEADER]`, `BODY[1.TEXT]`
  literal 응답 픽스처를 추가해 회귀 방지.
- **구현 대상**: `internal/imapgw/*_test.go` 픽스처 추가
- **완료 조건**:
  - [ ] `go test ./internal/imapgw/...` 통과
  - [ ] 새 픽스처 최소 5종 추가
- **다음 태스크**: TASK-019 — Drive 파일 공유 — Directory delegation 통합

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + docs), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```