# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 🔄 TASK-082: Domain Settings UI

**STATUS: IN_PROGRESS**

### Progress
- ✅ Database migration created (0081_domain_settings.sql)
- ✅ DomainSettings type added to internal/admin/models.go
- ✅ Documentation updated (ACTIVE_TASK.md, CURRENT_STATUS.md)
- ✅ Tests passing (5723 passed)
- ⏳ Next: API routes, service layer implementation

### 제목
Domain Settings UI — Admin Console 도메인 설정 페이지 구현

### 배경
Phase 8-D (UI/UX & Settings)에서 정의한 Domain Settings 기능:
- TLS 정책 (Opportunistic, Require, Disable)
- 사용자당 스토리지 Quota
- IP 화이트리스트
- 2FA 요구 여부
- 세션 타임아웃
- 비밀번호 정책

Admin Console Frontend (TASK-079)는 기본 페이지들(Users, Domains, Audit Logs 등)을 구현했으나,
도메인 단위 상세 설정(domain settings, api settings, alerts) 페이지는 scope 밖.

### 구현 대상

#### 1. 백엔드 API (`internal/httpapi/admin.go`)
- `GET /admin/v1/domains/{id}/settings` — 현재 도메인 설정 조회
- `PUT /admin/v1/domains/{id}/settings` — 도메인 설정 업데이트 (DomainSettingsRequest)
- `POST /admin/v1/domains/{id}/settings/validate` — 설정값 검증 (요청 없이 변경)

#### 2. 서비스 계층 (`internal/admin/admin.go`)
```go
type DomainSettings struct {
  DomainID              string    // e.g., "example.com"
  TLSPolicy             string    // "opportunistic" | "require" | "disable"
  QuotaPerUser          int64     // bytes per user (e.g., 10GB = 10737418240)
  IPWhitelistEnabled    bool
  IPWhitelist           []string  // CIDR 또는 단일 IP
  Require2FA            bool
  SessionTimeoutMinutes int       // 기본값 480 (8시간)
  PasswordPolicy        struct {
    MinLength           int       // 기본값 8
    RequireUppercase    bool
    RequireNumbers      bool
    RequireSpecialChars bool
    ExpiryDays          int       // 0 = 만료 없음
  }
  UpdatedAt             time.Time
  UpdatedBy             string    // admin user ID
}

// Service methods
func (svc *AdminService) GetDomainSettings(ctx context.Context, domainID string) (*DomainSettings, error)
func (svc *AdminService) UpdateDomainSettings(ctx context.Context, domainID string, settings *DomainSettings) error
func (svc *AdminService) ValidateDomainSettings(ctx context.Context, settings *DomainSettings) error
```

#### 3. 데이터베이스
- 새 테이블: `domain_settings` (또는 `domain_config` 확장)
  ```sql
  CREATE TABLE domain_settings (
    domain_id           TEXT PRIMARY KEY REFERENCES companies(domain) ON DELETE CASCADE,
    tls_policy          TEXT NOT NULL DEFAULT 'opportunistic',
    quota_per_user      BIGINT NOT NULL DEFAULT 10737418240,
    ip_whitelist_enabled BOOLEAN NOT NULL DEFAULT false,
    ip_whitelist        TEXT[] DEFAULT '{}', -- CIDR 배열
    require_2fa         BOOLEAN NOT NULL DEFAULT false,
    session_timeout_min INT NOT NULL DEFAULT 480,
    password_min_length INT NOT NULL DEFAULT 8,
    password_require_upper BOOLEAN NOT NULL DEFAULT true,
    password_require_num BOOLEAN NOT NULL DEFAULT true,
    password_require_special BOOLEAN NOT NULL DEFAULT false,
    password_expiry_days INT NOT NULL DEFAULT 0,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by          TEXT NOT NULL REFERENCES admin_users(id),
    CONSTRAINT valid_tls_policy CHECK (tls_policy IN ('opportunistic', 'require', 'disable')),
    CONSTRAINT valid_session_timeout CHECK (session_timeout_min > 0)
  );
  ```

#### 4. 프론트엔드 (`apps/admin/src/`)
- **Page**: `(console)/domains/[id]/settings/page.tsx`
  - 도메인별 설정 폼 (Cloudscape Form 컴포넌트)
  - 섹션별 정리: TLS, Quota, IP Whitelist, 2FA, Session, Password Policy
  - 저장/취소 버튼

- **Hook**: `hooks/useDomainSettings.ts`
  - `useQuery('domainSettings', ...)` — GET 설정
  - `useMutation(updateDomainSettings)` — PUT 업데이트

- **타입**: `types/admin.ts`에 추가
  ```ts
  export interface DomainSettings {
    domainId: string;
    tlsPolicy: 'opportunistic' | 'require' | 'disable';
    quotaPerUser: number;
    // ... 나머지 필드
  }
  ```

- **컴포넌트**:
  - `DomainSettingsForm` — 폼 렌더링
  - `TLSPolicySection` — TLS 라디오 선택
  - `QuotaSection` — 스토리지 입력 (GB 단위)
  - `IPWhitelistSection` — 목록 추가/제거
  - `2FASection` — 토글
  - `SessionTimeoutSection` — 시간 입력
  - `PasswordPolicySection` — 복합 정책 설정

- **권한**: Domain Admin 이상만 접근 가능 (middleware + 클라이언트 권한 확인)

### 완료 조건

- [ ] `go test ./...` 통과
- [ ] GET `/admin/v1/domains/{id}/settings` API 구현 및 테스트
- [ ] PUT `/admin/v1/domains/{id}/settings` API 구현 및 테스트
- [ ] 데이터베이스 마이그레이션 작성
- [ ] DomainSettings 서비스 메서드 구현
- [ ] 프론트엔드 page.tsx 구현 (폼 렌더링)
- [ ] React Query 훅 구현
- [ ] 폼 검증 (Quota > 0, SessionTimeout > 0 등)
- [ ] 에러 처리 (ValidationError, PermissionError)
- [ ] Vitest 단위 테스트 작성
- [ ] Playwright E2E 테스트 작성 (로그인 → 설정 조회 → 수정 → 저장)
- [ ] docs/CURRENT_STATUS.md 갱신
- [ ] docs/backend-roadmap.md TASK-082 체크

### 다음 태스크
TASK-083: API Settings UI

### 즉시 다음 작업 (Next Commit)

1. Add service methods to AdminService interface:
   ```go
   GetDomainSettings(ctx context.Context, domainID string) (*admin.DomainSettings, error)
   UpdateDomainSettings(ctx context.Context, settings *admin.DomainSettings) error
   ```

2. Add API routes to RegisterAdminRoutes in internal/httpapi/admin.go:
   - GET /admin/v1/domains/{id}/settings
   - PUT /admin/v1/domains/{id}/settings

3. Implement handlers in admin.go with proper error handling and validation

4. Write tests in admin_test.go:
   - TestGetDomainSettings (success + not found)
   - TestUpdateDomainSettings (success + validation error)

5. Implement service methods in internal/admin/service.go

6. Run: go test ./... → ensure all pass

7. Add tests for frontend (apps/admin/src/)

8. Implement frontend page: apps/admin/src/app/(console)/domains/[id]/settings/page.tsx

### 루프 절차

```
1. 이 파일 읽기 ✓
2. 서비스 인터페이스 메서드 추가
3. API 핸들러 구현 (GET, PUT)
4. 단위 테스트 작성 및 통과
5. go test ./... 실행 (모두 통과)
6. 프론트엔드 페이지 구현
7. pnpm test (admin) 실행
8. docs 업데이트
9. git add + commit + push
10. 이 파일을 TASK-083으로 교체
```
