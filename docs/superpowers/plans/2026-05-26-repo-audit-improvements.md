# Repository Audit Improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 리포지토리 전수 조사에서 발견된 12개 개선 항목을 모두 구현한다.

**Architecture:** 항목을 위험도·영향도 순으로 배치한다. 독립 버그픽스(Tasks 1–4)를 먼저 처리해 안전한 기반을 마련한 후, CI 품질 게이트(Task 5)를 추가하고, 컨텍스트 전파(Task 6)와 에러 래핑(Task 7)을 정리한 뒤, 대형 파일 분리(Tasks 8–11)를 순차적으로 진행한다.

**Tech Stack:** Go 1.25, `net/http`, `log/slog`, `golangci-lint`, `govulncheck`, GitHub Actions

---

## File Map

| 파일 | 역할 |
|------|------|
| `internal/sso/verify.go` | OIDC discovery / JWKS HTTP 클라이언트 타임아웃 추가 |
| `internal/pushnotify/pushnotify.go` | Web push HTTP 클라이언트 타임아웃 추가 |
| `internal/config/config.go` | 4개 env var를 Config 구조체로 이동 |
| `internal/apikeys/middleware.go` | os.Getenv 제거 → config 주입 |
| `internal/mailservice/systememail.go` | os.Getenv 제거 → config 주입 |
| `internal/httpapi/admin_auth.go` | os.Getenv 제거 → config 주입 |
| `internal/jmap/types.go` | mustRawString panic → safe encoding |
| `.github/workflows/ci.yml` | golangci-lint, govulncheck, integration→docker 의존성 추가 |
| `.golangci.yml` | 신규: linter 설정 |
| `internal/app/run.go` | 서브시스템별 파일로 분리 (run_imap.go 등) |
| `internal/app/run_imap.go` | 신규: IMAP 게이트웨이 기동 코드 |
| `internal/app/run_pop3.go` | 신규: POP3 게이트웨이 기동 코드 |
| `internal/app/run_caldav.go` | 신규: CalDAV/CardDAV/WebDAV 게이트웨이 |
| `internal/app/run_ldap.go` | 신규: LDAP 게이트웨이 |
| `internal/app/run_smtp.go` | 신규: SMTP inbound/outbound |
| `internal/app/run_http.go` | 신규: HTTP API 서버 기동 |
| `internal/app/run_batch.go` | 신규: 배치 워커 |
| `internal/httpapi/mail.go` | 핵심 라우터 + helpers만 유지 |
| `internal/httpapi/mail_folders.go` | 신규: 폴더 핸들러 |
| `internal/httpapi/mail_messages.go` | 신규: 메시지 핸들러 |
| `internal/httpapi/mail_drafts.go` | 신규: 드래프트/첨부파일 핸들러 |
| `internal/httpapi/mail_threads.go` | 신규: 스레드/검색 핸들러 |
| `internal/mailservice/service.go` | Service 구조체 + 생성자만 유지 |
| `internal/mailservice/service_folders.go` | 신규: 폴더 메서드 |
| `internal/mailservice/service_messages.go` | 신규: 메시지 조회/변경 메서드 |
| `internal/mailservice/service_drafts.go` | 신규: 드래프트/전송 메서드 |
| `internal/mailservice/service_attachments.go` | 신규: 첨부파일 메서드 |
| `internal/mailservice/service_search.go` | 신규: 검색 메서드 |

---

### Task 1: SSO + pushnotify HTTP 클라이언트 타임아웃

**Goal:** `http.DefaultClient` 사용처에 타임아웃을 가진 전용 HTTP 클라이언트를 주입해 goroutine 누수를 방지한다.

**Files:**
- Modify: `internal/sso/verify.go`
- Modify: `internal/pushnotify/pushnotify.go`

**Acceptance Criteria:**
- [ ] `sso/verify.go`의 OIDC discovery, JWKS fetch가 `http.DefaultClient` 대신 15초 타임아웃 클라이언트를 사용한다
- [ ] `pushnotify/pushnotify.go`의 모든 HTTP 호출이 타임아웃 클라이언트를 사용한다
- [ ] `go test ./internal/sso/... ./internal/pushnotify/...` 통과

**Verify:** `go test ./internal/sso/... ./internal/pushnotify/... -v` → PASS

**Steps:**

- [ ] **Step 1: sso/verify.go — 패키지 수준 HTTP 클라이언트 추가**

`internal/sso/verify.go` 파일 상단(import 블록 아래)에 추가:

```go
// oidcHTTPClient is used for all OIDC discovery and JWKS fetches.
// 15-second timeout matches the per-request context timeout already set by callers.
var oidcHTTPClient = &http.Client{
    Timeout: 15 * time.Second,
}
```

- [ ] **Step 2: sso/verify.go — DefaultClient 교체**

```go
// 변경 전
resp, err := http.DefaultClient.Do(req)
// ...
resp, err := http.DefaultClient.Do(jwksReq)

// 변경 후
resp, err := oidcHTTPClient.Do(req)
// ...
resp, err := oidcHTTPClient.Do(jwksReq)
```

- [ ] **Step 3: pushnotify/pushnotify.go — 패키지 수준 HTTP 클라이언트 추가**

```go
// defaultPushHTTPClient is shared across all push notification senders.
// 30s is generous for web-push endpoints but still bounded.
var defaultPushHTTPClient = &http.Client{
    Timeout: 30 * time.Second,
}
```

- [ ] **Step 4: pushnotify/pushnotify.go — DefaultClient 교체**

파일 내 `http.DefaultClient` 7곳 전부를 `defaultPushHTTPClient`로 변경.

검색: `grep -n "http.DefaultClient" internal/pushnotify/pushnotify.go`

각 등장 위치:
```go
// 패턴: client = http.DefaultClient → client = defaultPushHTTPClient
// 패턴: client = *http.DefaultClient → client = *defaultPushHTTPClient (포인터 역참조 형태인 경우)
```

- [ ] **Step 5: 테스트 실행**

```bash
go test ./internal/sso/... ./internal/pushnotify/... -v 2>&1 | tail -20
```

- [ ] **Step 6: 커밋**

```bash
git add internal/sso/verify.go internal/pushnotify/pushnotify.go
git commit -m "fix(http): replace http.DefaultClient with timeout-bounded clients in sso and pushnotify"
```

---

### Task 2: os.Getenv — config 패키지 밖 직접 사용 제거

**Goal:** 4개 env var를 `config.Config` 구조체로 이동해 설정 계층을 단일화하고 테스트 용이성을 확보한다.

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/validate.go` (필요 시)
- Modify: `internal/apikeys/middleware.go`
- Modify: `internal/mailservice/systememail.go`
- Modify: `internal/httpapi/admin_auth.go`
- Modify: `internal/app/run.go` (config 전달 경로 확인)

**Acceptance Criteria:**
- [ ] `GOGOMAIL_TRUSTED_PROXY_CIDRS`가 `Config.TrustedProxyCIDRs`로 노출된다
- [ ] `GOGOMAIL_SYSTEM_EMAIL_FROM`, `GOGOMAIL_SYSTEM_SMTP_ADDR`, `GOGOMAIL_SYSTEM_SMTP_USER`, `GOGOMAIL_SYSTEM_SMTP_PASS`가 `Config.SystemEmail.*`로 노출된다
- [ ] `GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL`, `GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD`가 `Config.AdminBootstrap.*`로 노출된다
- [ ] 세 파일의 `os.Getenv` 호출이 전부 제거된다
- [ ] `configs/config.example.yaml`에 새 필드 주석이 추가된다
- [ ] `go test ./...` 통과

**Verify:** `grep -rn "os.Getenv" internal/ --include="*.go" | grep -v "_test.go" | grep -v "config/"` → 빈 결과

**Steps:**

- [ ] **Step 1: config.go에 새 필드 추가**

`internal/config/config.go`의 `Config` 구조체에 추가:

```go
// TrustedProxyCIDRs is a comma-separated list of CIDR ranges whose
// X-Forwarded-For header is trusted for real-IP extraction.
TrustedProxyCIDRs string

// SystemEmail holds SMTP credentials used for system-generated email
// (password reset, quota warnings, etc.)
SystemEmail struct {
    From     string
    SMTPAddr string
    SMTPUser string
    SMTPPass string
}

// AdminBootstrap holds the one-time admin account seed credentials.
// Consumed once on first startup; leave empty after bootstrapping.
AdminBootstrap struct {
    Email    string
    Password string
}
```

- [ ] **Step 2: config.go의 환경변수 파싱 블록에 추가**

기존 `intEnvOrDefault`, `stringEnvOrDefault` 패턴을 따라:

```go
TrustedProxyCIDRs: os.Getenv("GOGOMAIL_TRUSTED_PROXY_CIDRS"),
SystemEmail: struct {
    From     string
    SMTPAddr string
    SMTPUser string
    SMTPPass string
}{
    From:     strings.TrimSpace(os.Getenv("GOGOMAIL_SYSTEM_EMAIL_FROM")),
    SMTPAddr: strings.TrimSpace(os.Getenv("GOGOMAIL_SYSTEM_SMTP_ADDR")),
    SMTPUser: os.Getenv("GOGOMAIL_SYSTEM_SMTP_USER"),
    SMTPPass: os.Getenv("GOGOMAIL_SYSTEM_SMTP_PASS"),
},
AdminBootstrap: struct {
    Email    string
    Password string
}{
    Email:    strings.TrimSpace(os.Getenv("GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL")),
    Password: os.Getenv("GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD"),
},
```

- [ ] **Step 3: apikeys/middleware.go — os.Getenv 제거**

미들웨어가 config를 받도록 시그니처 확인 후:

```go
// 변경 전
for _, raw := range strings.Split(os.Getenv("GOGOMAIL_TRUSTED_PROXY_CIDRS"), ",") {

// 변경 후 (config를 생성자나 파라미터로 받는 구조에 따라)
for _, raw := range strings.Split(cfg.TrustedProxyCIDRs, ",") {
```

미들웨어 생성 함수 시그니처에 `cfg config.Config` 추가 또는 `trustedCIDRs string` 파라미터 추가.

- [ ] **Step 4: mailservice/systememail.go — os.Getenv 제거**

```go
// 변경 전
from := strings.TrimSpace(os.Getenv("GOGOMAIL_SYSTEM_EMAIL_FROM"))
addr := strings.TrimSpace(os.Getenv("GOGOMAIL_SYSTEM_SMTP_ADDR"))
user := os.Getenv("GOGOMAIL_SYSTEM_SMTP_USER")
pass := os.Getenv("GOGOMAIL_SYSTEM_SMTP_PASS")

// 변경 후: cfg config.Config를 파라미터로 주입
from := cfg.SystemEmail.From
addr := cfg.SystemEmail.SMTPAddr
user := cfg.SystemEmail.SMTPUser
pass := cfg.SystemEmail.SMTPPass
```

- [ ] **Step 5: httpapi/admin_auth.go — os.Getenv 제거**

```go
// 변경 전
bootstrapEmail := strings.TrimSpace(os.Getenv("GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL"))
bootstrapPassword := os.Getenv("GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD")

// 변경 후
bootstrapEmail := cfg.AdminBootstrap.Email
bootstrapPassword := cfg.AdminBootstrap.Password
```

- [ ] **Step 6: configs/config.example.yaml에 주석 추가**

```yaml
# Trusted proxy CIDR ranges for X-Forwarded-For (comma-separated)
# GOGOMAIL_TRUSTED_PROXY_CIDRS: "10.0.0.0/8,172.16.0.0/12"

# System email sender (password reset, quota alerts)
# GOGOMAIL_SYSTEM_EMAIL_FROM: "noreply@example.com"
# GOGOMAIL_SYSTEM_SMTP_ADDR: "localhost:25"
# GOGOMAIL_SYSTEM_SMTP_USER: ""
# GOGOMAIL_SYSTEM_SMTP_PASS: ""

# Admin bootstrap (seed admin account on first startup, clear after use)
# GOGOMAIL_ADMIN_BOOTSTRAP_EMAIL: ""
# GOGOMAIL_ADMIN_BOOTSTRAP_PASSWORD: ""
```

- [ ] **Step 7: 테스트 및 검증**

```bash
go build ./...
go test ./internal/config/... ./internal/apikeys/... ./internal/mailservice/... ./internal/httpapi/... -v 2>&1 | tail -20
grep -rn "os.Getenv" internal/ --include="*.go" | grep -v "_test.go" | grep -v "config/"
```

- [ ] **Step 8: 커밋**

```bash
git add internal/config/config.go internal/apikeys/middleware.go \
        internal/mailservice/systememail.go internal/httpapi/admin_auth.go \
        configs/config.example.yaml
git commit -m "refactor(config): move os.Getenv calls into config.Config (apikeys, systememail, admin_auth)"
```

---

### Task 3: JMAP types.go — panic 제거

**Goal:** `mustRawString`의 `panic` 경로를 안전한 fallback으로 교체해 HTTP handler goroutine 사망을 방지한다.

**Files:**
- Modify: `internal/jmap/types.go`

**Acceptance Criteria:**
- [ ] `mustRawString`이 더 이상 `panic`을 호출하지 않는다
- [ ] `json.Marshal`이 실패할 수 없는 string 타입에서는 `strconv.AppendQuote` fallback으로 대체된다
- [ ] `go test ./internal/jmap/... -v` 통과

**Verify:** `grep -n "panic" internal/jmap/types.go` → 빈 결과

**Steps:**

- [ ] **Step 1: mustRawString 교체**

```go
// 변경 전
func mustRawString(s string) json.RawMessage {
    b, err := json.Marshal(s)
    if err != nil {
        panic(err)
    }
    return b
}

// 변경 후
// rawString JSON-encodes s as a JSON string.
// json.Marshal cannot fail for a Go string, but we avoid panic defensively.
func rawString(s string) json.RawMessage {
    b, err := json.Marshal(s)
    if err != nil {
        // Fallback: strconv.AppendQuote produces valid JSON string encoding.
        return strconv.AppendQuote(nil, s)
    }
    return b
}
```

- [ ] **Step 2: 호출부 이름 변경**

`mustRawString` → `rawString` 으로 파일 내 전체 교체:

```bash
grep -n "mustRawString" internal/jmap/types.go
```

해당 줄(51, 53, 95, 97번 줄 근방) 4곳을 `rawString`으로 변경.

- [ ] **Step 3: import에 strconv 추가**

`internal/jmap/types.go`의 import 블록에:
```go
"strconv"
```
추가.

- [ ] **Step 4: 테스트**

```bash
go test ./internal/jmap/... -v 2>&1 | tail -20
```

- [ ] **Step 5: 커밋**

```bash
git add internal/jmap/types.go
git commit -m "fix(jmap): replace panic in rawString with safe strconv fallback"
```

---

### Task 4: CI 품질 게이트 강화

**Goal:** `golangci-lint`, `govulncheck`를 CI에 추가하고, integration test가 완료된 뒤에 docker-image job이 실행되도록 의존성을 수정한다.

**Files:**
- Create: `.golangci.yml`
- Modify: `.github/workflows/ci.yml`

**Acceptance Criteria:**
- [ ] `.golangci.yml`이 존재하며 errcheck, staticcheck, gosec, exhaustive 등 핵심 linter가 활성화된다
- [ ] CI `go-lint` job이 PR에서 실행된다
- [ ] CI `go-vuln` job이 `govulncheck ./...`를 실행한다
- [ ] `docker-image` job의 `needs`에 `go-integration-test`가 포함된다
- [ ] 로컬에서 `golangci-lint run ./...`이 통과한다 (신규 위반 없음)

**Verify:** `golangci-lint run ./... 2>&1 | grep -c "^"` → 낮은 숫자 또는 0

**Steps:**

- [ ] **Step 1: .golangci.yml 생성**

```yaml
# .golangci.yml
version: "2"

linters:
  enable:
    - errcheck        # 무시된 에러 반환값
    - staticcheck     # SA*, S1* 정적 분석
    - gosec           # 보안 패턴 (G101-G602)
    - exhaustive      # switch 누락 케이스
    - govet           # go vet (기본 포함이지만 명시)
    - unused          # 미사용 코드
    - goimports       # import 정렬
    - misspell        # 오탈자
    - noctx           # http.Request without context
    - bodyclose       # resp.Body.Close() 누락

linters-settings:
  errcheck:
    # 로그 write, flush 등 관례적으로 무시되는 것은 예외
    exclude-functions:
      - (io.Closer).Close
      - (*os.File).Close
      - (net.Conn).Close
      - (*net/http.Response.Body).Close
  gosec:
    excludes:
      - G104  # errcheck가 이미 커버
      - G304  # filepath.Join 관련 (의도적 사용)
  exhaustive:
    default-signifies-exhaustive: true

issues:
  # 기존 코드베이스 대규모 위반은 단계적 해결 — 신규 코드에만 강제
  new: false
  # 단, 보안/안전 관련 linter는 기존 코드도 검사
  exclude-rules:
    - linters: [errcheck, staticcheck, unused, exhaustive]
      path: "_test\\.go"
```

- [ ] **Step 2: .github/workflows/ci.yml — golangci-lint job 추가**

`go-build` job 다음에 삽입:

```yaml
  go-lint:
    name: Go lint (golangci-lint)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: v1.64.8
          args: --timeout=5m
```

- [ ] **Step 3: .github/workflows/ci.yml — govulncheck job 추가**

```yaml
  go-vuln:
    name: Go vulnerability scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - name: govulncheck
        run: govulncheck ./...
```

- [ ] **Step 4: docker-image job needs 수정**

```yaml
# 변경 전
  docker-image:
    needs: [go-test, go-build]

# 변경 후
  docker-image:
    needs: [go-test, go-build, go-integration-test]
```

- [ ] **Step 5: 로컬 검증**

```bash
# golangci-lint 설치 (없으면)
which golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
golangci-lint run ./... --timeout=5m 2>&1 | head -40
```

- [ ] **Step 6: 커밋**

```bash
git add .golangci.yml .github/workflows/ci.yml
git commit -m "ci: add golangci-lint, govulncheck, fix docker-image dependency on integration tests"
```

---

### Task 5: context.Background() → caller context 전파

**Goal:** 서비스 코드에서 `context.Background()`로 caller 취소를 무시하는 35곳을 분류해, 실제로 caller context를 전파해야 하는 곳을 수정한다.

**Files:**
- Modify: `internal/batchlock/batchlock.go`
- Modify: `internal/idprovider/rdbms/validator.go`
- Modify: `internal/mailservice/pop3_adapter.go`
- Modify: `internal/sso/verify.go`
- Modify: `internal/httpapi/admin_auth.go`
- Modify: `internal/httpapi/password_reset.go`

**Acceptance Criteria:**
- [ ] `batchlock.go`의 Lock/Unlock이 caller context를 사용한다
- [ ] `idprovider/rdbms/validator.go`가 caller context를 사용한다
- [ ] `pop3_adapter.go`가 요청 context를 사용한다
- [ ] `sso/verify.go`에서 이미 있는 `context.WithTimeout`이 caller ctx로부터 파생된다
- [ ] `go test ./...` 통과

**Verify:** `go test ./internal/batchlock/... ./internal/idprovider/... ./internal/sso/... -v` → PASS

**Steps:**

- [ ] **Step 1: batchlock.go — Lock/Unlock 시그니처 확인 후 ctx 전파**

```go
// 현재 패턴
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

// Lock(ctx context.Context, ...) 형태라면 이미 ctx를 받고 있을 가능성
// 시그니처 확인:
grep -n "^func " internal/batchlock/batchlock.go
```

내부 전용 goroutine (배치 워커에서 시작, 자체 수명주기)이 맞으면 `context.Background()` 유지.
caller context를 받는 exported 함수 내부라면 전달받은 ctx로부터 `WithTimeout` 파생.

```go
// 변경 예 (caller context가 있는 경우)
ctx, cancel := context.WithTimeout(ctx, 5*time.Second) // ctx = caller's
```

- [ ] **Step 2: idprovider/rdbms/validator.go**

```go
// 변경 전
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

// 변경 후 (Validate 함수가 ctx를 받는다면)
ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
```

Validate 함수 시그니처 확인:
```bash
grep -n "^func.*Validate" internal/idprovider/rdbms/validator.go
```

- [ ] **Step 3: mailservice/pop3_adapter.go**

```go
// 변경 전 (line 43)
ctx := context.Background()

// 변경 후: 어댑터 메서드가 이미 request-scoped ctx를 받는다면
// 메서드 시그니처 확인 후 해당 ctx 사용
grep -n "^func " internal/mailservice/pop3_adapter.go
```

- [ ] **Step 4: sso/verify.go — context.Background() 제거**

Task 1에서 이미 수정되었을 수 있음. 확인 후:

```go
// fetchDiscoveryDocument(ctx context.Context, ...) 같은 형태로
// caller에서 이미 ctx를 전달받고 있다면:
ctx, cancel := context.WithTimeout(ctx, 15*time.Second) // ctx = caller's
```

- [ ] **Step 5: 전체 수정 후 테스트**

```bash
go build ./...
go test ./internal/batchlock/... ./internal/idprovider/... \
        ./internal/mailservice/... ./internal/sso/... -v 2>&1 | tail -30
```

- [ ] **Step 6: app/run.go의 shutdown context는 그대로**

`run.go` 내의 `context.WithTimeout(context.Background(), 30*time.Second)` 패턴은 graceful shutdown용이라 `context.Background()` 가 맞다. 수정하지 않는다.

- [ ] **Step 7: 커밋**

```bash
git add internal/batchlock/batchlock.go internal/idprovider/rdbms/validator.go \
        internal/mailservice/pop3_adapter.go internal/sso/verify.go \
        internal/httpapi/admin_auth.go internal/httpapi/password_reset.go
git commit -m "fix(ctx): propagate caller context instead of context.Background() in service code"
```

---

### Task 6: 핵심 패키지 에러 래핑

**Goal:** 디버깅이 가장 어려운 핵심 패키지들(jmap, mailservice, httpapi)의 `return err` 패턴을 `fmt.Errorf("...: %w", err)`로 교체한다. 전체 852건이 아닌 고영향 패키지를 대상으로 한다.

**Files:**
- Modify: `internal/jmap/*.go` (테스트 제외)
- Modify: `internal/mailservice/service.go`
- Modify: `internal/httpapi/mail.go`

**Acceptance Criteria:**
- [ ] `internal/jmap` 패키지에서 bare `return err`가 0건이다 (기존 컨텍스트 없는 것)
- [ ] `internal/mailservice/service.go`의 주요 exported 함수에서 bare `return err`가 0건이다
- [ ] `go test ./internal/jmap/... ./internal/mailservice/...` 통과
- [ ] `.golangci.yml`의 wrapcheck가 신규 위반을 잡아낸다

**Verify:** `go test ./internal/jmap/... ./internal/mailservice/... -v 2>&1 | tail -10` → all PASS

**Steps:**

- [ ] **Step 1: jmap 패키지 분석**

```bash
grep -rn "return err$" internal/jmap/ --include="*.go" | grep -v "_test.go"
```

각 위치별로:
```go
// 변경 전
return err

// 변경 후 (함수명과 컨텍스트에 맞게)
return fmt.Errorf("jmap handler: %w", err)
// 또는
return fmt.Errorf("Email/get: %w", err)
```

- [ ] **Step 2: mailservice/service.go 주요 함수**

```bash
grep -n "return err$" internal/mailservice/service.go | head -30
```

exported 함수 내부의 bare `return err`를 래핑:

```go
// 예: CreateDraft
func (s *Service) CreateDraft(ctx context.Context, req maildb.CreateDraftRequest) (maildb.Draft, error) {
    draft, err := s.repository.CreateDraft(ctx, req)
    if err != nil {
        return maildb.Draft{}, fmt.Errorf("create draft: %w", err) // was: return maildb.Draft{}, err
    }
    return draft, nil
}
```

- [ ] **Step 3: .golangci.yml에 wrapcheck 추가 (선택적)**

```yaml
linters:
  enable:
    - wrapcheck   # 외부 패키지 에러 래핑 강제

linters-settings:
  wrapcheck:
    ignore-sigs:
      - .Errorf(
      - errors.New(
      - errors.Unwrap(
```

- [ ] **Step 4: 테스트**

```bash
go test ./internal/jmap/... ./internal/mailservice/... -count=1 2>&1 | tail -15
```

- [ ] **Step 5: 커밋**

```bash
git add internal/jmap/ internal/mailservice/service.go .golangci.yml
git commit -m "fix(errors): wrap bare return err with context in jmap and mailservice"
```

---

### Task 7: internal/app/run.go 분리

**Goal:** 4,047줄의 `run.go`를 서브시스템별 파일로 분리해 유지보수성을 높인다. `Run()` 함수는 `run.go`에 남기고 서브시스템 기동 함수들을 각자 파일로 이동한다.

**Files:**
- Modify: `internal/app/run.go` (Run, 타입 정의, 공통 유틸리티만 유지)
- Create: `internal/app/run_imap.go` (lines ~363–569: IMAP 게이트웨이)
- Create: `internal/app/run_pop3.go` (lines ~570–658: POP3 게이트웨이)
- Create: `internal/app/run_dav.go` (lines ~659–840: CalDAV/CardDAV/WebDAV)
- Create: `internal/app/run_ldap.go` (lines ~841–1200: LDAP 게이트웨이)
- Create: `internal/app/run_smtp.go` (SMTP inbound/outbound 관련)
- Create: `internal/app/run_http.go` (HTTP API 서버)
- Create: `internal/app/run_batch.go` (lines ~155–362: 배치 워커)

**Acceptance Criteria:**
- [ ] `internal/app/run.go`가 2,000줄 이하로 줄어든다
- [ ] 모든 신규 파일이 동일 패키지 `app`을 사용한다
- [ ] `go build ./internal/app/...` 통과
- [ ] `go test ./internal/app/...` 통과 (기존 테스트 그대로)

**Verify:** `wc -l internal/app/run.go` → 2000 이하; `go test ./internal/app/... -v` → PASS

**Steps:**

- [ ] **Step 1: 분리 대상 함수 목록 확인**

```bash
grep -n "^func " internal/app/run.go
```

출력 결과로 각 함수의 line range를 확인하고 파일별 귀속을 결정한다.

- [ ] **Step 2: run_batch.go 생성**

`package app` 헤더와 배치 워커 관련 함수(runBatchWorker, 관련 인터페이스/타입들)를 이동:

```go
package app

import (
    // run.go에서 사용하는 것과 동일한 import — 사용하는 것만
    "context"
    "log/slog"
    // ...
)

// runBatchWorker starts the background batch processor.
// 원래 run.go의 func runBatchWorker(...)를 그대로 이동
```

- [ ] **Step 3: run_imap.go 생성**

IMAP 관련 타입과 함수 이동:
- `fanOutAdapter`
- `imapGatewayRuntime`
- `newIMAPGatewayRuntime`
- `imapServerOptionsForConfig`
- `newIMAPServer`
- `newIMAPMailboxEventRouter`
- `runIMAPGateway`

- [ ] **Step 4: run_pop3.go 생성**

POP3 관련 함수 이동:
- `pop3TLSConfig`
- `runPOP3Gateway`
- `pop3ServerForConfig`

- [ ] **Step 5: run_dav.go 생성**

CalDAV/CardDAV/WebDAV 관련 함수 이동:
- `runCalDAVGateway`, `newCalDAVHTTPServer`
- `runCardDAVGateway`, `newCardDAVHTTPServer`
- `runWebDAVGateway`, `newWebDAVHTTPServer`

- [ ] **Step 6: run_ldap.go 생성**

LDAP 관련 함수와 타입 이동:
- `runLDAPGateway`, `ldapTLSConfig`
- `ldapDirectoryQuerier`와 모든 메서드
- `ldapShouldExpandGroupMembers`, `ldapShouldExpandMemberOf`, `ldapAttributeRequested`
- `ldapPrincipalEntry`

- [ ] **Step 7: run.go에서 이동된 함수 제거 후 빌드 확인**

```bash
go build ./internal/app/...
```

빌드 에러 → 이동 누락 함수 또는 import 누락 수정.

- [ ] **Step 8: 테스트**

```bash
go test ./internal/app/... -v -timeout=120s 2>&1 | tail -30
```

- [ ] **Step 9: 라인 수 확인**

```bash
wc -l internal/app/run*.go | sort -rn
```

- [ ] **Step 10: 커밋**

```bash
git add internal/app/
git commit -m "refactor(app): split 4047-line run.go into subsystem files (imap, pop3, dav, ldap, batch)"
```

---

### Task 8: internal/httpapi/mail.go 분리

**Goal:** 3,726줄의 `mail.go`를 도메인별 파일로 분리한다.

**Files:**
- Modify: `internal/httpapi/mail.go` (RegisterMailRoutes, 공통 helpers만 유지 → ~600줄 목표)
- Create: `internal/httpapi/mail_folders.go` (폴더 CRUD 핸들러)
- Create: `internal/httpapi/mail_messages.go` (메시지 조회/검색 핸들러)
- Create: `internal/httpapi/mail_drafts.go` (드래프트/첨부파일 핸들러)
- Create: `internal/httpapi/mail_threads.go` (스레드 핸들러)
- Create: `internal/httpapi/mail_contacts.go` (연락처/프로필 핸들러)

**Acceptance Criteria:**
- [ ] `mail.go`가 1,000줄 이하로 줄어든다
- [ ] `go build ./internal/httpapi/...` 통과
- [ ] `go test ./internal/httpapi/...` 통과

**Verify:** `wc -l internal/httpapi/mail.go` → 1000 이하; `go test ./internal/httpapi/... -v` → PASS

**Steps:**

- [ ] **Step 1: 핸들러 함수 도메인별 분류**

```bash
grep -n "^func \|^// " internal/httpapi/mail.go | head -80
```

- RegisterMailRoutesWithOptions 내부의 `mux.HandleFunc` 패턴 확인
- 각 핸들러가 어느 도메인에 속하는지 결정

- [ ] **Step 2: mail_folders.go 생성**

폴더 관련 핸들러(handleListFolders, handleCreateFolder, handleRenameFolder, handleDeleteFolder 등):

```go
package httpapi

// 폴더 핸들러들을 이곳으로 이동
// import는 mail.go에서 사용하는 것과 동일한 것만 포함
```

- [ ] **Step 3: mail_messages.go 생성**

메시지 조회, 플래그 변경, 이동, 삭제 핸들러.

- [ ] **Step 4: mail_drafts.go 생성**

드래프트 CRUD, 첨부파일 업로드/다운로드 핸들러.

- [ ] **Step 5: mail_threads.go 생성**

스레드 목록, 스레드별 메시지 핸들러.

- [ ] **Step 6: mail.go 정리**

남은 내용: RegisterMailRoutes, RegisterMailRoutesWithOptions, helper 함수들 (parseQueryLimit, decodeJSONBody 등), 공통 타입 정의.

- [ ] **Step 7: 빌드 및 테스트**

```bash
go build ./internal/httpapi/...
go test ./internal/httpapi/... -v -timeout=120s 2>&1 | tail -30
```

- [ ] **Step 8: 커밋**

```bash
git add internal/httpapi/mail*.go
git commit -m "refactor(httpapi): split 3726-line mail.go into domain handler files"
```

---

### Task 9: internal/mailservice/service.go 분리

**Goal:** 3,415줄의 `service.go`를 기능별 파일로 분리한다.

**Files:**
- Modify: `internal/mailservice/service.go` (Service 구조체, New(), With*() 메서드만 → ~200줄 목표)
- Create: `internal/mailservice/service_folders.go`
- Create: `internal/mailservice/service_messages.go`
- Create: `internal/mailservice/service_threads.go`
- Create: `internal/mailservice/service_drafts.go`
- Create: `internal/mailservice/service_attachments.go`
- Create: `internal/mailservice/service_search.go`
- Create: `internal/mailservice/service_delivery.go`

**Acceptance Criteria:**
- [ ] `service.go`가 300줄 이하로 줄어든다
- [ ] `go build ./internal/mailservice/...` 통과
- [ ] `go test ./internal/mailservice/...` 통과

**Verify:** `wc -l internal/mailservice/service.go` → 300 이하; `go test ./internal/mailservice/... -v` → PASS

**Steps:**

- [ ] **Step 1: 함수 분류**

```bash
grep -n "^func (s \*Service)" internal/mailservice/service.go
```

메서드별 도메인:
- `ListFolders`, `CreateFolder`, `RenameFolder`, `DeleteFolder` → `service_folders.go`
- `ListMessages`, `GetMessage`, `MoveMessages`, `DeleteMessages` → `service_messages.go`
- `ListThreads`, `ListThreadMessages` → `service_threads.go`
- `CreateDraft`, `UpdateDraft`, `SendDraft`, `DeleteDraft` → `service_drafts.go`
- `CreateUploadSession`, `AppendChunk`, ... → `service_attachments.go`
- `SearchMessages`, `SearchDrafts` → `service_search.go`
- `DeliverMessage`, `IngestInbound`, ... → `service_delivery.go`

- [ ] **Step 2: 각 파일 생성 (동일 패키지)**

```go
// service_folders.go
package mailservice

import (
    "context"
    "fmt"
    "github.com/gogomail/gogomail/internal/maildb"
)

func (s *Service) ListFolders(ctx context.Context, userID string) ([]maildb.Folder, error) {
    // 원래 코드 그대로 이동
}
// ... 나머지 폴더 메서드
```

- [ ] **Step 3: service.go에서 이동된 코드 제거**

이동 완료 후 service.go에는 다음만 남김:
- `package mailservice`
- import 블록 (사용하는 것만)
- `Service` 구조체 정의
- `New()` 생성자
- `With*()` 옵션 메서드
- 내부 유틸리티 (emitQuotaWarningIfNeeded, lookupGCStoragePaths 등)

- [ ] **Step 4: 빌드 및 테스트**

```bash
go build ./internal/mailservice/...
go test ./internal/mailservice/... -v -timeout=180s 2>&1 | tail -40
```

- [ ] **Step 5: 커밋**

```bash
git add internal/mailservice/service*.go
git commit -m "refactor(mailservice): split 3415-line service.go into domain files"
```

---

### Task 10: internal/maildb/admin_api_usage.go 분리

**Goal:** 2,361줄의 `admin_api_usage.go`를 책임별 파일로 분리한다.

**Files:**
- Modify: `internal/maildb/admin_api_usage.go` (공통 타입, 인터페이스만 유지)
- Create: `internal/maildb/admin_api_usage_ledger.go` (ledger CRUD)
- Create: `internal/maildb/admin_api_usage_aggregate.go` (집계 쿼리)
- Create: `internal/maildb/admin_api_usage_export.go` (export 배치)
- Create: `internal/maildb/admin_api_usage_retention.go` (retention runs)

**Acceptance Criteria:**
- [ ] `admin_api_usage.go`가 600줄 이하로 줄어든다
- [ ] `go build ./internal/maildb/...` 통과
- [ ] `go test ./internal/maildb/...` 통과

**Verify:** `wc -l internal/maildb/admin_api_usage.go` → 600 이하; `go test ./internal/maildb/... -count=1` → PASS

**Steps:**

- [ ] **Step 1: 함수 분류**

```bash
grep -n "^func " internal/maildb/admin_api_usage.go
```

- Ledger CRUD (Insert, Stream, Stats) → `admin_api_usage_ledger.go`
- Aggregate 쿼리 → `admin_api_usage_aggregate.go`
- Export 배치 함수 → `admin_api_usage_export.go`
- Retention runs → `admin_api_usage_retention.go`

- [ ] **Step 2~4: 각 파일 생성 및 이동 (service 분리와 동일한 방법)**

- [ ] **Step 5: 빌드 및 테스트**

```bash
go build ./internal/maildb/...
go test ./internal/maildb/... -count=1 -timeout=300s 2>&1 | tail -20
```

- [ ] **Step 6: 커밋**

```bash
git add internal/maildb/admin_api_usage*.go
git commit -m "refactor(maildb): split 2361-line admin_api_usage.go into domain files"
```

---

### Task 11: imapgw server_search.go + server_fetch.go 추가 분리

**Goal:** 분리 후에도 2,000줄 이상인 `server_search.go`(2,170줄)와 `server_fetch.go`(2,005줄)을 1,200줄 이하로 더 분리한다.

**Files:**
- Modify: `internal/imapgw/server_search.go` → 핵심 search 디스패처만 유지
- Create: `internal/imapgw/server_search_criteria.go` (search criteria 파싱)
- Create: `internal/imapgw/server_search_executor.go` (search 실행 로직)
- Modify: `internal/imapgw/server_fetch.go` → FETCH 디스패처만 유지
- Create: `internal/imapgw/server_fetch_body.go` (BODYSTRUCTURE, BODY[] 처리)
- Create: `internal/imapgw/server_fetch_envelope.go` (ENVELOPE, RFC822 처리)

**Acceptance Criteria:**
- [ ] `server_search.go`가 1,200줄 이하
- [ ] `server_fetch.go`가 1,200줄 이하
- [ ] `go test ./internal/imapgw/... -short` 통과

**Verify:** `wc -l internal/imapgw/server_search.go internal/imapgw/server_fetch.go` → 각 1200 이하; `go test ./internal/imapgw/... -short -v` → PASS

**Steps:**

- [ ] **Step 1: search criteria 파싱 함수 추출**

```bash
grep -n "^func.*[Cc]riteria\|^func.*[Pp]arse\|^func.*[Ss]earch" internal/imapgw/server_search.go | head -20
```

파싱 전용 함수들을 `server_search_criteria.go`로 이동.

- [ ] **Step 2: search 실행 로직 추출**

실제 DB 쿼리 조합 및 결과 필터링 함수 → `server_search_executor.go`.

- [ ] **Step 3: fetch BODY 처리 추출**

```bash
grep -n "^func.*[Bb]ody\|^func.*[Bb]odystruct\|^func.*MIME" internal/imapgw/server_fetch.go | head -20
```

BODY[] / BODYSTRUCTURE 렌더링 → `server_fetch_body.go`.

- [ ] **Step 4: fetch ENVELOPE 처리 추출**

ENVELOPE, RFC822, HEADER 렌더링 → `server_fetch_envelope.go`.

- [ ] **Step 5: 빌드 및 테스트**

```bash
go build ./internal/imapgw/...
go test ./internal/imapgw/... -short -v 2>&1 | tail -30
wc -l internal/imapgw/server_search.go internal/imapgw/server_fetch.go
```

- [ ] **Step 6: 커밋**

```bash
git add internal/imapgw/
git commit -m "refactor(imapgw): further split server_search.go and server_fetch.go"
```

---

## 실행 순서 (의존성)

```
Task 1 (HTTP 타임아웃)
Task 2 (os.Getenv)      ← Task 1과 독립
Task 3 (JMAP panic)     ← 독립
Task 4 (CI 강화)        ← 독립
Task 5 (ctx 전파)       ← Task 1 완료 후 (sso/verify.go 충돌 방지)
Task 6 (에러 래핑)      ← Task 3 완료 후
Task 7 (run.go 분리)    ← Task 2 완료 후 (config 변경 완료 후)
Task 8 (mail.go 분리)   ← Task 2, 6 완료 후
Task 9 (service 분리)   ← Task 6 완료 후
Task 10 (admin_api_usage 분리)  ← 독립
Task 11 (imapgw 추가 분리)      ← 독립
```

Tasks 1, 2, 3, 4, 10, 11은 병렬 실행 가능.
Tasks 5, 6은 Tasks 1, 3 완료 후.
Tasks 7, 8, 9는 Tasks 2, 6 완료 후.
