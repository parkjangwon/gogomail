# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-005
- **제목**: Phase 2-D — 실시간 설정 전파 (SSE) + 스코프 보안
- **배경**: ConfigStore의 LISTEN/NOTIFY 기반 캐시 무효화를 확장하여,
  SSE(Server-Sent Events)를 통해 설정 변경을 실시간으로 클라이언트에 전파한다.
  또한 관리자가 user 스코프 설정을 직접 수정하지 못하도록 보안 제한을 추가한다.
- **구현 대상**: SSE 엔드포인트, configstore.Notifier, 스코프 보안

### 완료 조건

- [x] `internal/configstore.Notifier` 인터페이스 + subscriber fan-out
- [x] `GET /api/v1/config/stream` (사용자) + `GET /admin/v1/config/stream` (관리자) SSE 엔드포인트
- [x] 스코프 보안: `user` 스코프 관리자 직접 쓰기 차단 (403)
- [x] 테스트: DB 설정 변경 → SSE 이벤트 수신 통합 테스트
- [x] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-006 — Phase 2-E Open API 키 관리 (도메인 관리자용)**

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
