# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-010
- **제목**: Phase 4 — Drive WebDAV Gateway (RFC 4918)
- **배경**: Drive 파일을 WebDAV 프로토콜로 노출하여 외부 클라이언트가
  파일을 마운트하고 동기화할 수 있도록 한다.
- **구현 대상**: internal/webdavgw, PROPFIND, GET, PUT, DELETE, MKCOL

### 완료 조건

- [ ] `internal/webdavgw` 패키지: WebDAV 핸들러
- [ ] PROPFIND: 파일/폴더 속성 조회
- [ ] GET/PUT/DELETE/MKCOL 기본 지원
- [ ] 테스트: WebDAV ops, 속성 정확성
- [ ] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-011 — Phase 5-A Milter Adapter**

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
