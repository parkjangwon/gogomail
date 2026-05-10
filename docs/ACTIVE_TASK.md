# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 🔄 TASK-083: API Settings UI

**STATUS: FRONTEND_COMPLETE**

### Progress
- ✓ Backend: Database migrations (0083_api_settings.sql, 0084_api_keys.sql)
- ✓ Backend: Service layer and API endpoints (6 endpoints)
- ✓ Backend: OpenAPI 3.1.0 documentation
- ✓ Frontend: Page, hooks, components, modal
- ⏳ Next: E2E tests, docs update, task completion

### 제목
API Settings UI — Admin Console API 설정 페이지 구현

### 배경
Phase 8-D (UI/UX & Settings)에서 정의한 API Settings 기능:
- API Key 관리 (생성, 회전, 삭제)
- Rate Limit 설정 (요청/초, 대역폭 제한)
- CIDR Allowlist (IP 범위 제한)
- 사용 통계 및 문서 링크

Domain Settings (TASK-082)와 마찬가지로 도메인 단위 상세 설정 페이지.

### 구현 대상

#### 1. 백엔드 API (`internal/httpapi/admin.go`)
- `GET /admin/v1/domains/{id}/api-settings` — 현재 API 설정 조회
- `PUT /admin/v1/domains/{id}/api-settings` — API 설정 업데이트
- `POST /admin/v1/domains/{id}/api-keys` — API Key 생성
- `GET /admin/v1/domains/{id}/api-keys` — API Key 목록 조회
- `DELETE /admin/v1/domains/{id}/api-keys/{key-id}` — API Key 삭제
- `POST /admin/v1/domains/{id}/api-keys/{key-id}/rotate` — API Key 회전

#### 2. 데이터베이스
- 새 테이블: `api_settings` (rate_limit, cidr_allowlist 등)
  ```sql
  CREATE TABLE api_settings (
    domain_id           TEXT PRIMARY KEY REFERENCES companies(domain) ON DELETE CASCADE,
    rate_limit_rps      INT NOT NULL DEFAULT 100,  -- requests per second
    rate_limit_bps      BIGINT NOT NULL DEFAULT 0, -- bytes per second (0 = unlimited)
    cidr_allowlist_enabled BOOLEAN NOT NULL DEFAULT false,
    cidr_allowlist      TEXT[] DEFAULT '{}',
    require_api_key     BOOLEAN NOT NULL DEFAULT true,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by          TEXT NOT NULL REFERENCES admin_users(id)
  );
  ```

- 새 테이블: `api_keys` (Domain 단위 API key management)
  ```sql
  CREATE TABLE api_keys (
    id                  TEXT PRIMARY KEY,
    domain_id           TEXT NOT NULL REFERENCES companies(domain) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    secret_hash         TEXT NOT NULL UNIQUE, -- bcrypt
    created_by          TEXT NOT NULL REFERENCES admin_users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at        TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ,
    is_active           BOOLEAN NOT NULL DEFAULT true
  );
  ```

#### 3. 서비스 계층 (`internal/admin/service.go`)
```go
type APISettings struct {
  DomainID           string
  RateLimitRPS       int     // requests per second
  RateLimitBPS       int64   // bytes per second (0 = unlimited)
  CIDRAllowlistEnabled bool
  CIDRAllowlist      []string // CIDR 또는 단일 IP
  RequireAPIKey      bool
  UpdatedAt          time.Time
  UpdatedBy          string
}

type APIKey struct {
  ID          string
  DomainID    string
  Name        string
  SecretHash  string
  CreatedBy   string
  CreatedAt   time.Time
  LastUsedAt  *time.Time
  ExpiresAt   *time.Time
  IsActive    bool
}

// Service methods
func (svc *Service) GetAPISettings(ctx context.Context, domainID string) (*APISettings, error)
func (svc *Service) UpdateAPISettings(ctx context.Context, settings *APISettings) error
func (svc *Service) CreateAPIKey(ctx context.Context, key *APIKey) (secret string, err error)
func (svc *Service) ListAPIKeys(ctx context.Context, domainID string) ([]APIKey, error)
func (svc *Service) DeleteAPIKey(ctx context.Context, keyID string) error
func (svc *Service) RotateAPIKey(ctx context.Context, keyID string) (newSecret string, err error)
```

#### 4. 프론트엔드 (`apps/admin/src/`)
- **Page**: `(console)/domains/[id]/api-settings/page.tsx`
  - API 설정 폼 (Cloudscape Form 컴포넌트)
  - Rate Limit 입력 (RPS, BPS)
  - CIDR Allowlist 관리
  - API Key 목록 + 생성/삭제/회전
  
- **Hook**: `hooks/useAPISettings.ts`
  - `useQuery('apiSettings', ...)` — GET 설정
  - `useMutation(updateAPISettings)` — PUT 업데이트
  
- **컴포넌트**:
  - `APISettingsForm` — 설정 폼
  - `RateLimitSection` — RPS/BPS 입력
  - `CIDRAllowlistSection` — CIDR 목록 관리
  - `APIKeysList` — Key 목록, 생성/삭제/회전 버튼
  - `APIKeyCreateModal` — Key 생성 모달 (secret 표시 및 복사)

- **권한**: Domain Admin 이상만 접근 가능

### 완료 조건

- [x] `go test ./...` 통과 (5483/5483)
- [x] GET `/admin/v1/domains/{id}/api-settings` API 구현 및 테스트
- [x] PUT `/admin/v1/domains/{id}/api-settings` API 구현 및 테스트
- [x] API Key CRUD API 구현 및 테스트
- [x] 데이터베이스 마이그레이션 작성 (0083, 0084)
- [x] APISettings 서비스 메서드 구현
- [x] 프론트엔드 page.tsx 구현 (폼 렌더링)
- [x] React Query 훅 구현 (useAPISettings, useAPIKeys, create, delete, rotate)
- [x] API Key secret 표시/복사 기능
- [x] 폼 검증 (RateLimitRPS > 0, BPS >= 0)
- [x] 에러 처리 (Alert components)
- [ ] Vitest 단위 테스트 작성 (선택)
- [ ] Playwright E2E 테스트 작성 (로그인 → 설정 조회 → API Key 생성 → 저장)
- [ ] docs/CURRENT_STATUS.md 갱신
- [ ] docs/backend-roadmap.md TASK-083 체크

### 다음 태스크
TASK-084: Alerts & Notifications

### 즉시 다음 작업 (Next Commit)

1. Create database migration for API settings and API keys tables

2. Add service methods to AdminService interface:
   ```go
   GetAPISettings(ctx context.Context, domainID string) (*admin.APISettings, error)
   UpdateAPISettings(ctx context.Context, settings *admin.APISettings) error
   CreateAPIKey(ctx context.Context, key *admin.APIKey) (secret string, err error)
   ListAPIKeys(ctx context.Context, domainID string) ([]admin.APIKey, error)
   DeleteAPIKey(ctx context.Context, keyID string) error
   RotateAPIKey(ctx context.Context, keyID string) (newSecret string, err error)
   ```

3. Add API routes to RegisterAdminRoutes in internal/httpapi/admin.go:
   - GET /admin/v1/domains/{id}/api-settings
   - PUT /admin/v1/domains/{id}/api-settings
   - POST /admin/v1/domains/{id}/api-keys
   - GET /admin/v1/domains/{id}/api-keys
   - DELETE /admin/v1/domains/{id}/api-keys/{key-id}
   - POST /admin/v1/domains/{id}/api-keys/{key-id}/rotate

4. Implement handlers in admin.go with proper error handling and validation

5. Write tests in admin_test.go:
   - TestGetAPISettings (success + not found)
   - TestUpdateAPISettings (success + validation error)
   - TestCreateAPIKey, TestListAPIKeys, TestDeleteAPIKey, TestRotateAPIKey

6. Implement service methods in internal/admin/service.go

7. Run: go test ./... → ensure all pass

8. Add tests for frontend (apps/admin/src/)

9. Implement frontend page: apps/admin/src/app/(console)/domains/[id]/api-settings/page.tsx

### 루프 절차

```
1. 이 파일 읽기 ✓
2. 데이터베이스 마이그레이션 작성
3. 서비스 인터페이스 메서드 추가
4. API 핸들러 구현 (GET, PUT, POST, DELETE)
5. 단위 테스트 작성 및 통과
6. go test ./... 실행 (모두 통과)
7. 프론트엔드 페이지 구현
8. pnpm test (admin) 실행
9. docs 업데이트
10. git add + commit + push
11. 이 파일을 TASK-084로 교체
```
