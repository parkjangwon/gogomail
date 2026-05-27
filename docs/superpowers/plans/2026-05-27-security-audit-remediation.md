# Security Audit Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** CSO 감사에서 발견된 10개 보안 취약점을 Go 백엔드, Helm/Docker, Next.js 세 영역에서 모두 수정한다.

**Architecture:** 각 태스크는 독립적으로 커밋 가능한 단위로 설계됐다. JWT 마이그레이션은 기존 `Claims` 인터페이스를 유지하면서 내부 구현만 `golang-jwt/jwt/v5`로 교체한다. Redis rate limiter는 인터페이스를 통해 httpapi가 Redis에 직접 의존하지 않도록 한다. CSP nonce는 Next.js middleware에서 처리한다.

**Tech Stack:** Go 1.25, golang-jwt/jwt/v5 (기존 의존성), redis/go-redis/v9 (기존), Next.js 15 App Router, Helm 3

---

## File Map

| 변경 파일 | 용도 |
|-----------|------|
| `internal/config/config.go` | GOGOMAIL_ENV 기본값, APNS key file 옵션 |
| `internal/config/config_file.go` | APNS key file YAML 키 파싱 |
| `internal/app/run_push.go` | APNS key file 로딩 |
| `internal/httpapi/admin_middleware.go` | StripInternalHeadersMiddleware 추가 |
| `internal/app/run.go` | 미들웨어 체인에 헤더 스트리핑 삽입, Redis rate limiter 주입 |
| `internal/httpapi/admin_types.go` | LoginRateLimiter 인터페이스, WithLoginRateLimiter 옵션 |
| `internal/httpapi/admin.go` | loginLimiter 타입 → 인터페이스 |
| `internal/httpapi/admin_rate_limiter_redis.go` | Redis 기반 login rate limiter (신규) |
| `internal/auth/jwt.go` | golang-jwt/jwt/v5로 내부 구현 교체 |
| `internal/auth/password.go` | VerifyPasswordHashResult, GenerateSalt 추가 |
| `internal/maildb/user_auth.go` | 로그인 성공 시 레거시 해시 자동 업그레이드 |
| `internal/idprovider/rdbms/provider.go` | validateSourceQuery 추가 |
| `helm/gogomail/templates/_helpers.tpl` | requireNotChangeme 헬퍼 |
| `helm/gogomail/templates/secret.yaml` | CHANGEME 가드 적용 |
| `docker/docker-compose.scale.yml` | sslmode=disable → require |
| `docker/.env.example` | APNS_PRIVATE_KEY_FILE 문서화 |
| `apps/webmail/src/middleware.ts` | nonce 생성 + CSP 헤더 |
| `apps/webmail/src/app/layout.tsx` | nonce를 inline script에 적용 |
| `apps/webmail/next.config.ts` | script-src에서 unsafe-inline 제거 |
| `apps/console/src/middleware.ts` | 동일 |
| `apps/console/src/app/layout.tsx` | 동일 (inline script 여부 확인 후 적용) |
| `apps/console/next.config.ts` | 동일 |

---

## Task 0: Secure defaults — GOGOMAIL_ENV + sslmode

**Goal:** GOGOMAIL_ENV 기본값을 `production`으로 바꾸고, docker-compose.scale.yml의 DB URL 기본값에서 `sslmode=disable`을 제거한다.

**Files:**
- Modify: `internal/config/config.go:371`
- Modify: `docker/docker-compose.scale.yml:27`

**Acceptance Criteria:**
- [ ] `config.go` 에서 `envOrDefault("GOGOMAIL_ENV", "development")` → `envOrDefault("GOGOMAIL_ENV", "production")` 로 변경됨
- [ ] `docker-compose.scale.yml` 기본 DB URL이 `sslmode=require` 를 사용함
- [ ] `go test ./internal/config/...` 통과

**Verify:** `go test ./internal/config/... -v 2>&1 | tail -5` → `ok` 라인 출력

**Steps:**

- [ ] **Step 1: config.go 기본값 변경**

`internal/config/config.go` 에서:
```go
// 변경 전
Environment: envOrDefault("GOGOMAIL_ENV", "development"),
// 변경 후
Environment: envOrDefault("GOGOMAIL_ENV", "production"),
```

- [ ] **Step 2: `config_test.go` 또는 extra test에서 development 하드코딩 확인**

```bash
grep -rn '"development"' internal/config/ --include="*.go" | grep -v "_test.go"
```

기본값을 직접 테스트하는 코드가 있으면 `t.Setenv("GOGOMAIL_ENV", "development")` 로 변경.

- [ ] **Step 3: docker-compose.scale.yml sslmode 변경**

`docker/docker-compose.scale.yml:27` 에서:
```yaml
# 변경 전
GOGOMAIL_DATABASE_URL: ${GOGOMAIL_DATABASE_URL:-postgres://gogomail:gogomail@postgres:5432/gogomail?sslmode=disable}
# 변경 후
GOGOMAIL_DATABASE_URL: ${GOGOMAIL_DATABASE_URL:-postgres://gogomail:gogomail@postgres:5432/gogomail?sslmode=require}
```

- [ ] **Step 4: 테스트**

```bash
go test ./internal/config/... -v 2>&1 | grep -E "^(ok|FAIL|---)"
```

모든 줄이 `ok` 또는 `--- PASS` 이어야 함.

- [ ] **Step 5: 커밋**

```bash
git add internal/config/config.go docker/docker-compose.scale.yml
git commit -m "security: default GOGOMAIL_ENV to production; sslmode=require in scale compose

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Task 1: Strip internal proxy headers (#3)

**Goal:** 외부 요청에서 들어오는 `X-Gogomail-Resolved-User-ID` 등 내부 전용 헤더를 HTTP 핸들러 체인 최입구에서 제거하여 metering 데이터 위조를 차단한다.

**Files:**
- Modify: `internal/httpapi/admin_middleware.go`
- Modify: `internal/app/run.go` (미들웨어 삽입)
- Test: `internal/httpapi/admin_middleware_test.go` (기존 파일에 테스트 추가)

**Acceptance Criteria:**
- [ ] `StripInternalHeadersMiddleware` 가 6개 헤더를 모두 요청에서 제거함
- [ ] 핸들러 체인에서 `RequestIDMiddleware` 바로 다음에 적용됨
- [ ] `go test ./internal/httpapi/... -run TestStripInternalHeaders` 통과

**Verify:** `go test ./internal/httpapi/... -run TestStrip -v` → `PASS`

**Steps:**

- [ ] **Step 1: StripInternalHeadersMiddleware 추가**

`internal/httpapi/admin_middleware.go` 끝에 추가:
```go
// internalProxyHeaders lists headers that are set by internal middleware only.
// Any value sent by an external caller is stripped before processing.
var internalProxyHeaders = []string{
    "X-Gogomail-Resolved-User-ID",
    "X-Gogomail-Tenant-ID",
    "X-Gogomail-Company-ID",
    "X-Gogomail-Domain-ID",
    "X-Gogomail-Principal-ID",
    "X-Gogomail-API-Key-ID",
}

// StripInternalHeadersMiddleware removes internal proxy headers from every
// inbound request. This prevents external callers from spoofing metering
// or billing attribution.
func StripInternalHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        for _, h := range internalProxyHeaders {
            r.Header.Del(h)
        }
        next.ServeHTTP(w, r)
    })
}
```

- [ ] **Step 2: 테스트 작성 (먼저 실패를 확인한 뒤 위 코드 추가)**

`internal/httpapi/admin_middleware_test.go` 에 추가:
```go
func TestStripInternalHeadersMiddleware(t *testing.T) {
    headers := []string{
        "X-Gogomail-Resolved-User-ID",
        "X-Gogomail-Tenant-ID",
        "X-Gogomail-Company-ID",
        "X-Gogomail-Domain-ID",
        "X-Gogomail-Principal-ID",
        "X-Gogomail-API-Key-ID",
    }
    var captured http.Header
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        captured = r.Header.Clone()
    })
    handler := StripInternalHeadersMiddleware(next)

    req := httptest.NewRequest(http.MethodGet, "/", nil)
    for _, h := range headers {
        req.Header.Set(h, "attacker-value")
    }
    handler.ServeHTTP(httptest.NewRecorder(), req)

    for _, h := range headers {
        if v := captured.Get(h); v != "" {
            t.Errorf("header %q not stripped, got %q", h, v)
        }
    }
}
```

- [ ] **Step 3: 테스트 실패 확인 후 코드 추가, 테스트 통과 확인**

```bash
go test ./internal/httpapi/... -run TestStripInternalHeaders -v
```

- [ ] **Step 4: run.go 미들웨어 삽입**

`internal/app/run.go` 에서 `handler = httpapi.RequestIDMiddleware(handler)` 줄 바로 다음 줄에 삽입:
```go
handler = httpapi.StripInternalHeadersMiddleware(handler)
```

- [ ] **Step 5: 전체 테스트**

```bash
go test ./internal/httpapi/... ./internal/app/... 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 6: 커밋**

```bash
git add internal/httpapi/admin_middleware.go internal/httpapi/admin_middleware_test.go internal/app/run.go
git commit -m "security: strip internal X-Gogomail-* proxy headers on ingress

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Task 2: APNS private key file option (#10)

**Goal:** `GOGOMAIL_APNS_PRIVATE_KEY_FILE` 환경변수를 추가해 APNS 키를 파일로 마운트할 수 있게 한다. 파일 경로가 설정된 경우 env var보다 우선한다.

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_file.go`
- Modify: `internal/app/run_push.go`
- Modify: `docker/.env.example`

**Acceptance Criteria:**
- [ ] `GOGOMAIL_APNS_PRIVATE_KEY_FILE` 이 설정되면 해당 파일 내용을 `APNSPrivateKey` 로 사용함
- [ ] 파일 경로와 env var가 모두 설정되면 파일 경로가 우선함
- [ ] 파일이 존재하지 않으면 명확한 에러 메시지 반환

**Verify:** `go test ./internal/config/... ./internal/app/... 2>&1 | grep -E "^(ok|FAIL)"`

**Steps:**

- [ ] **Step 1: Config 구조체에 필드 추가**

`internal/config/config.go` 에서 `APNSPrivateKey string` 근처에 추가:
```go
APNSPrivateKeyFile string // GOGOMAIL_APNS_PRIVATE_KEY_FILE
```

환경변수 로딩 섹션에 추가 (APNSPrivateKey= 근처):
```go
APNSPrivateKeyFile: os.Getenv("GOGOMAIL_APNS_PRIVATE_KEY_FILE"),
```

- [ ] **Step 2: config_file.go YAML 파싱 추가**

`internal/config/config_file.go` 에서 `case "apns_private_key":` 케이스 다음에 추가:
```go
case "apns_private_key_file":
    return setYAMLString(value, &cfg.APNSPrivateKeyFile, key)
```

- [ ] **Step 3: run_push.go 에서 파일 로딩 우선 처리**

`internal/app/run_push.go` 에서 APNS 키를 사용하는 부분을 찾아 앞에 삽입:
```go
// Prefer key file over env var.
apnsKey := cfg.APNSPrivateKey
if strings.TrimSpace(cfg.APNSPrivateKeyFile) != "" {
    data, err := os.ReadFile(strings.TrimSpace(cfg.APNSPrivateKeyFile))
    if err != nil {
        return fmt.Errorf("read APNS private key file: %w", err)
    }
    apnsKey = string(data)
}
```
이후 `cfg.APNSPrivateKey` 참조를 `apnsKey` 로 변경.

- [ ] **Step 4: .env.example 문서화**

`docker/.env.example` 에서 `GOGOMAIL_APNS_PRIVATE_KEY=` 아래에 추가:
```bash
# Preferred over GOGOMAIL_APNS_PRIVATE_KEY when set.
# Mount the PEM file as a secret and point here.
# GOGOMAIL_APNS_PRIVATE_KEY_FILE=/run/secrets/apns_private_key.pem
```

- [ ] **Step 5: 테스트**

```bash
go test ./internal/config/... ./internal/app/... 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 6: 커밋**

```bash
git add internal/config/config.go internal/config/config_file.go internal/app/run_push.go docker/.env.example
git commit -m "security: add GOGOMAIL_APNS_PRIVATE_KEY_FILE option (prefer over env var)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Task 3: Helm CHANGEME guard (#2)

**Goal:** Helm chart 설치/업그레이드 시 DM 마스터 키, JWT 시크릿, 어드민 토큰이 기본 placeholder 값이면 배포를 실패시킨다.

**Files:**
- Modify: `helm/gogomail/templates/_helpers.tpl`
- Modify: `helm/gogomail/templates/secret.yaml`

**Acceptance Criteria:**
- [ ] `GOGOMAIL_DM_MASTER_KEY` 가 `CHANGEME` 를 포함하면 `helm template` 이 실패함
- [ ] `GOGOMAIL_AUTH_JWT_SECRET` 이 `CHANGEME` 를 포함하면 실패함
- [ ] `GOGOMAIL_ADMIN_TOKEN` 이 `CHANGEME` 를 포함하면 실패함
- [ ] 실제 값이 설정되면 정상 렌더링됨

**Verify:** `helm template ./helm/gogomail 2>&1 | grep -c "must be set"` → `3` (세 시크릿 모두 CHANGEME이므로)

**Steps:**

- [ ] **Step 1: _helpers.tpl에 헬퍼 추가**

`helm/gogomail/templates/_helpers.tpl` 끝에 추가:
```
{{/*
requireNotChangeme validates that a secret value has been changed from its placeholder.
Usage: {{ include "gogomail.requireNotChangeme" (dict "name" "SECRET_NAME" "value" .Values.secrets.SECRET_NAME) }}
*/}}
{{- define "gogomail.requireNotChangeme" -}}
{{- $name := .name -}}
{{- $val  := .value | default "" -}}
{{- if or (eq $val "") (contains "CHANGEME" $val) -}}
{{- fail (printf "Secret %s must be set to a non-placeholder value before deploying" $name) -}}
{{- end -}}
{{- end -}}
```

- [ ] **Step 2: secret.yaml에 가드 적용**

`helm/gogomail/templates/secret.yaml` 에서 세 시크릿을 사용하는 줄 바로 앞에 검증 추가:

파일 상단(또는 data 블록 앞)에:
```yaml
{{- include "gogomail.requireNotChangeme" (dict "name" "GOGOMAIL_DM_MASTER_KEY"       "value" .Values.secrets.GOGOMAIL_DM_MASTER_KEY) }}
{{- include "gogomail.requireNotChangeme" (dict "name" "GOGOMAIL_AUTH_JWT_SECRET"     "value" .Values.secrets.GOGOMAIL_AUTH_JWT_SECRET) }}
{{- include "gogomail.requireNotChangeme" (dict "name" "GOGOMAIL_ADMIN_TOKEN"          "value" .Values.secrets.GOGOMAIL_ADMIN_TOKEN) }}
```

- [ ] **Step 3: 기본값으로 렌더링 실패 확인**

```bash
helm template ./helm/gogomail 2>&1 | head -20
```
`Error: ... must be set to a non-placeholder value` 가 포함되어야 함.

- [ ] **Step 4: 실제 값으로 렌더링 성공 확인**

```bash
helm template ./helm/gogomail \
  --set secrets.GOGOMAIL_DM_MASTER_KEY=realkey123abc \
  --set secrets.GOGOMAIL_AUTH_JWT_SECRET=realjwtsecret32bytesminimum!! \
  --set secrets.GOGOMAIL_ADMIN_TOKEN=realadmintoken \
  2>&1 | grep -c "Error"
```
출력이 `0` 이어야 함.

- [ ] **Step 5: 커밋**

```bash
git add helm/gogomail/templates/_helpers.tpl helm/gogomail/templates/secret.yaml
git commit -m "security: fail helm deploy when secrets contain CHANGEME placeholder

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Task 4: RDBMS identity provider SQL allowlist (#8)

**Goal:** 관리자가 설정하는 `UserQuery`/`GroupQuery`가 SELECT 전용인지 검증하여 악의적인 SQL 실행을 차단한다.

**Files:**
- Modify: `internal/idprovider/rdbms/provider.go`
- Test: `internal/idprovider/rdbms/provider_test.go` (신규)

**Acceptance Criteria:**
- [ ] `validateSourceQuery("")` → 에러
- [ ] `validateSourceQuery("SELECT * FROM users")` → nil
- [ ] `validateSourceQuery("SELECT * FROM users UNION SELECT * FROM secrets")` → 에러
- [ ] `validateSourceQuery("DROP TABLE users")` → 에러
- [ ] `validateSourceQuery("  select id from t where x=$1  ")` → nil (대소문자 무관, 공백 허용)
- [ ] `Connect()` 시 UserQuery/GroupQuery 검증 실행

**Verify:** `go test ./internal/idprovider/rdbms/... -v -run TestValidateSourceQuery` → `PASS`

**Steps:**

- [ ] **Step 1: 테스트 파일 작성**

`internal/idprovider/rdbms/provider_test.go`:
```go
package rdbms

import (
    "testing"
)

func TestValidateSourceQuery(t *testing.T) {
    cases := []struct {
        name    string
        query   string
        wantErr bool
    }{
        {"empty", "", true},
        {"valid select", "SELECT id, name FROM users", false},
        {"valid select lowercase", "  select id from t where x=$1  ", false},
        {"no select prefix", "DELETE FROM users", true},
        {"union injection", "SELECT * FROM users UNION SELECT * FROM secrets", true},
        {"insert injection", "SELECT id FROM (INSERT INTO log VALUES(1)) t", true},
        {"drop injection", "SELECT id FROM t; DROP TABLE t", true},
        {"semicolon end only", "SELECT id FROM t;", false},
        {"semicolon in middle", "SELECT id FROM t; SELECT 1", true},
        {"too long", string(make([]byte, 4097)), true},
        {"exec injection", "SELECT EXEC('cmd')", true},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            err := validateSourceQuery(c.query)
            if c.wantErr && err == nil {
                t.Errorf("expected error, got nil")
            }
            if !c.wantErr && err != nil {
                t.Errorf("unexpected error: %v", err)
            }
        })
    }
}
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
go test ./internal/idprovider/rdbms/... -run TestValidateSourceQuery -v 2>&1 | head -20
```

- [ ] **Step 3: validateSourceQuery 구현**

`internal/idprovider/rdbms/provider.go` 상단 import에 `"regexp"` 추가 후, 파일 끝에 추가:
```go
// forbiddenQueryPattern matches SQL keywords that must not appear in a
// read-only source query.
var forbiddenQueryPattern = regexp.MustCompile(
    `(?i)\b(UNION|INSERT|UPDATE|DELETE|DROP|TRUNCATE|CREATE|ALTER|EXEC|EXECUTE|GRANT|REVOKE)\b`,
)

const maxSourceQueryBytes = 4096

// validateSourceQuery returns an error if query is not a safe read-only SELECT.
func validateSourceQuery(query string) error {
    q := strings.TrimSpace(query)
    if q == "" {
        return fmt.Errorf("source query is required")
    }
    if len(q) > maxSourceQueryBytes {
        return fmt.Errorf("source query must be <= %d bytes", maxSourceQueryBytes)
    }
    upper := strings.ToUpper(q)
    if !strings.HasPrefix(upper, "SELECT") {
        return fmt.Errorf("source query must start with SELECT")
    }
    if forbiddenQueryPattern.MatchString(q) {
        return fmt.Errorf("source query contains forbidden keyword")
    }
    // Allow a trailing semicolon but not a semicolon inside the query.
    trimmed := strings.TrimRight(q, " \t\r\n;")
    if strings.ContainsRune(trimmed, ';') {
        return fmt.Errorf("source query must not contain semicolons except at end")
    }
    return nil
}
```

- [ ] **Step 4: Connect()에서 검증 호출**

`Connect()` 함수의 `if p.config.UserQuery != ""` 블록 추가 (Ping 이후):
```go
if p.config.UserQuery != "" {
    if err := validateSourceQuery(p.config.UserQuery); err != nil {
        db.Close()
        return fmt.Errorf("invalid user_query: %w", err)
    }
}
if p.config.GroupQuery != "" {
    if err := validateSourceQuery(p.config.GroupQuery); err != nil {
        db.Close()
        return fmt.Errorf("invalid group_query: %w", err)
    }
}
```

- [ ] **Step 5: 테스트 통과 확인**

```bash
go test ./internal/idprovider/rdbms/... -run TestValidateSourceQuery -v
```

- [ ] **Step 6: 커밋**

```bash
git add internal/idprovider/rdbms/provider.go internal/idprovider/rdbms/provider_test.go
git commit -m "security: validate RDBMS identity provider SQL queries (SELECT-only allowlist)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Task 5: JWT 내부 구현 → golang-jwt/jwt/v5 (#5)

**Goal:** 손으로 작성된 `internal/auth/jwt.go` 내부 구현을 `golang-jwt/jwt/v5` 라이브러리로 교체한다. 외부에 노출된 `Claims`·`TokenManager` 타입과 모든 테스트는 그대로 통과해야 한다. 토큰 포맷(HS256)이 동일하므로 기존 발급 토큰은 유효성을 유지한다.

**Files:**
- Modify: `internal/auth/jwt.go`

**Acceptance Criteria:**
- [ ] `go test ./internal/auth/... -run TestJWT` 전체 통과
- [ ] `jwt_secret_trim_extra_test.go`, `jwt_default_ttl_extra_test.go`, `session_revocation_extra_test.go` 통과
- [ ] `Sign` 으로 생성한 토큰을 `Verify` 로 검증 가능
- [ ] `alg: none` 공격 → 에러 반환

**Verify:** `go test ./internal/auth/... -v 2>&1 | grep -E "^(ok|FAIL|--- (PASS|FAIL))"` → 모두 PASS

**Steps:**

- [ ] **Step 1: 현재 테스트 기준선 확인**

```bash
go test ./internal/auth/... -v 2>&1 | grep -E "--- (PASS|FAIL)"
```

모두 PASS인 상태를 기록해둔다.

- [ ] **Step 2: jwt.go 교체**

`internal/auth/jwt.go` 의 import 블록과 `sign()` / `Verify()` 내부를 교체한다. 기존 파일을 아래 내용으로 전체 교체:

```go
package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// RevocationChecker lets the TokenManager validate session_version on every request.
type RevocationChecker interface {
	SessionVersionFor(ctx context.Context, userID string) (int64, error)
}

// SessionRevoker increments a user's session_version, invalidating all existing tokens.
type SessionRevoker interface {
	IncrementSessionVersion(ctx context.Context, userID string) (int64, error)
}

const (
	maxJWTTokenBytes    = 8192
	maxJWTIdentityBytes = 200
)

// Claims holds the parsed, validated fields of a GoGoMail JWT.
type Claims struct {
	Subject        string    `json:"sub"`
	UserID         string    `json:"user_id"`
	DomainID       string    `json:"domain_id"`
	CompanyID      string    `json:"company_id,omitempty"`
	Role           string    `json:"role"`
	SessionVersion int64     `json:"session_ver,omitempty"`
	TokenType      string    `json:"token_type,omitempty"`
	MFAVerified    bool      `json:"mfa_verified,omitempty"`
	Expires        time.Time `json:"-"`
	Expiry         int64     `json:"exp"`
	IssuedAt       int64     `json:"iat"`
}

// jwtInternalClaims is the wire format used with golang-jwt/jwt/v5.
type jwtInternalClaims struct {
	UserID         string `json:"user_id"`
	DomainID       string `json:"domain_id"`
	CompanyID      string `json:"company_id,omitempty"`
	Role           string `json:"role"`
	SessionVersion int64  `json:"session_ver,omitempty"`
	TokenType      string `json:"token_type,omitempty"`
	MFAVerified    bool   `json:"mfa_verified,omitempty"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies GoGoMail JWTs.
type TokenManager struct {
	secret  []byte
	now     func() time.Time
	checker RevocationChecker
}

func (m *TokenManager) SetRevocationChecker(c RevocationChecker) {
	m.checker = c
}

// VerifyFull validates signature + expiry, then checks session_version against the
// RevocationChecker if one is configured.
func (m *TokenManager) VerifyFull(ctx context.Context, token string) (Claims, error) {
	claims, err := m.Verify(token)
	if err != nil {
		return Claims{}, err
	}
	if m.checker != nil {
		ver, err := m.checker.SessionVersionFor(ctx, claims.UserID)
		if err != nil {
			return Claims{}, fmt.Errorf("session check: %w", err)
		}
		if claims.SessionVersion < ver {
			return Claims{}, fmt.Errorf("session revoked")
		}
	}
	return claims, nil
}

func NewTokenManager(secret string) (*TokenManager, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}
	if len([]byte(secret)) < 32 {
		return nil, fmt.Errorf("jwt secret must be at least 32 bytes")
	}
	return &TokenManager{secret: []byte(secret), now: time.Now}, nil
}

func (m *TokenManager) Sign(claims Claims, ttl time.Duration) (string, error) {
	if m == nil || len(m.secret) == 0 {
		return "", fmt.Errorf("token manager is not configured")
	}
	now := m.now().UTC()
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	var err error
	claims.UserID, err = normalizeJWTIdentity(claims.UserID)
	if err != nil {
		return "", err
	}
	claims.Subject, err = normalizeJWTIdentity(claims.Subject)
	if err != nil {
		return "", err
	}
	if claims.UserID == "" && claims.Subject != "" {
		claims.UserID = claims.Subject
	}
	if claims.Subject == "" {
		claims.Subject = claims.UserID
	}
	if claims.Subject == "" {
		return "", fmt.Errorf("user_id is required")
	}

	internal := jwtInternalClaims{
		UserID:         claims.UserID,
		DomainID:       claims.DomainID,
		CompanyID:      claims.CompanyID,
		Role:           claims.Role,
		SessionVersion: claims.SessionVersion,
		TokenType:      claims.TokenType,
		MFAVerified:    claims.MFAVerified,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   claims.Subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, internal)
	return token.SignedString(m.secret)
}

func (m *TokenManager) Verify(tokenString string) (Claims, error) {
	if m == nil || len(m.secret) == 0 {
		return Claims{}, fmt.Errorf("token manager is not configured")
	}
	tokenString = strings.TrimSpace(tokenString)
	if len(tokenString) > maxJWTTokenBytes {
		return Claims{}, fmt.Errorf("jwt token is too long")
	}

	var internal jwtInternalClaims
	token, err := jwt.ParseWithClaims(tokenString, &internal, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unsupported jwt algorithm: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithLeeway(time.Minute), jwt.WithIssuedAt())
	if err != nil {
		return Claims{}, fmt.Errorf("invalid jwt: %w", err)
	}
	if !token.Valid {
		return Claims{}, fmt.Errorf("invalid jwt")
	}

	userID, err := normalizeJWTIdentity(internal.UserID)
	if err != nil {
		return Claims{}, err
	}
	subject, err := normalizeJWTIdentity(internal.Subject)
	if err != nil {
		return Claims{}, err
	}
	if userID == "" {
		userID = subject
	}
	if userID == "" {
		return Claims{}, fmt.Errorf("jwt missing user_id")
	}

	expiry := int64(0)
	var expires time.Time
	if internal.ExpiresAt != nil {
		expiry = internal.ExpiresAt.Unix()
		expires = internal.ExpiresAt.Time
	}
	issuedAt := int64(0)
	if internal.IssuedAt != nil {
		issuedAt = internal.IssuedAt.Unix()
	}

	return Claims{
		Subject:        subject,
		UserID:         userID,
		DomainID:       internal.DomainID,
		CompanyID:      internal.CompanyID,
		Role:           internal.Role,
		SessionVersion: internal.SessionVersion,
		TokenType:      internal.TokenType,
		MFAVerified:    internal.MFAVerified,
		Expires:        expires,
		Expiry:         expiry,
		IssuedAt:       issuedAt,
	}, nil
}

func normalizeJWTIdentity(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("jwt identity must not contain CR or LF")
	}
	if len(value) > maxJWTIdentityBytes {
		return "", fmt.Errorf("jwt identity is too long")
	}
	return value, nil
}

// MFAMode represents the multi-factor authentication enforcement level.
type MFAMode string

const (
	MFAModeDisabled MFAMode = "disabled"
	MFAModeOptional MFAMode = "optional"
	MFAModeRequired MFAMode = "required"
)

func (m MFAMode) IsValid() bool {
	switch m {
	case MFAModeDisabled, MFAModeOptional, MFAModeRequired:
		return true
	}
	return false
}
```

- [ ] **Step 3: 테스트 실행**

```bash
go test ./internal/auth/... -v 2>&1 | grep -E "^(ok|FAIL|--- (PASS|FAIL))"
```

모두 PASS여야 함. FAIL이 있으면 에러 메시지를 확인해 수정.

- [ ] **Step 4: 커밋**

```bash
git add internal/auth/jwt.go
git commit -m "refactor(auth): replace hand-rolled JWT with golang-jwt/jwt/v5

Keeps Claims/TokenManager interfaces identical.
Token format (HS256 HMAC-SHA256) unchanged — existing tokens remain valid.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Task 6: 레거시 패스워드 해시 로그인 시 자동 업그레이드 (#6)

**Goal:** `plain:` 또는 `sha256:` 형식의 해시로 로그인에 성공하면, 백그라운드에서 pbkdf2-sha256으로 즉시 재해시한다. 업그레이드 실패는 로그인 자체를 막지 않는다.

**Files:**
- Modify: `internal/auth/password.go`
- Modify: `internal/maildb/user_auth.go`
- Test: `internal/auth/password_test.go`, `internal/maildb/user_auth_test.go` (기존 파일 확장)

**Acceptance Criteria:**
- [ ] `VerifyPasswordHashResult("pw", "plain:pw")` → `(true, true)`
- [ ] `VerifyPasswordHashResult("pw", "sha256:<hex>")` → `(true, true)`
- [ ] `VerifyPasswordHashResult("pw", "pbkdf2-sha256$...")` → `(true, false)`
- [ ] `VerifyPasswordHashResult("wrong", "plain:pw")` → `(false, false)`
- [ ] `AuthenticateUser` 가 레거시 해시 로그인 성공 후 DB의 hash를 pbkdf2-sha256으로 업데이트함
- [ ] `go test ./internal/auth/... ./internal/maildb/... 2>&1 | grep FAIL` → 출력 없음

**Verify:** `go test ./internal/auth/... -run TestVerifyPasswordHashResult -v` → PASS

**Steps:**

- [ ] **Step 1: VerifyPasswordHashResult 테스트 작성**

`internal/auth/password_test.go` 에 추가:
```go
func TestVerifyPasswordHashResult(t *testing.T) {
    cases := []struct {
        name        string
        password    string
        hash        string
        wantOK      bool
        wantUpgrade bool
    }{
        {"plain match", "secret", "plain:secret", true, true},
        {"plain mismatch", "wrong", "plain:secret", false, false},
        {"sha256 match", "secret", func() string {
            sum := sha256.Sum256([]byte("secret"))
            return "sha256:" + hex.EncodeToString(sum[:])
        }(), true, true},
        {"sha256 mismatch", "wrong", func() string {
            sum := sha256.Sum256([]byte("secret"))
            return "sha256:" + hex.EncodeToString(sum[:])
        }(), false, false},
        {"pbkdf2 no upgrade", "secret", func() string {
            salt := make([]byte, 16)
            rand.Read(salt)
            h, _ := HashPasswordPBKDF2SHA256("secret", salt, 210_000)
            return h
        }(), true, false},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            ok, upgrade := VerifyPasswordHashResult(c.password, c.hash)
            if ok != c.wantOK {
                t.Errorf("ok: got %v want %v", ok, c.wantOK)
            }
            if upgrade != c.wantUpgrade {
                t.Errorf("upgrade: got %v want %v", upgrade, c.wantUpgrade)
            }
        })
    }
}
```

필요한 import 추가: `"crypto/rand"`, `"crypto/sha256"`, `"encoding/hex"`.

- [ ] **Step 2: VerifyPasswordHashResult + GenerateSalt 구현**

`internal/auth/password.go` 에 추가:
```go
// GenerateSalt returns n cryptographically random bytes for use as a PBKDF2 salt.
// Panics if the system CSPRNG fails (this should never happen in production).
func GenerateSalt(n int) []byte {
    if n <= 0 {
        n = 32
    }
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        panic(fmt.Sprintf("auth: generate salt: %v", err))
    }
    return b
}

// VerifyPasswordHashResult is like VerifyPasswordHash but additionally returns
// needsUpgrade=true when the hash used a legacy format (plain: or sha256:).
// Callers should re-hash with PBKDF2-SHA256 when needsUpgrade is true.
func VerifyPasswordHashResult(password string, encoded string) (verified bool, needsUpgrade bool) {
    encoded = strings.TrimSpace(encoded)
    if encoded == "" {
        return false, false
    }
    isLegacy := strings.HasPrefix(encoded, "plain:") || strings.HasPrefix(encoded, "sha256:")
    ok := VerifyPasswordHash(password, encoded)
    if !ok {
        return false, false
    }
    return true, isLegacy
}
```

import에 `"crypto/rand"` 추가 (아직 없다면).

- [ ] **Step 3: user_auth.go 에서 업그레이드 로직 추가**

`internal/maildb/user_auth.go` 에서 `AuthenticateUser` 를 수정:

기존:
```go
if !auth.VerifyPasswordHash(password, passwordHash) {
    return AuthenticatedUser{}, fmt.Errorf("invalid credentials")
}
return user, nil
```

변경:
```go
verified, needsUpgrade := auth.VerifyPasswordHashResult(password, passwordHash)
if !verified {
    return AuthenticatedUser{}, fmt.Errorf("invalid credentials")
}
if needsUpgrade {
    r.upgradePasswordHash(ctx, user.UserID, password)
}
return user, nil
```

같은 파일 끝에 추가:
```go
// upgradePasswordHash re-hashes password with PBKDF2-SHA256 and updates the DB.
// Best-effort: any error is logged but does not affect the caller.
func (r *Repository) upgradePasswordHash(ctx context.Context, userID, password string) {
    newHash, err := auth.HashPasswordPBKDF2SHA256(password, auth.GenerateSalt(32), 210_000)
    if err != nil {
        slog.WarnContext(ctx, "password hash upgrade: hash generation failed", "user_id", userID)
        return
    }
    _, err = r.db.ExecContext(ctx,
        `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1::uuid`,
        userID, newHash)
    if err != nil {
        slog.WarnContext(ctx, "password hash upgrade: db update failed", "user_id", userID)
        return
    }
    slog.InfoContext(ctx, "password hash upgraded to pbkdf2-sha256", "user_id", userID)
}
```

import에 `"log/slog"` 추가 (없으면).

- [ ] **Step 4: 테스트**

```bash
go test ./internal/auth/... -run TestVerifyPasswordHashResult -v
go test ./internal/maildb/... 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 5: 커밋**

```bash
git add internal/auth/password.go internal/maildb/user_auth.go
git commit -m "security: auto-upgrade legacy password hashes on successful login

plain: and sha256: hashes are silently upgraded to pbkdf2-sha256 (210k rounds)
on the next successful authentication. Best-effort — upgrade failure is logged
but does not block login.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Task 7: Redis 기반 어드민 로그인 rate limiter (#4)

**Goal:** 어드민 로그인 endpoint의 IP당 rate limit을 Redis를 사용해 다중 인스턴스 환경에서도 공유되도록 교체한다. Redis가 없을 때는 기존 in-memory로 폴백한다.

**Files:**
- Modify: `internal/httpapi/admin_types.go`
- Modify: `internal/httpapi/admin.go`
- Create: `internal/httpapi/admin_rate_limiter_redis.go`
- Modify: `internal/app/run.go`
- Test: `internal/httpapi/admin_rate_limiter_redis_test.go` (신규)

**Acceptance Criteria:**
- [ ] `WithRedisLoginLimiter(client)` 옵션이 존재하고 `adminRouteConfig` 에 저장됨
- [ ] Redis client가 주입되면 `RedisAdminLoginLimiter` 사용, 없으면 `AdminIPRateLimiter` 사용
- [ ] `registerAuthAndAdminUserRoutes` 가 인터페이스 타입(`adminLoginRateLimiter`)을 받음
- [ ] `go test ./internal/httpapi/... 2>&1 | grep FAIL` → 출력 없음

**Verify:** `go test ./internal/httpapi/... 2>&1 | grep -E "^(ok|FAIL)"` → `ok`

**Steps:**

- [ ] **Step 1: 인터페이스 + Redis 구현 파일 생성**

`internal/httpapi/admin_rate_limiter_redis.go` (신규):
```go
package httpapi

import (
	"net/http"
	"time"

	"github.com/gogomail/gogomail/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

// adminLoginRateLimiter is satisfied by both AdminIPRateLimiter (in-memory)
// and RedisAdminLoginLimiter (distributed).
type adminLoginRateLimiter interface {
	Middleware(next http.Handler) http.Handler
}

// RedisAdminLoginLimiter wraps ratelimit.RedisFixedWindowLimiter to implement
// adminLoginRateLimiter. It shares counters across all server instances.
type RedisAdminLoginLimiter struct {
	inner *ratelimit.RedisFixedWindowLimiter
}

// NewRedisAdminLoginLimiter returns a limiter backed by Redis.
// Falls back gracefully when the Redis client is nil.
func NewRedisAdminLoginLimiter(client *redis.Client, limit int64, window time.Duration) *RedisAdminLoginLimiter {
	return &RedisAdminLoginLimiter{
		inner: ratelimit.NewRedisFixedWindowLimiter(client, "admin:login", limit, window),
	}
}

func (l *RedisAdminLoginLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := adminClientIP(r)
		dec, err := l.inner.Allow(r.Context(), ip)
		if err != nil || !dec.Allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: admin_types.go 에 redisLoginClient 필드 + 옵션 추가**

`adminRouteConfig` 구조체에 추가:
```go
redisLoginClient interface {
    // github.com/redis/go-redis/v9.Cmdable subset used by ratelimit
    Incr(ctx interface{}, key string) interface{}
}
```

실제로는 `*redis.Client` 타입을 저장하면 된다. `admin_types.go` import에 `"github.com/redis/go-redis/v9"` 를 추가하고:
```go
// adminRouteConfig struct 에 추가:
redisLoginClient *redis.Client
```

옵션 함수 추가:
```go
// WithRedisLoginLimiter enables a distributed Redis-backed rate limiter for
// the admin login endpoint. Falls back to in-memory when not configured.
func WithRedisLoginLimiter(client *redis.Client) AdminRouteOption {
    return func(cfg *adminRouteConfig) { cfg.redisLoginClient = client }
}
```

- [ ] **Step 3: registerAdminUtilityRoutes 에서 분기**

`internal/httpapi/admin.go:70` 근처 변경:
```go
// 변경 전:
loginLimiter := NewAdminIPRateLimiter(5, time.Minute)
registerAuthAndAdminUserRoutes(mux, service, cfg, loginLimiter, adminAuth)

// 변경 후:
var loginLimiter adminLoginRateLimiter
if cfg.redisLoginClient != nil {
    loginLimiter = NewRedisAdminLoginLimiter(cfg.redisLoginClient, 5, time.Minute)
} else {
    loginLimiter = NewAdminIPRateLimiter(5, time.Minute)
}
registerAuthAndAdminUserRoutes(mux, service, cfg, loginLimiter, adminAuth)
```

`registerAuthAndAdminUserRoutes` 시그니처 변경:
```go
// 변경 전
func registerAuthAndAdminUserRoutes(..., loginLimiter *AdminIPRateLimiter, ...)
// 변경 후
func registerAuthAndAdminUserRoutes(..., loginLimiter adminLoginRateLimiter, ...)
```

- [ ] **Step 4: run.go 에서 Redis client 주입**

`internal/app/run.go` 의 `adminRouteOpts` 구성 부분에서 `redisClient != nil` 블록 안에 추가:
```go
if redisClient != nil {
    // (기존 코드)
    if dlqReader, err := eventstream.NewRedisDLQReader(redisClient); err == nil {
        adminRouteOpts = append(adminRouteOpts, httpapi.WithDLQReader(dlqReader))
    }
    // 추가:
    adminRouteOpts = append(adminRouteOpts, httpapi.WithRedisLoginLimiter(redisClient))
}
```

- [ ] **Step 5: 테스트**

```bash
go test ./internal/httpapi/... 2>&1 | grep -E "^(ok|FAIL)"
go test ./internal/app/... 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 6: 커밋**

```bash
git add internal/httpapi/admin_rate_limiter_redis.go internal/httpapi/admin_types.go internal/httpapi/admin.go internal/app/run.go
git commit -m "security: replace in-memory admin login rate limiter with Redis-backed limiter

Falls back to in-memory when Redis is not configured (self-hosted single instance).
Distributed Redis counter prevents bypass via multiple instances or restarts.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Task 8: Next.js CSP nonce — webmail + console (#7)

**Goal:** 양쪽 Next.js 앱에서 `script-src 'unsafe-inline'` 을 제거하고, 요청마다 생성한 nonce 기반 CSP로 교체한다.

**Files:**
- Modify: `apps/webmail/src/middleware.ts`
- Modify: `apps/webmail/src/app/layout.tsx`
- Modify: `apps/webmail/next.config.ts`
- Modify: `apps/console/src/middleware.ts`
- Modify: `apps/console/src/app/layout.tsx`
- Modify: `apps/console/next.config.ts`

**Acceptance Criteria:**
- [ ] webmail의 CSP 응답 헤더가 `'nonce-<base64>'` 를 포함하고 `'unsafe-inline'` 을 포함하지 않음
- [ ] console 동일
- [ ] `layout.tsx` 의 inline `<script>` 에 `nonce` 속성이 부여됨 (webmail; console은 inline script 없으면 불필요)
- [ ] `next build` 가 에러 없이 완료됨 (가능하면 확인)

**Verify:** 로컬 앱 시작 후 `curl -sI http://localhost:3003 | grep content-security-policy` → `nonce-` 포함 확인. 또는 `grep "unsafe-inline" apps/webmail/next.config.ts` → 출력 없음

**Steps:**

- [ ] **Step 1: webmail middleware.ts — nonce 생성 + CSP 헤더**

`apps/webmail/src/middleware.ts` 전체 교체:
```typescript
import { NextResponse, type NextRequest } from 'next/server';

const REQUEST_ID_HEADER = 'x-request-id';
const REQUEST_ID_RESPONSE_HEADER = 'X-Request-ID';
const MAX_REQUEST_ID_LENGTH = 128;
const NONCE_HEADER = 'x-nonce';

export function middleware(req: NextRequest) {
  const requestID =
    sanitizeRequestID(req.headers.get(REQUEST_ID_HEADER)) || crypto.randomUUID();
  const nonce = Buffer.from(crypto.randomUUID()).toString('base64');

  const cspHeader = [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}'`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "connect-src 'self'",
    "font-src 'self' data:",
    "frame-src 'none'",
    "frame-ancestors 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "upgrade-insecure-requests",
  ].join('; ');

  const requestHeaders = new Headers(req.headers);
  requestHeaders.set(REQUEST_ID_HEADER, requestID);
  requestHeaders.set(NONCE_HEADER, nonce);

  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set(REQUEST_ID_RESPONSE_HEADER, requestID);
  response.headers.set('Content-Security-Policy', cspHeader);
  return response;
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
};

function sanitizeRequestID(value: string | null): string {
  const trimmed = (value ?? '').trim();
  if (!trimmed || trimmed.length > MAX_REQUEST_ID_LENGTH) return '';
  if (!/^[A-Za-z0-9._:-]+$/.test(trimmed)) return '';
  return trimmed;
}
```

- [ ] **Step 2: webmail layout.tsx — nonce를 inline script에 적용**

`apps/webmail/src/app/layout.tsx` 에서 `RootLayout` 함수를 수정:
1. `import { headers } from 'next/headers';` 추가
2. 함수 본문 첫 줄에 nonce 읽기 추가:
```typescript
const nonce = (await headers()).get('x-nonce') ?? '';
```
3. 기존 `<script dangerouslySetInnerHTML={...}>` 태그에 `nonce={nonce}` 속성 추가:
```tsx
<script
  nonce={nonce}
  dangerouslySetInnerHTML={{ __html: `...` }}
/>
```
4. `RootLayout` 이 이미 `async` 함수인지 확인; 아니면 `async` 로 변경.

- [ ] **Step 3: webmail next.config.ts — CSP 헤더 섹션 수정**

`apps/webmail/next.config.ts` 에서:
- `scriptSrc` 변수 및 `const isProduction =` 관련 줄 삭제
- `headers()` 내의 `Content-Security-Policy` 항목 삭제 (middleware가 담당)
- `Strict-Transport-Security`, `X-Content-Type-Options` 등 non-CSP 헤더는 유지
- `upgrade-insecure-requests` 는 middleware의 CSP에 이미 포함되므로 config에서 제거

결과적으로 `headers()` 함수는 CSP를 포함하지 않고, 나머지 보안 헤더만 설정.

```typescript
// headers() 반환 배열에서 Content-Security-Policy 항목을 제거한 예시:
{ key: 'X-Content-Type-Options', value: 'nosniff' },
{ key: 'X-Frame-Options', value: 'DENY' },
{ key: 'Cross-Origin-Opener-Policy', value: 'same-origin' },
{ key: 'Cross-Origin-Resource-Policy', value: 'same-origin' },
{ key: 'X-DNS-Prefetch-Control', value: 'off' },
{ key: 'Strict-Transport-Security', value: 'max-age=63072000; includeSubDomains; preload' },
{ key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
{ key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
```

- [ ] **Step 4: console 앱에 동일 패턴 적용**

`apps/console/src/middleware.ts` — 동일하게 nonce 추가 (Step 1 패턴 적용)
`apps/console/src/app/layout.tsx` — inline `<script>` 가 있는지 확인:
```bash
grep -n "dangerouslySetInnerHTML\|<script" apps/console/src/app/layout.tsx
```
있으면 webmail과 동일하게 nonce 적용. 없으면 headers() 추가 불필요.

`apps/console/next.config.ts` — webmail과 동일하게 CSP 항목 제거.

- [ ] **Step 5: unsafe-inline 제거 확인**

```bash
grep -rn "unsafe-inline" apps/webmail/next.config.ts apps/console/next.config.ts
```
출력이 없어야 함 (style-src는 제외 — 의도적으로 유지).

- [ ] **Step 6: 빌드 확인 (가능하면)**

```bash
cd apps/webmail && pnpm build 2>&1 | tail -10
cd ../console && pnpm build 2>&1 | tail -10
```

- [ ] **Step 7: 커밋**

```bash
git add apps/webmail/src/middleware.ts apps/webmail/src/app/layout.tsx apps/webmail/next.config.ts \
        apps/console/src/middleware.ts apps/console/src/app/layout.tsx apps/console/next.config.ts
git commit -m "security: replace CSP unsafe-inline with per-request nonce in Next.js apps

Middleware generates a crypto nonce per request and sets it in the
Content-Security-Policy header. The nonce is passed to layout via
x-nonce request header and applied to inline <script> tags.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Final verification

```bash
go test ./... 2>&1 | grep -E "^(ok|FAIL)"
```
모든 줄이 `ok` 이어야 함.
