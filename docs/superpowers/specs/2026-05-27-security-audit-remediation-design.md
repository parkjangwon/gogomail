# Security Audit Remediation Design
**Date:** 2026-05-27  
**Scope:** All 10 findings from CSO audit

---

## Overview

10개 보안 감사 결과를 3개 영역(Go 백엔드, Helm/Docker, Next.js 프론트엔드)으로 나눠 적용한다.  
기존 외부 API·토큰 형식은 변경하지 않는다.

---

## Section 1 — Go Backend

### #1 GOGOMAIL_ENV 기본값 → `production`
- **파일:** `internal/config/config.go:371`
- `envOrDefault("GOGOMAIL_ENV", "development")` → `envOrDefault("GOGOMAIL_ENV", "production")`
- 개발자는 `GOGOMAIL_ENV=development`를 명시적으로 설정해야 한다.
- 영향: 기존 dev 컨테이너·테스트는 이미 env를 명시하므로 안전. `go test`는 별도 처리 불필요 (테스트 내부에서 `t.Setenv` 사용 중).

### #3 내부 X-Gogomail-* 헤더 ingress에서 제거
- **파일:** `internal/httpapi/admin_middleware.go` + `internal/app/run.go`
- `StripInternalHeadersMiddleware` 함수 추가.
- 제거 대상 헤더:
  - `X-Gogomail-Resolved-User-ID`
  - `X-Gogomail-Tenant-ID`
  - `X-Gogomail-Company-ID`
  - `X-Gogomail-Domain-ID`
  - `X-Gogomail-Principal-ID`
  - `X-Gogomail-API-Key-ID`
- `run.go`의 최외곽 핸들러 체인에 RequestIDMiddleware 다음에 삽입.

### #4 Redis 기반 admin 로그인 rate limiter
- **파일:** `internal/httpapi/admin_rate_limiter_redis.go` (신규)
- 인터페이스: `AdminRateLimiter` (기존 `AdminIPRateLimiter`와 동일한 `Middleware` 메서드)
- Redis 구현: `INCR key` + `EXPIRE key window` (atomic하지 않으므로 Lua 스크립트 또는 pipeline 사용)
- fallback: Redis nil/error 시 in-memory로 폴백, 경고 로그.
- `registerAdminUtilityRoutes`에 redis client 주입 경로 추가 (`AdminRouteOption`).

### #5 JWT 내부 구현 → golang-jwt/jwt/v5
- **파일:** `internal/auth/jwt.go` (전면 교체)
- `Claims` 구조체·`TokenManager` 구조체·`NewTokenManager`·`VerifyFull`·`Sign`·`Verify` 시그니처 유지.
- 내부 `sign()`·`Verify()` 로직을 `jwt.NewWithClaims(jwt.SigningMethodHS256, ...)` + `jwt.ParseWithClaims(...)` 로 교체.
- 토큰 포맷(Base64URL HS256)은 동일하여 기존 발급 토큰 무효화 없음.
- `password_go125.go`와 관련 없는 파일은 손대지 않음.
- 기존 테스트 파일(`jwt_test.go`, `_extra_test.go` 포함) 전부 통과해야 함.

### #6 레거시 패스워드 해시 로그인 시 자동 업그레이드
- **파일:** `internal/auth/password.go`
- 기존 `VerifyPasswordHash(password, encoded string) bool` 시그니처 유지 (호환성).
- 신규 함수 `VerifyPasswordHashResult(password, encoded string) (verified, needsUpgrade bool)` 추가:
  - `needsUpgrade = true` 조건: `plain:` 또는 `sha256:` prefix로 검증 성공한 경우.
- 로그인 핸들러에서 `VerifyPasswordHashResult` 사용:
  - `needsUpgrade == true`이면 pbkdf2-sha256으로 재해시 후 DB 업데이트 (비동기 가능).
  - 업그레이드 성공/실패 모두 로그 기록 (해시 값 미포함).
- 기존 `VerifyPasswordHash` 내부를 `VerifyPasswordHashResult` 를 호출하도록 위임.
- 테스트: 레거시 해시로 로그인 시 `needsUpgrade == true` 반환 검증.

### #8 RDBMS identity provider SQL allowlist
- **파일:** `internal/idprovider/rdbms/provider.go` (신규 함수 추가)
- `validateSourceQuery(query string) error` 함수:
  - 공백 제거 후 대소문자 무관하게 `SELECT`로 시작해야 함.
  - 금지 키워드(word-boundary 정규식): `UNION`, `INSERT`, `UPDATE`, `DELETE`, `DROP`, `TRUNCATE`, `CREATE`, `ALTER`, `EXEC`, `EXECUTE`, `GRANT`, `REVOKE`.
  - 세미콜론은 문자열 끝(trailing)에만 허용.
  - 길이 제한: 4096자 이하.
- `Config` 유효성 검사 시(`Connect()` 또는 별도 `Validate()`) 호출.

### #10 APNS private key 파일 경로 옵션
- **파일:** `internal/config/config.go`, `internal/config/config_file.go`, `docker/.env.example`
- `GOGOMAIL_APNS_PRIVATE_KEY_FILE` 환경변수 추가.
- 로딩 우선순위: file path > env var (하위 호환).
- 파일 미존재 시 명확한 에러 메시지.

---

## Section 2 — Helm / Docker

### #2 Helm CHANGEME 가드
- **파일:** `helm/gogomail/templates/_helpers.tpl`, `helm/gogomail/templates/secret.yaml`
- `_helpers.tpl`에 `gogomail.requireNotChangeme` 헬퍼 추가:
  ```
  {{- define "gogomail.requireNotChangeme" -}}
  {{- if or (contains "CHANGEME" .) (eq . "") -}}
  {{- fail (printf "Secret value must be set: %s" .) -}}
  {{- end -}}
  {{- end -}}
  ```
- `secret.yaml`에서 `GOGOMAIL_DM_MASTER_KEY`, `GOGOMAIL_AUTH_JWT_SECRET`, `GOGOMAIL_ADMIN_TOKEN` 각각에 적용.

### #9 docker-compose.scale.yml sslmode 기본값
- **파일:** `docker/docker-compose.scale.yml:27`
- `sslmode=disable` → `sslmode=require`

---

## Section 3 — Next.js CSP Nonce (#7)

**webmail + console 동일 패턴 적용.**

### middleware.ts
```typescript
// apps/webmail/src/middleware.ts (신규 또는 갱신)
import { NextRequest, NextResponse } from 'next/server'

export function middleware(request: NextRequest) {
  const nonce = Buffer.from(crypto.randomUUID()).toString('base64')
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
  ].join('; ')

  const response = NextResponse.next({
    request: { headers: new Headers(request.headers) },
  })
  response.headers.set('Content-Security-Policy', cspHeader)
  response.headers.set('x-nonce', nonce)
  return response
}

export const config = { matcher: '/((?!_next/static|_next/image|favicon.ico).*)' }
```

### layout.tsx
- `import { headers } from 'next/headers'`
- `const nonce = (await headers()).get('x-nonce') ?? ''`
- `<Script nonce={nonce} ...>` 형식으로 nonce 전달.

### next.config.ts
- `headers()` async 함수 제거 (middleware가 CSP 담당).
- `X-Content-Type-Options` 등 non-CSP 헤더는 config에 유지.

---

## Testing

- `go test ./...` 전체 통과 필수.
- JWT 마이그레이션: 기존 `jwt_test.go` + `_extra_test.go` 모두 통과.
- 패스워드 업그레이드: `VerifyPasswordHash` 호출부 테스트 추가.
- SQL allowlist: `validateSourceQuery` 단위 테스트 추가.
- 헤더 스트리핑: 미들웨어 단위 테스트 추가.

---

## Non-Goals

- 기존 발급 JWT 무효화하지 않음.
- RDBMS provider에 완전한 SQL 파서 도입하지 않음 (keyword blocklist로 충분).
- CSP nonce를 WebDAV 엔드포인트에 적용하지 않음.
