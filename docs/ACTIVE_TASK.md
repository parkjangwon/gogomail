# ACTIVE_TASK

## TASK-088: Mail Infrastructure Hardening (Performance & RFC Compliance)

### 배경

개발 목표: "파워풀한 백엔드 성능, 초고속 수발신 처리, 버그 없는 단단한 코드, RFC 완전 준수, 몬스터 같은 SMTP 경험"

Admin Console Phase 1-3이 완료됨. Phase 1 mail infrastructure (receive, send, delivery) 기본 구현도 완료.
이제 대량 메일 발신 환경에서의 성능, 안정성, RFC 준수 등을 강화해야 함.

### 진행 상황

**Phase 1: Connection Pooling (✓ 완료)**
- SMTP ConnectionPool 구현: 재사용 가능한 연결 관리
- SMTPConnPoolKey로 호스트별 연결 구분
- MaxIdle, IdleTimeout, MaxAge 설정 가능
- DirectSMTPTransport에 통합 완료
- Thread-safe 초기화 (sync.Once)
- 모든 테스트 통과 (6056 tests)

### 구현 대상

Go Backend (`internal/`):
- SMTP 파이프라인 최적화: 배치 전송, 커넥션 풀 재활용, 홀드백 조정
- RFC compliance 검증: SMTPUTF8, DSN, DKIM, SPF/DMARC, trace headers
- Delivery worker 성능: 병렬 처리, 재시도 정책 튜닝, 핸들링 에러
- Message parsing 성능: 메모리 할당 최소화, 스트리밍 우선
- Database 쿼리 최적화: 인덱싱, 쿼리 재작성, 배치 처리
- Observability: 메트릭, 로깅, 추적(tracing) 

### 완료 조건

**Phase 1: Connection Pooling (✓)**
- [x] SMTP 커넥션 풀 구현 (connection pooling)
- [x] 풀 초기화 동시성 보호 (sync.Once)

**Phase 2: RFC Compliance & Pipelining (진행 중)**
- [ ] SMTP 파이프라인 성능 벤치마크 및 최적화
- [ ] RFC compliance 검증 (Received headers, DKIM, SPF/DMARC)
- [ ] Delivery worker 재시도 정책 튜닝

**Phase 3: Performance Optimization**
- [ ] Message parser 성능 개선 및 메모리 할당 감소
- [ ] Database 쿼리 최적화 및 인덱싱
- [ ] 대량 발신 부하 테스트 (1000+ msg/sec)
- [ ] 성능 메트릭 수집 및 대시보드

**Documentation**
- [ ] docs/CURRENT_STATUS.md 갱신
- [ ] docs/backend-roadmap.md 해당 항목 체크

### 검증

- `go test ./...` 통과
- `go build ./...` 성공
- 벤치마크 결과 기록 (msg/sec, latency, memory)

### 다음 태스크

TASK-089: Protocol Gateway Hardening (IMAP, CalDAV, CardDAV)
