# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-007
- **제목**: Phase 3-A — LDAP Gateway (RFC 4511)
- **배경**: LDAP v3 프로토콜 게이트웨이를 구현하여 외부 디렉토리 서비스가
  gogomail 사용자/그룹 정보를 조회할 수 있도록 한다.
- **구현 대상**: internal/ldapgw, LDAP v3 프로토콜, Bind/Search

### 완료 조건

- [x] `internal/ldapgw` 패키지: LDAP v3 프로토콜 리스너
- [x] BindRequest (simple bind), SearchRequest 지원
- [x] Read-only 강제: Modify/Delete/ModifyDN → `unwillingToPerform`
- [x] LDAPS (포트 636) + StartTLS 지원
- [x] 테스트: Bind 성공/실패, Search 결과, Read-only 거부
- [x] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-008 — Phase 3-B SCIM 2.0 Provisioning API**

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
