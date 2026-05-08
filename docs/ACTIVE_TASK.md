# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-011
- **제목**: Phase 5-A — Milter Adapter
- **배경**: Sendmail Milter 프로토콜을 구현하여 외부 필터/스캐너와
  SMTP 메일 흐름을 연동한다.
- **구현 대상**: internal/milter, SMFIC_* 명령 처리

### 완료 조건

- [ ] `internal/milter` 패키지: Milter 프로토콜 핸들러
- [ ] SMFIC_CONNECT, SMFIC_HELO, SMFIC_MAIL, SMFIC_RCPT, SMFIC_DATA, SMFIC_EOB 지원
- [ ] SMFIR_CONTINUE, SMFIR_REJECT, SMFIR_TEMPFAIL 응답
- [ ] 테스트: Milter 명령 인코딩/디코딩
- [ ] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-012 — Phase 5-B DNSBL (RFC 5782)**

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
