# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: COMPLETE
- **제목**: All backlog tasks completed
- **배경**: BACKLOG.md의 모든 태스크(TASK-002 ~ TASK-015)가 완료되었다.
  docs/backend-roadmap.md의 모든 Phase(0 ~ 7)가 구현되었다.

### 상태

- [x] 모든 백로그 태스크 완료
- [x] 모든 로드맵 Phase 구현 완료

### 다음 단계

사용자로부터 새로운 백로그 항목이나 기능 요청을 받을 때까지
대기한다. 또는 docs/backend-roadmap.md에 새 Phase를 추가한다.

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
