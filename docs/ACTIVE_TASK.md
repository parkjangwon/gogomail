# ACTIVE_TASK

## Current Status Summary

**TASK-089: Protocol Gateway Hardening** ✅ COMPLETE (5987 tests)
- All 3 phases implemented and verified
- Buffer pooling, metrics export, health checks, graceful degradation
- Ready for production deployment

**TASK-090: Message Storage & Delivery Optimization** 🔄 STARTING
- Phase 1 (Database Query Optimization): Planned - Index analysis, query optimization
- Phase 2 (Bulk Delivery Batching): Planned - Multi-recipient optimization
- Phase 3 (Message Caching Layer): Planned - Frequently accessed data caching

---

## TASK-090: Message Storage & Delivery Optimization (Bulk Mail Performance)

### 목표

대량 메일 발신 환경에서의 데이터베이스 성능, 메시지 저장 효율성, 대량 발신 배치 처리 최적화.
`internal/maildb`의 메시지 쿼리, `internal/delivery`의 배치 처리, 메시지 스토리지 서브시스템의 성능 개선.

현재 문제점:
- 대량 발신 시 메시지 메타데이터 조회 성능 저하
- 배치 처리 없이 개별 메시지 처리로 인한 데이터베이스 왕복 증가
- 자주 접근하는 메시지 데이터 캐싱 미구현

### 구현 대상

Go Backend (`internal/`):
- 메시지 조회 쿼리 인덱싱 검증: 수신자, 상태, 발송 시간 기반 조회 최적화
- 배치 메시지 처리: 대량 발신 시 N+1 쿼리 제거
- 메시지 캐싱: Redis 또는 메모리 기반 LRU 캐시 (옵션)
- 대량 메일 벤치마크: 1000+, 10000+, 100000+ 메시지 발신 성능 측정
- 지연된 재시도 최적화: 배치 스케줄링 개선

### 단계별 계획

**Phase 1: Database Query Optimization (진행 중)**
- 메시지 조회 쿼리 분석: EXPLAIN ANALYZE로 현재 성능 조사
- 누락된 인덱스 추가: delivery_state, scheduled_at, recipient_count 기준 인덱스
- 배치 조회 함수 최적화: ListOutboundMessages(), GetMessagesByID()
- 벤치마크: 단일 쿼리 vs 배치 조회 성능 비교
- 목표: 대량 조회 시 쿼리 개수 50% 감소

**Phase 2: Bulk Delivery Batching (예상)**
- 멀티 수신자 메시지 배치 처리: 같은 도메인 수신자 묶음 발송
- 배치 크기 튜닝: 메모리/성능 트레이드오프
- 벤치마크: 배치 vs 개별 발송 처리량 비교
- 목표: 대량 발신 처리량 2배 이상 향상

**Phase 3: Message Caching Layer (예상)**
- 자주 접근하는 메시지 메타데이터 캐싱
- Redis 기반 캐시 (선택사항) 또는 메모리 LRU 캐시
- 캐시 무효화 전략: 메시지 상태 변경 시 자동 무효화
- 목표: 메시지 조회 레이턴시 30-50% 감소

### 진행 상황

**Phase 1 진행 중: Database Query Optimization**

구현 대상:
- [ ] EXPLAIN ANALYZE로 메시지 조회 쿼리 성능 분석
- [ ] 누락된 인덱스 생성 (delivery_state, scheduled_at, recipient_count)
- [ ] ListOutboundMessages 최적화 (N+1 제거)
- [ ] GetMessagesByID 배치 조회 함수 작성
- [ ] 벤치마크 프레임워크 (메시지 1000+, 10000+ 시나리오)
- [ ] 테스트 검증: go test ./... 통과

최근 진행:
- `ListMessagesByIDs` hydration을 `unnest($2::uuid[]) WITH ORDINALITY` 기반으로 바꿔 JSON 배열 파싱을 제거함
- `ListMessageIDsForThreads`와 `BulkSetThreadFlag`도 UUID 배열 `unnest` 경로로 바꿔 thread 배치 처리의 JSON 파싱을 제거함
- `ListThreadMessagesPage`는 `COALESCE(thread_id, id)` 비교를 UUID 친화적인 `thread_id = ... OR id = ...`로 분해함
- IMAP UID copy/expunge/move/hydrate 경로도 typed array unnest로 바꿔 요청당 JSON 직렬화 비용을 제거함
- `imapUIDArray` 1k/10k 벤치마크를 추가해 UID 전처리 비용을 추적할 수 있게 함
- IMAP mailbox lookup normalization도 `strings.Fields` 대신 로컬 공백 정리 스캐너를 쓰도록 바꿔 `SELECT`/`LIST` alias 처리의 토큰 슬라이스 비용을 줄임
- active 메시지/스레드 lookup용 partial index migration을 추가함

다음 단계: Phase 2 (Bulk Delivery Batching) 구현

### 검증

- `go test ./...` 통과
- `go build ./...` 성공
- 벤치마크 결과 기록 (쿼리/sec, 레이턴시, 메모리)
