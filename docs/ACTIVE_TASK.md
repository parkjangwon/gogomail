# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-012
- **제목**: Phase 5-B — DNSBL (RFC 5782)
- **배경**: SMTP 수신 경로에 DNSBL 조회를 추가하여 스팸/악성 IP를
  차단한다.
- **구현 대상**: internal/dnsbl, DNSBL 조회 및 평가 경계

### 완료 조건

- [ ] `internal/dnsbl` 패키지: DNSBL 조회 인터페이스 및 구현
- [ ] SMTP 수신 경로에 DNSBL 체크 플러그인 추가
- [ ] RFC 5782 기반 DNSBL 쿼리 (A 레코드 조회, 반환 코드 해석)
- [ ] 테스트: DNSBL 조회 모킹, 허용/차단/목록 없음 시나리오
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
