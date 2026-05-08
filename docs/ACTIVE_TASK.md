# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-017
- **제목**: CalDAV/CardDAV 네이티브 클라이언트 호환성 픽스처
- **배경**: Phase 4-B 하드닝 항목. Apple iCal, Thunderbird Lightning, DAVx⁵ 실제 요청 형태를
  픽스처로 캡처해 `internal/caldavgw` / `internal/carddavgw` 회귀 테스트 추가.
- **구현 대상**: `internal/caldavgw/*_test.go`, `internal/carddavgw/*_test.go` 픽스처 추가
- **완료 조건**:
  - [ ] `go test ./internal/caldavgw/... ./internal/carddavgw/...` 통과
  - [ ] 새 픽스처 최소 5종 추가
- **다음 태스크**: TASK-018 — IMAP FETCH BODY 실제 클라이언트 픽스처 확장

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