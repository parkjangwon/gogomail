# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-036
- **제목**: LDAP Gateway (RFC 4511) — Phase 3-A
- **배경**: `internal/ldapgw` 패키지에 BER 인코딩/디코딩 유틸만 존재. 실제 LDAP 서버 없음.
  `internal/directory` 리포지토리로 LDAP 필터를 SQL에 매핑하면 기존 사용자/주소 데이터를
  LDAP 클라이언트(CalDAV/CardDAV 리졸버, 메일 클라이언트)가 표준 프로토콜로 조회 가능.
- **구현 대상**:
  - `internal/ldapgw/server.go`: TCP 리스너 + LDAP v3 프로토콜 파서
    - `HandleBind`: simple bind → 기존 auth 경계에 위임 (directory.LookupUser)
    - `HandleSearch`: LDAP 필터 → `directory.SearchPrincipals` 매핑, `SearchResultEntry` 응답
    - read-only 적용: `Add/Modify/Delete/ModifyDN` → `unwillingToPerform`
    - `unbind` → 연결 종료
  - `internal/ldapgw/bind.go`: BindRequest BER 디코딩 + BindResponse 인코딩
  - `internal/ldapgw/search.go`: SearchRequest 디코딩 + SearchResultEntry/ResultDone 인코딩
  - `internal/ldapgw/server_test.go`: fake directory를使った 단위 테스트
  - `internal/app/run.go`: LDAP 서버를 AdminAPI 모드에 런타임 연동 (`ldapgw.NewServer`)
  - `internal/app/mode.go`: `ModeLDAPGateway` 추가
- **완료 조건**:
  - [x] `go test ./...` 통과
  - [x] BindRequest (simple bind) 처리 테스트
  - [x] SearchRequest → directory principals 매핑 + SearchResultEntry 응답 테스트
  - [x] read-only 연산 거부 테스트
  - [x] run.go에 LDAP 서버 등록
- **다음 태스크**: TASK-037 (백로그에서 선택)

---

## 완료됨

- **TASK-035**: SSE Config Stream — configstore.Notifier 연동 ✅ (2026-05-09)
- **TASK-034**: Batch Worker — Quota Alert Check (quota-alert-check 잡, Phase 2-C) ✅ (2026-05-09)
- **TASK-033**: Batch Worker — Token Cleanup (token-cleanup 잡) ✅ (2026-05-09)
- **TASK-032**: Batch Worker — TOTP Used-Code Pruning ✅ (2026-05-09)
- **TASK-031**: Delta Sync FanOut — mail.stored → deltasync.FanOut 연동 ✅ (2026-05-09)
- **TASK-030**: Delta Sync Cursor — Postgres 영속 스토어 ✅ (2026-05-09)

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
