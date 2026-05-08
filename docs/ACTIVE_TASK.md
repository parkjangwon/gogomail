# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-009
- **제목**: Phase 3-C — SAML 2.0 / OIDC SSO
- **배경**: SAML 2.0 및 OIDC 기반 Single Sign-On을 지원하여
  기업 고객이 외부 IdP와 연동할 수 있도록 한다.
- **구현 대상**: internal/sso, SAML metadata, OIDC discovery

### 완료 조건

- [x] `internal/sso` 패키지: SAML 2.0 SP 지원
- [x] SAML AuthnRequest 생성 / Assertion Consumer Service
- [x] OIDC Discovery + Authorization Code Flow
- [x] JWT ID token 검증
- [x] 테스트: SAML request/response, OIDC flow
- [x] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

백로그에 추가 태스크가 없으면 새로운 기능을 제안한다.

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
