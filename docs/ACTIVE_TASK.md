# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-015
- **제목**: Phase 7-B — Delta Sync Boundary
- **배경**: 디바이스별 델타 싱크 커서와 IMAP IDLE 팬아웃을
  구현하여 실시간 동기화를 지원한다.
- **구현 대상**: internal/deltasync, 커서 관리, IMAP IDLE 연동

### 완료 조건

- [ ] `internal/deltasync` 패키지: DeltaSync 경계
- [ ] 디바이스별 delta-sync cursor 관리
- [ ] IMAP IDLE fan-out 구조
- [ ] 테스트: 커서 생성, 갱신, 만료, 팬아웃
- [ ] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
백로그가 비었으므로 `docs/backend-roadmap.md`에서 다음 Phase 항목을 추가한다.

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
