# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-014
- **제목**: Phase 7-A — FCM / APNs / Web Push Adapters
- **배경**: 푸시 알림 어댑터를 구현하여 모바일/웹 클라이언트에
  실시간 알림을 전달한다.
- **구현 대상**: internal/pushnotify, PushSink 인터페이스, FCM/APNs/WebPush

### 완료 조건

- [ ] `internal/pushnotify` 패키지: PushSink 인터페이스 + 어댑터
- [ ] FCM (Firebase Cloud Messaging) 어댑터
- [ ] APNs (Apple Push Notification service) 어댑터
- [ ] Web Push (RFC 8030) 어댑터
- [ ] device_tokens 테이블 연동
- [ ] 테스트: 푸시 전송, 토큰 관리, 어댑터 선택
- [ ] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-015 — Phase 7-B Delta Sync Boundary**

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
