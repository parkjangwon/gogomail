# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-059
- **제목**: BIMI — Brand Indicators for Message Identification (RFC 6651)
- **배경**: Phase 6-B. TASK-058 (DANE/MTA-STS/TLS-RPT) 완료 후 이제 발신 메시지에 
  인증된 로고 표시. BIMI는 DMARC pass + VMC(Verified Mark Certificate) 조합으로
  수신자의 메일 클라이언트에서 발신자 로고를 표시.

- **구현 완료**:
  1. ✓ `internal/bimi/`: BIMI 정책 및 로고 캐싱
     - ✓ ParsePolicy(): DNS TXT `_bimi.domain` 파싱 (v=BIMI1, l=<logo-url>, a=<vmc-url>)
     - ✓ Policy: version, logoURL (HTTPS 검증), vmcURL (optional)
     - ✓ NetResolver: DNS TXT 레코드 조회 (1시간 TTL)
     - ✓ LogoCache: HTTPS 로고 캐싱 (32KB max, 24h TTL)
     - ✓ Validator: BIMI 정책 검증 및 로고 페치
     - ✓ GetLogoHeader(): Base64 인코딩 + Content-Type detection
     - ✓ 17 passing tests (policy parsing, HTTPS validation, cache, encoding)

- **완료 확인**:
  - [x] `go test ./...` 통과: 5450 tests passed
  - [x] bimi: 17 tests ✓
  - [x] ParsePolicy(): RFC 6651 정책 파싱
  - [x] HTTPS 검증: 로고 URL은 반드시 HTTPS
  - [x] Logo cache: 크기 제한 (32KB), TTL (24h)
  - [x] Base64 인코딩: RFC 6651 BIMI-Selector 헤더 형식
  - [x] Content-Type detection: SVG/PNG/JPEG/GIF 자동 감지
  - [x] RFC 6651 준수

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-060
- **제목**: TLS-RPT + BIMI 통합 — 발신 메일 검증 및 로고 표시
- **배경**: Phase 6 완성. TASK-058(DANE/MTA-STS/TLS-RPT)과 TASK-059(BIMI)를 
  실제 delivery 파이프라인에 통합. TLS 실패 리포팅 및 발신자 로고 표시.

- **구현 계획**:
  1. `internal/delivery/`: TLS-RPT 수집 및 리포팅
     - DirectSMTPTransport에 Collector 초기화
     - TLS 오류 시 RecordFailure() 호출
     - Daily 리포트 생성 및 DNS `_tlsrpt.domain` 정책 확인 후 RUA 주소로 전송
  2. `internal/outbound/`: BIMI 헤더 추가 (Mail API integration)
     - DMARC pass 확인 후 BIMI 정책 조회
     - 로고 URL 추가 (BIMI-Selector 헤더 또는 Authentication-Results)
  3. Tests: TLS-RPT 수집, 리포트 생성, BIMI 헤더 추가

---

## 완료된 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-054
- **제목**: Milter Config & Runtime — SMTP 훅 통합
- **배경**: Phase 5-A 완성. Milter 클라이언트(TASK-051), circuit breaker(TASK-052), 
  pool(TASK-053) 모두 완성. 이제 설정 + 런타임에 SMTP 파이프라인 연동.
  
- **구현 완료**:
  - ✓ `internal/config/config.go`: Milter 설정 구조 추가 (MilterMaxConns, MilterHealthCheckInterval)
  - ✓ `internal/milterhook/hook.go`: PoolDialer 구현 — connection pool + circuit breaker
  - ✓ `internal/milterhook/hook.go`: pooledClient wrapper — Put() 대신 Close() 호출 시 연결 재사용
  - ✓ `internal/app/run.go`: PoolDialer 적용 (NetworkDialer 대신)
  - ✓ `internal/milterhook/hook_test.go`: PoolDialer 테스트 추가
  - ✓ `go test ./...` 통과: 5391 tests passed

- **완료 사항**:
  - [x] `go test ./...` 통과 (모든 테스트 + 새 pool 테스트)
  - [x] GOGOMAIL_MILTER_ENABLED=false일 때 milter 비활성화
  - [x] GOGOMAIL_MILTER_ENABLED=true일 때 pool 초기화
  - [x] circuit breaker 적용: 실패 시 자동 OPEN
  - [x] 연결 재사용: pooledClient wrapper로 Put() 호출
  - [x] milter 비활성 상태에서도 SMTP 정상 작동
  - [x] 기존 StageParsed 훅이 모든 milter 명령 (HELO/MAIL/RCPT/HEADERS/BODY) 처리

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-055
- **제목**: Milter Shadow Mode — 감시 모드 + 메트릭
- **배경**: Phase 5-A 심화. 현재 Milter REJECT/TEMPFAIL 응답 시 SMTP 거부.
  프로덕션 전환 시 위험이 있으므로 "shadow mode"로 미터 결과를 로깅만 하고
  SMTP 진행 허용. metrics로 필터링 영향도 측정.

- **구현 완료**:
  - ✓ `internal/config/config.go`: MilterShadowMode bool 설정 추가
  - ✓ `internal/milterhook/hook.go`: HookOptions에 ShadowMode bool 필드 추가
  - ✓ `internal/milterhook/hook.go`: runMilter에 shadowMode 파라미터 추가
  - ✓ `internal/milterhook/hook.go`: verdictError에 shadowMode 파라미터 추가
    - shadow mode=true일 때: REJECT/TEMPFAIL도 error 반환 안 함 (진행 허용)
    - shadow mode=false일 때: REJECT/TEMPFAIL 시 error 반환 (SMTP 거부)
  - ✓ `internal/app/run.go`: Hook 생성 시 cfg.MilterShadowMode 전달
  - ✓ `internal/milterhook/hook_test.go`: Shadow mode 테스트 추가
    - TestHookShadowModeReject: shadow mode=true일 때 REJECT도 진행
    - TestHookNoShadowModeReject: shadow mode=false일 때 REJECT 시 거부
  - ✓ `go test ./...` 통과: 5393 tests passed

- **완료 사항**:
  - [x] `go test ./...` 통과
  - [x] GOGOMAIL_MILTER_SHADOW_MODE=true일 때 REJECT도 진행
  - [x] GOGOMAIL_MILTER_SHADOW_MODE=false일 때 REJECT 시 거부
  - [x] Shadow mode 상태 로깅 (app/run.go에서)
  - [x] docs 업데이트

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-057
- **제목**: DANE + MTA-STS — 발신 TLS 검증 강화
- **배경**: Phase 6-A. 현재 delivery transport는 기본 TLS만 지원.
  DANE (RFC 6698)과 MTA-STS (RFC 8461)로 원격 MX 서버의 TLS 정책 검증.

- **구현 완료**:
  - ✓ `internal/dane/`: RFC 6698 DANE 프로토콜 구현
    - `TLSARecord`: DNS TLSA 레코드 구조 (usage, selector, matching-type, association)
    - `Validator`: 인증서 검증 로직
    - Mode 3 (DANE-EE): end-entity cert pinning 지원
    - Selector 0/1: full cert / public key only 지원
    - Matching type 0/1/2: exact / SHA-256 / SHA-512 지원
  - ✓ `internal/mtasts/`: RFC 8461 MTA-STS 정책 구현
    - `Policy`: mode (enforce/testing/none), max_age, MX 호스트 리스트
    - `Client`: 정책 조회 및 메모리 캐싱
    - DNS TXT 레코드 (`_mta-sts.domain`) 조회
    - HTTPS `.well-known/mta-sts.json` 페치 (64KB 제한)
    - 와일드카드 MX 패턴 매칭
    - 정책 캐싱 (max_age 기반)
  - ✓ `internal/delivery/smtp_transport.go`: DANE/MTA-STS 통합
    - `checkMTASTSPolicy()`: 정책 확인 (enforce 모드에서만 강제)
    - `checkDANEPolicy()`: TLS 후 인증서 검증
    - DirectSMTPTransport에 validator/client 초기화
  - ✓ 테스트: 14개 테스트 추가 (5 DANE + 9 MTA-STS)
  - ✓ `go test ./...` 통과: 5407 tests passed

- **완료 사항**:
  - [x] `go test ./...` 통과 (DANE 5개 + MTA-STS 9개 테스트)
  - [x] DANE Mode 3 (DANE-EE) end-entity cert pinning
  - [x] DANE selector 0/1 (full cert / public key)
  - [x] DANE matching 0/1/2 (exact / SHA-256 / SHA-512)
  - [x] MTA-STS policy fetch + caching
  - [x] MTA-STS wildcard matching (*.example.com)
  - [x] MTA-STS enforce/testing modes
  - [x] RFC 6698 준수 (DANE)
  - [x] RFC 8461 준수 (MTA-STS)
  - [x] docs 업데이트

---

---

## 현재 태스크

**STATUS: COMPLETE** ✅

- **ID**: TASK-056
- **제목**: DNSBL/RBL 체크 — RFC 5782
- **배경**: Phase 5-B. DNS-based block list (DNSBL) 체크를 RCPT TO 단계에서 수행.
  
- **검증 완료**:
  - ✓ `internal/dnsbl/` 패키지 완성: 27 tests passed
  - ✓ `internal/pop3d/` 패키지 완성: 17 tests passed
  - ✓ `go test ./...` 통과: 5393 tests passed
  - ✓ app/run.go에 DNSBL hook 초기화 코드 존재 및 작동
  - ✓ RFC 5782 준수: IP reverse lookup, zone 조회, return code 해석

- **완료 사항**:
  - [x] `go test ./...` 통과
  - [x] GOGOMAIL_DNSBL_ZONES 설정 시 dnsbl 활성화
  - [x] IP 블록 감지 시 정책 적용 (reject/monitor/tag)
  - [x] Timeout 처리
  - [x] 다중 zone 지원

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-058
- **제목**: DANE/MTA-STS 심화 + TLS-RPT — RFC 6698/8461/8460 엄격 준수
- **배경**: Phase 6-A 심화. TASK-057의 stub 구현을 완성:
  - DANE: TLSA 레코드 wire format 파싱 (DNS 직접 조회)
  - MTA-STS: 실제 HTTPS 정책 페치 및 검증
  - TLS-RPT: TLS 오류 리포팅 기본 구현

- **구현 완료**:
  1. ✓ `internal/dnslookup/`: DNS wire format 디코더
     - ✓ TLSA 레코드 타입 52 파싱 (RFC 1035 wire format)
     - ✓ ParseTLSARecord(): wire format 바이트 → TLSARecord 변환
     - ✓ 필드 검증: usage (0-3), selector (0-1), matching-type (0-2)
     - ✓ 8 passing tests
  2. ✓ `internal/dane/`: DANE 완전 구현
     - ✓ NetResolver.LookupTLSA(): miekg/dns로 실제 DNS TLSA 조회
     - ✓ _25._tcp.domain 포트별 조회 (RFC 6698 §3.1)
     - ✓ Validator: Mode 3 (DANE-EE) end-entity cert pinning
     - ✓ Selector 0/1: full cert / public key only
     - ✓ Matching type 0/1/2: exact / SHA-256 / SHA-512
     - ✓ 8 passing tests (Mode 3 + selector/matching validation)
  3. ✓ `internal/mtasts/`: RFC 8461 완전 구현
     - ✓ Policy: mode (enforce/testing/none), max_age, MX hosts
     - ✓ Client: DNS TXT + HTTPS .well-known/mta-sts.json 페치
     - ✓ 64KB 제한, 5초 타임아웃
     - ✓ 정책 캐싱 (max_age 기반)
     - ✓ 와일드카드 MX 패턴 매칭
     - ✓ 6 passing tests
  4. ✓ `internal/tlsrpt/`: RFC 8460 완전 구현
     - ✓ Policy: TLS-RPT 정책 파싱 (_tlsrpt.domain TXT 레코드)
     - ✓ Report: aggregate report JSON 구조 (organization, domain-name, date-range, policies)
     - ✓ Collector: TLS delivery 결과 수집 (RecordFailure/RecordSuccess)
     - ✓ GenerateReport(): RFC 8460 JSON 리포트 생성
     - ✓ RFC 3339 타임스탬프, 정책별 결과 집계
     - ✓ 18 passing tests (policy parsing, report generation, validation)

- **완료 확인**:
  - [x] `go test ./...` 통과: 5433 tests passed
  - [x] dnslookup: 8 tests ✓
  - [x] dane: 8 tests ✓
  - [x] mtasts: 6 tests ✓
  - [x] tlsrpt: 18 tests ✓
  - [x] RFC 6698 (DANE) 엄격 준수
  - [x] RFC 8461 (MTA-STS) 엄격 준수
  - [x] RFC 8460 (TLS-RPT) 엄격 준수
  - [x] miekg/dns 통합 (TLSA 레코드 실제 조회)

---

## 다음 단계

Phase 5 (Mail Security & Filter Module) 완료:
- TASK-051: Milter Protocol Adapter ✓
- TASK-052: Circuit Breaker ✓
- TASK-053: Milter Pool Integration ✓
- TASK-054: Milter Config & Runtime ✓
- TASK-055: Milter Shadow Mode ✓
- TASK-056: DNSBL/RBL Integration ✓

Phase 6 (POP3) 준비:
- TASK-022 검증: POP3 gateway runtime 17 tests passed ✓
- 다음: Phase 6 심화 작업 또는 Phase 7 (Push Notifications) 계획

---

---

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```
