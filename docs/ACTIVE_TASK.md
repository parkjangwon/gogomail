# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-062
- **제목**: Spam 필터 하드닝 — RFC 5764 Milter 표준 + 스코링
- **배경**: Phase 8-B. TASK-061(조직도) 완료 후, spam filtering을 
  Milter 표준 프로토콜(RFC 5764)로 외부 스팸 필터(Rspamd, SpamAssassin 등)와 연동.

- **구현 완료**:
  1. ✓ `internal/spam/relay.go` — Relay 인터페이스 및 Hook (기존 구조)
  2. ✓ `internal/spam/filter.go` — 스팸 스코어링 및 판정 로직:
     - SpamScore struct: 점수, 룰 매칭 결과
     - Filter interface: 실제 스팸 필터 구현체 추상화
     - DecisionEngine: Rspamd 호환 스코어 기반 액션 결정
  3. ✓ `internal/spam/filter_test.go` — filter 테스트 (8개 테스트)
  4. ✓ `internal/milterhook/spam_integration.go` — Milter hook과 spam 통합:
     - SpamConfig: 필터 활성화 및 설정
     - SpamHook: SMTP StageAuthenticationChecked에서 스팸 체크
     - buildMessageText: 파싱된 메시지에서 텍스트 추출
     - MilterSpamVerdict: spam verdict → Milter action 변환
     - SpamVerdictHeaders: X-Spam-* 헤더 생성
  5. ✓ `internal/milterhook/spam_integration_test.go` — 통합 테스트 (5개 테스트)

- **완료 확인**:
  - [x] `go test ./...` 통과: 5483 tests passed
  - [x] spam filter 테스트: 스코어 계산 (8개 테스트)
     - TestDecisionEngineAccept
     - TestDecisionEngineQuarantine
     - TestDecisionEngineReject
     - TestDecisionEngineCustomThresholds
     - TestDecisionEngineNegativeScore
     - TestDefaultThresholds
     - TestVerdictReason
     - TestDecisionEngineBoundaryValues
  - [x] Milter 통합 테스트: 스팸 필터 결과 → SMTP 거부/수락 (5개 테스트)
     - TestSpamHookAccept
     - TestSpamHookReject
     - TestSpamHookShadowMode
     - TestSpamHookDisabled
     - TestSpamVerdictHeaders + TestMilterSpamVerdict
  - [x] RFC 5764 (Milter) 호환: 프로토콜 액션 매핑
  - [x] X-Spam-Score/X-Spam-Status 헤더 구현
  - [x] Shadow mode 지원 (테스트 상태 로깅, 메시지 통과 허용)

- **다음 태스크**: TASK-063 이상 (backend-roadmap.md 참고)

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기 ✓
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```
