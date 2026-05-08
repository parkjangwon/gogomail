# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-019
- **제목**: Drive 파일 공유 — Directory delegation 통합
- **배경**: `internal/drive` HTTP API는 구현됨. `internal/accesspolicy.DelegatedAccessAuthorizer`도 존재.
  Drive HTTP 엔드포인트에 `drive` scope delegation 체크가 없어 크로스 유저 접근이 미구현.
- **구현 대상**: `internal/httpapi/drive.go` — cross-user 경로에 `DelegatedAccessAuthorizer` 적용
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] 위임된 read/write/manage 롤로 Drive 접근 테스트
- **다음 태스크**: TASK-020 — OpenAPI → TypeScript 클라이언트 생성

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