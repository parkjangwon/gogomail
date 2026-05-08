# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-013
- **제목**: Phase 6 — POP3 Server (RFC 1939)
- **배경**: POP3 프로토콜을 구현하여 메일 클라이언트가 수신 메일을
  다운로드하고 관리할 수 있게 한다.
- **구현 대상**: internal/pop3d, POP3 명령 처리, UIDL/TOP/STLS 확장

### 완료 조건

- [ ] `internal/pop3d` 패키지: POP3 서버 구현
- [ ] RFC 1939 기본 명령: USER, PASS, STAT, LIST, RETR, DELE, NOOP, RSET, QUIT
- [ ] 확장: UIDL, TOP, STLS, CAPA, AUTH
- [ ] POP3S (포트 995) 지원
- [ ] 테스트: 명령 시퀀스, 인증, 메일 다운로드, 삭제
- [ ] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-013 — Phase 6 POP3 Server (RFC 1939)**

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
