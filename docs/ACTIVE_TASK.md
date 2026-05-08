# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-008
- **제목**: Phase 3-B — SCIM 2.0 Provisioning API (RFC 7642/7643/7644)
- **배경**: SCIM 2.0 표준을 구현하여 외부 IDP에서 사용자/그룹 프로비저닝을
  자동화한다. Create/Read/Update/Delete/List + Filter를 지원한다.
- **구현 대상**: internal/scim, Admin API, SCIM JSON 포맷

### 완료 조건

- [x] `internal/scim` 패키지: SCIM 2.0 User/Group CRUDL
- [x] Filter 파싱: userName, email, displayName 등
- [x] Bulk operations 지원
- [x] SCIM JSON 응답 포맷 (schemas, meta 등)
- [x] 테스트: CRUDL, Filter, Bulk
- [x] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-009 — Phase 3-C SAML 2.0 / OIDC SSO**

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
