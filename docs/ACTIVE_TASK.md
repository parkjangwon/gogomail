# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-006
- **제목**: Phase 2-E — Open API 키 관리 (도메인 관리자용)
- **배경**: 도메인 관리자가 프로그래밍 방식으로 API를 호출할 수 있도록
  API 키를 생성하고 관리하는 기능을 구현한다. CIDR 기반 접근 제어와
  스코프 검증을 포함한다.
- **구현 대상**: migration, internal/apikeys, ApiKeyMiddleware, Admin API

### 완료 조건

- [x] Migration: `domain_api_keys` 테이블 (CIDR 배열 포함)
- [x] `internal/apikeys` 패키지: 키 생성/검증/CIDR 체크
- [x] `ApiKeyMiddleware`: `gm_` prefix 감지 → JWT 경로와 분기
- [x] Admin API CRUD + rotate 엔드포인트
- [x] 기존 Mail/Calendar/Contacts API에 scope 검증 레이어
- [x] 테스트: CIDR 허용/차단, 스코프 부족 거부, 만료/폐기 키 거부, rotate 후 구 키 무효화
- [x] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-007 — Phase 3-A LDAP Gateway (RFC 4511)**

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
