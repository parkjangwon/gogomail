# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-003
- **제목**: Phase 2-B — 2FA / TOTP (RFC 6238)
- **배경**: Runtime config store의 `auth.mfa.mode` 설정을 기반으로 TOTP 기반 2FA를 구현한다.
  사용자는 선택적/강제 2FA를 설정할 수 있으며, 로그인 시 TOTP 코드 검증이 필요하다.
- **구현 대상**: migration, internal/authmfa, JWT claims, auth flow 연동

### 완료 조건

- [ ] Migration: `user_mfa_secrets`, `totp_used_codes` 테이블
- [ ] `internal/authmfa` 패키지: TOTP 생성/검증, ±2 window, 리플레이 방지
- [ ] Recovery codes (8개 단일 사용)
- [ ] Auth flow 연동: `auth.mfa.mode` 설정값 기반 강제/선택/비활성화
- [ ] JWT 클레임 `mfa_verified: true` 추가
- [ ] 테스트: TOTP 생성/검증, window, 리플레이 방지, recovery codes
- [ ] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-004 — Phase 2-C Batch Worker & Distributed Job Lock**

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
