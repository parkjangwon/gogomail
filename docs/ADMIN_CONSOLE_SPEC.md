# gogomail Admin Console — Comprehensive Specification

**Version**: 1.0  
**Last Updated**: 2026-05-10  
**Phase**: 8 (Admin Console & Enterprise Features)

---

## 목차

1. [개요](#개요)
2. [아키텍처](#아키텍처)
3. [관리자 계층 구조](#관리자-계층-구조)
4. [권한 모델](#권한-모델)
5. [Identity 관리](#identity-관리)
6. [주요 기능](#주요-기능)
7. [데이터 모델](#데이터-모델)
8. [API 스펙](#api-스펙)
9. [UI/UX 원칙](#uiux-원칙)
10. [감시 & 규정](#감시--규정)
11. [구현 로드맵](#구현-로드맵)

---

## 개요

### 목표

gogomail Admin Console은 **엔터프라이즈급 멀티테넌시 SaaS**와 **온프레미스 자체호스팅** 모두를 지원하는 고급 관리자 인터페이스입니다.

- **SaaS 모드**: gogomail.com 호스팅, 다중 회사 관리, 시스템 관리자 필요
- **On-Premises 모드**: 자체 호스팅, 단일/소수 회사, 로컬 IT팀이 관리

### 핵심 특성

```
✅ 다중 테넌시 (Enterprise multi-tenant)
✅ 세밀한 RBAC (Role-Based Access Control)
✅ 동적 역할 커스터마이징 (고정 역할 X)
✅ Identity 추상화 (DB, LDAP, Azure AD, External RDBMS)
✅ 고급 모니터링 & 분석 (Mail logs, Security, API metering)
✅ 완전한 감시 & 규정 준수 (Audit logs, Compliance levels)
✅ AWS Console 스타일 UI (정보 밀도, 다크 테마, 강력한 제어)
```

---

## 아키텍처

### 시스템 구조

```
┌─────────────────────────────────────────────────────┐
│           Admin Console (Frontend)                  │
│    (Notion Mail-inspired User UI, AWS-like Console)│
└────────────┬────────────────────────────────────────┘
             │
             │ HTTP API (내부, Admin Token)
             ↓
┌─────────────────────────────────────────────────────┐
│      Admin Console Backend Service                  │
│  ├─ Auth & Session Management                      │
│  ├─ User/Organization CRUD                         │
│  ├─ Identity Provider Abstraction                  │
│  ├─ Audit & Compliance                             │
│  └─ Statistics & Analytics                         │
└────────────┬────────────────────────────────────────┘
             │
    ┌────────┼──────────┬──────────┐
    ↓        ↓          ↓          ↓
   DB      LDAP       Azure       External
(PostgreSQL) Sync      AD         RDBMS
   Users   (선택)  (선택)       (선택)
   Orgs
```

### 배포 모드

| 모드 | 설정 | 특징 |
|------|------|------|
| SaaS | GOGOMAIL_MODE=saas | System Admin (staff only), Multi-company, 청구 통합 |
| On-Prem | GOGOMAIL_MODE=on-premises | System Admin (customer IT), 단일 회사, 모든 기능 동일 |

### 환경 설정

```bash
# 모드 설정
GOGOMAIL_ADMIN_MODE=saas              # saas | on-premises
GOGOMAIL_ADMIN_CONSOLE_ENABLED=true   # 콘솔 활성화

# 암호화 키
GOGOMAIL_ENCRYPTION_KEY=<base64>      # LDAP password, HR DB password 암호화

# 토큰
GOGOMAIL_ADMIN_INITIAL_TOKEN=xyz...   # 초기 관리자 token (setup 후 제거)
```

---

## 관리자 계층 구조

### SaaS 모드

```
┌─────────────────────────────────────────┐
│  System Admin (gogomail.com 스태프)      │
│  • 모든 회사/도메인 관리                │
│  • 시스템 설정, 라이선스, 백업           │
│  • 전체 감시 로그 접근                  │
└─────────────────────────────────────────┘
          │
          ├─→ Company 1
          │   └─→ Domain Admin (회사 관리자)
          │       ├─→ Security Officer
          │       ├─→ HR Officer
          │       ├─→ Monitoring Officer
          │       ├─→ Auditor
          │       └─→ Support Specialist
          │           (+ Custom Roles)
          │
          └─→ Company 2
              └─→ [같은 구조]
```

### On-Premises 모드

```
┌─────────────────────────────────────────┐
│  System Admin (고객사 IT팀)              │
│  • 모든 설정 접근                       │
│  • 사용자/도메인 관리                   │
└─────────────────────────────────────────┘
          │
          └─→ Single/Few Companies
              └─→ Domain Admin, Security Officer, ...
```

### 내장 역할 (Built-in, 변경 불가)

| 역할 | 설명 | 권한 범위 |
|------|------|---------|
| System Admin | 시스템 관리자 | 모든 회사 |
| Domain Admin | 도메인 관리자 | 자신 회사/도메인 |
| Security Officer | 보안 담당자 | 보안 설정, 로그 (읽기) |
| HR Officer | HR 담당자 | 사용자 관리, 조직도 |
| Monitoring Officer | 모니터링 요원 | 모니터링 데이터 (읽기) |
| Auditor | 감사자 | 감시 로그, 내보내기 (읽기) |
| Support Specialist | 기술 지원담당자 | 사용자 지원 (읽기) |

### 커스텀 역할

Domain Admin이나 System Admin이 추가 역할을 생성 가능:

```
예: "Finance Officer", "Legal Counsel", "Compliance Manager" 등
각 역할마다 세밀한 권한 설정 가능
```

---

## 권한 모델

### Permission Matrix

#### Resource: User Management

| 액션 | System Admin | Domain Admin | Security Officer | HR Officer | Monitoring Officer | Auditor |
|------|-------------|--------------|------------------|------------|-------------------|---------|
| Create | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ |
| Read | ✅ all | ✅ own | ✅ own | ✅ own | ❌ | ❌ |
| Update | ✅ | ✅ (own) | 제한 | ✅ | ❌ | ❌ |
| Delete | ✅ | ✅ (soft) | ❌ | ✅ (soft) | ❌ | ❌ |
| Reset Password | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ |
| Assign Role | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |

#### Resource: Domain Settings

| 액션 | System Admin | Domain Admin | Security Officer | Others |
|------|-------------|--------------|------------------|--------|
| Read | ✅ all | ✅ own | ✅ own | ✅ |
| Update | ✅ | ✅ | 제한 | ❌ |
| Configure TLS | ✅ | ✅ | ✅ | ❌ |
| Configure DKIM | ✅ | ✅ | ✅ | ❌ |
| Configure Quota | ✅ | ✅ | ❌ | ❌ |

#### Resource: Logs & Monitoring

| 액션 | System Admin | Domain Admin | Security Officer | Monitoring Officer | Auditor |
|------|-------------|--------------|------------------|-------------------|---------|
| Mail Logs | ✅ all | ✅ own | ✅ own | ✅ own | ❌ |
| Login Logs | ✅ all | ✅ own | ✅ own | ❌ | ✅ own |
| Audit Logs | ✅ all | ✅ own | ✅ own | ❌ | ✅ own |
| Spam Logs | ✅ all | ✅ own | ✅ own | ✅ own | ❌ |
| Export | ✅ | ✅ | ❌ | ❌ | ✅ |

---

## Identity 관리

### Identity Provider 추상화

Admin Console은 사용자/조직도를 여러 소스에서 관리할 수 있습니다:

```
┌──────────────────────────────────────────────┐
│   Admin Console (통일된 UI)                   │
└────────────────┬─────────────────────────────┘
                 │
                 ├─→ Mode 1: Database Only
                 │   (gogomail 자체 DB만 사용)
                 │
                 ├─→ Mode 2: LDAP/Active Directory
                 │   (외부 LDAP과 동기화)
                 │
                 ├─→ Mode 3: Azure AD
                 │   (Microsoft AD 연동)
                 │
                 └─→ Mode 4: External RDBMS
                     (HR 시스템의 독자 DB)
```

### Mode 1: Database Only (기본)

```
특징:
✅ 별도 시스템 없이 gogomail DB만 사용
✅ 관리자가 웹 UI에서 직접 생성/수정
✅ 빠른 초기 설정
✅ 소규모 회사에 적합

사용자/조직 관리:
- [+ New User] 버튼으로 생성
- 모든 필드 편집 가능
- 사용자 삭제 가능
```

### Mode 2: LDAP/Active Directory 동기화

```
특징:
✅ 기존 LDAP/AD와 동기화
✅ 사용자/조직도를 외부에서 관리
✅ gogomail은 읽기 전용 (선택적 프로비저닝)

설정:
- LDAP 서버 URL, 포트
- Bind DN, 비밀번호
- User Search Base/Filter
- Organization Search Base/Filter
- 속성 매핑 (LDAP attr → gogomail field)

동기화:
- 스케줄: 수동 또는 자동 (cron)
- LDAP에만 있는 사용자: 자동 생성 (선택)
- LDAP에서 제거된 사용자: 자동 비활성화 (선택)
```

### Mode 3: Azure AD 연동

```
특징:
✅ Microsoft Azure AD와 동기화
✅ Office 365 사용자와 일관성
✅ SSO 지원

설정:
- Tenant ID, Client ID, Client Secret
- User/Group 동기화 범위
- 속성 매핑

제약:
- gogomail에서 사용자 직접 생성 불가
- Azure Portal에서만 수정
```

### Mode 4: External RDBMS 동기화

```
특징:
✅ HR 시스템이 자신의 DB 테이블 구조를 가진 경우
✅ 커스텀 SQL 쿼리로 연결
✅ 유연한 매핑

설정:
- 외부 DB 연결 정보 (host, port, user, password)
- User Query: SELECT 문
- Organization Query: SELECT 문 (선택)
- 필드 매핑: 외부 DB 컬럼 → gogomail 필드
- Unique Key: 사용자 비교 기준 (email, emp_id 등)

예:
SELECT email, full_name, department_id FROM employees 
WHERE status='active'

Mapping:
- email → email
- full_name → name
- department_id → organization_id
```

### Mode 전환

Domain Admin이 언제든 Mode 변경 가능:

```
Database → LDAP:
- 기존 사용자 유지
- LDAP 동기화 시작

LDAP → Database:
- 동기화 중단
- 기존 데이터 "고정" (더 이상 LDAP과 동기화 안됨)

위험: 전환 전 미리보기 필수
```

---

## 주요 기능

### 1. Dashboard

#### System Admin 대시보드
```
- 전체 회사 수, 사용자 수, 메일량 통계
- 실시간 메일 트래픽 (inbound/outbound)
- 주간/월간 트렌드
- 시스템 상태 (DB, Redis, Storage)
- 주요 알림 (quota 초과, 에러율, 스팸율)
- 상위 회사별 API 사용량
```

#### Domain Admin 대시보드
```
- 자신 회사: 사용자 수, 메일량, 스토리지 사용
- 주간 메일 트렌드 (송/수신)
- 활동 사용자 순위 (top 10)
- 스팸 차단율
- 최근 로그인 시도 (실패)
- API 사용량 및 Rate Limit 상태
- 저장소 사용률 + 예상 소진 날짜
```

### 2. User Management

```
페이지:
├─ 사용자 목록 (검색, 필터, 페이지네이션)
├─ 사용자 상세 (이메일, 상태, 2FA, 마지막 접속)
├─ 사용자 생성/편집/삭제
├─ 일괄 import (CSV)
├─ 권한 위임 (역할 부여)
└─ 비밀번호 초기화

필드:
- 이메일, 이름
- 상태 (active/suspended/deactivated)
- 2FA 활성화 여부
- 마지막 로그인
- 스토리지 사용량
- 할당된 역할
```

### 3. Organization Management

```
페이지:
├─ 조직 트리 (부서/팀 계층)
├─ 부서 생성/편집/삭제
├─ 부서별 사용자 할당
├─ LDAP/HR DB 동기화 설정
├─ 동기화 이력 및 상태
└─ 조직 내보내기 (구조 다이어그램)

기능:
- 드래그앤드롭으로 부서 이동
- 부서별 통계 (사용자 수, 메일량)
- 관리자 위임 (부서별 관리자)
```

### 4. Identity & Directory

```
페이지:
├─ Identity Mode 선택 (Database/LDAP/Azure/RDBMS)
├─ LDAP 설정 (server, bind, mapping)
├─ Azure AD 설정 (tenant, client)
├─ External RDBMS 설정 (connection, query, mapping)
├─ 동기화 스케줄
└─ 동기화 상태/로그

기능:
- 연결 테스트 (Test Connection)
- 쿼리 검증 (SQL validation, column preview)
- 동기화 실행 (수동 또는 자동)
- 동기화 이력 조회
- 실패한 행 상세 로그
```

### 5. Mail Monitoring

```
메일 로그:
├─ 발신 로그 (from, to[], subject, timestamp, status)
├─ 수신 로그 (from, to, subject, timestamp, size)
├─ 검색 (sender, recipient, subject, date range)
└─ 상세 조회 (raw headers, Authentication-Results)

스팸 필터 모니터링 (필터 활성화시):
├─ 일일 스팸 차단율
├─ 상위 스팸 규칙
├─ False positive 신고 기능
└─ 격리 메시지 복구
```

### 6. Login & Security Logs

```
로그인 이력:
├─ 사용자, 시간, IP, 디바이스
├─ 성공/실패, 실패 원인
├─ 의심 활동 감지 (unusual location, time)
└─ 2FA 사용 여부

보안 이벤트:
├─ 실패 로그인 (반복)
├─ 비정상 위치 로그인
├─ API Rate Limit 초과
├─ 비정상 API 호출
└─ 권한 변경
```

### 7. Statistics & Analytics

```
메일 통계:
├─ 일/주/월 메일 송수신량
├─ 활동 사용자 수 (DAU, WAU, MAU)
├─ 평균 메일 크기, 첨부파일 비율
└─ 사용자별 메일량 (상위 50명)

스토리지 & Quota:
├─ 전체 사용 스토리지 / 할당량
├─ 사용자별 스토리지 분포
├─ 대용량 메일 (> 5MB)
├─ 1년 이상 된 메일 크기
└─ 예상 소진 날짜

사용자 활동:
├─ 활동도별 사용자 분포 (매우활동/보통/비활동)
├─ 사용자별 메일량 분포
├─ 마지막 활동 시간
└─ 로그인 빈도

API 메터링:
├─ 총 호출 수 (월별)
├─ 일별 호출 수 (추이)
├─ 엔드포인트별 상위 10개
├─ 오류율 (4xx, 5xx)
└─ 평균 응답시간

내보내기:
├─ CSV (모든 통계)
├─ PDF 리포트 (월간 요약, 차트)
└─ NDJSON (대용량 로그 분석용)
```

### 8. Domain & API Settings

```
도메인 설정:
├─ DNS 설정 (DKIM, SPF, DMARC 검증 상태)
├─ TLS 정책 (require/opportunistic/disable)
├─ Quotas (사용자당 스토리지, 메일 보관 기한)
├─ 보안 정책 (IP 화이트리스트, 2FA 요구, 세션 관리)
└─ 세션 타임아웃, 비밀번호 정책

API 설정:
├─ API Key 관리 (생성, 회전, 삭제)
├─ Rate Limit 설정 (기본값, 도메인별 오버라이드)
├─ CIDR Allowlist (특정 IP만 허용)
└─ API 문서 (OpenAPI 링크)

API 미터링 (Domain Admin 보기):
├─ 이번 달 호출량
├─ 엔드포인트별 사용량
├─ 일별 추이 그래프
└─ Rate Limit 현재 상태
```

### 9. Role Management

```
내장 역할:
├─ System Admin, Domain Admin, Security Officer, ...
└─ 수정 불가 (읽기만)

커스텀 역할 (Domain Admin 생성):
├─ 역할 이름, 설명
├─ 권한 체크박스 (resource × action × scope)
├─ 조건 설정 (선택, 예: 특정 부서만)
└─ 사용자에게 부여

권한 위임:
├─ 관리자 → 다른 사용자에게 역할 부여
├─ 임시 위임 (30일 후 자동 해제, 선택)
└─ 위임 기간 설정
```

### 10. Audit Logs

```
감시 로그 조회:
├─ 필터 (기간, 사용자, 액션, 상태)
├─ Admin 행위 (사용자 생성/삭제, 정책 변경)
├─ 보안 이벤트 (로그인 실패, 특이 활동)
├─ 사용자 행위 (Level 3 활성화시, mail read/delete 등)
└─ 감시 로그 레벨 설정 (Domain Admin)

상세 조회:
├─ 변경 내용 (before/after)
├─ IP 주소, 지역, 디바이스
├─ 타임스탬프
└─ 상태 (success/failed)

내보내기:
├─ CSV/JSON
├─ 기간 선택
└─ 필터 적용된 결과만
```

---

## 데이터 모델

### 핵심 테이블

```sql
-- 관리자 역할 정의
CREATE TABLE admin_role_definitions (
  id UUID PRIMARY KEY,
  company_id UUID NOT NULL,
  name TEXT NOT NULL,              -- "Senior Support", "Finance Officer"
  description TEXT,
  is_builtin BOOLEAN,              -- true = 내장 (수정 불가)
  created_by UUID,
  created_at TIMESTAMP,
  UNIQUE(company_id, name)
);

-- 역할별 권한
CREATE TABLE admin_role_permissions (
  id UUID PRIMARY KEY,
  role_definition_id UUID REFERENCES admin_role_definitions(id),
  resource TEXT,                   -- 'users', 'domains', 'logs'
  action TEXT,                     -- 'create', 'read', 'update', 'delete'
  scope TEXT,                      -- 'own_company', 'own_domain', 'all'
  conditions JSONB                 -- {can_reset_password: true}
);

-- 사용자-역할 할당
CREATE TABLE admin_user_roles (
  id UUID PRIMARY KEY,
  company_id UUID NOT NULL,
  user_id UUID NOT NULL,
  role_definition_id UUID REFERENCES admin_role_definitions(id),
  assigned_at TIMESTAMP,
  assigned_by UUID,
  UNIQUE(company_id, user_id, role_definition_id)
);

-- 감시 로그
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY,
  company_id UUID NOT NULL,
  admin_user_id UUID NOT NULL,
  action TEXT,                     -- 'user.create', 'domain.update'
  resource_type TEXT,              -- 'user', 'domain', 'organization'
  resource_id UUID,
  changes JSONB,                   -- {before: {...}, after: {...}}
  ip_address INET,
  user_agent TEXT,
  timestamp TIMESTAMP NOT NULL,
  INDEX (company_id, timestamp)
);

-- 로그인 감시
CREATE TABLE login_audit_logs (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL,
  company_id UUID NOT NULL,
  ip_address INET,
  user_agent TEXT,
  success BOOLEAN,
  failure_reason TEXT,
  timestamp TIMESTAMP NOT NULL,
  INDEX (user_id, timestamp)
);

-- 감시 정책 설정
CREATE TABLE audit_policy_configs (
  id UUID PRIMARY KEY,
  company_id UUID NOT NULL,
  domain_id UUID NOT NULL,
  audit_level TEXT DEFAULT 'level_1',   -- level_1, level_2, level_3
  audit_admin_actions BOOLEAN DEFAULT true,
  audit_security_events BOOLEAN DEFAULT true,
  audit_user_mail_actions BOOLEAN DEFAULT false,
  retention_days INT DEFAULT 90,
  mask_mail_content BOOLEAN DEFAULT true,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  UNIQUE(company_id, domain_id)
);

-- Identity 설정
CREATE TABLE domain_identity_config (
  id UUID PRIMARY KEY,
  domain_id UUID NOT NULL UNIQUE,
  company_id UUID NOT NULL,
  identity_mode TEXT NOT NULL DEFAULT 'database',
  config JSONB,                    -- 모드별 설정
  sync_enabled BOOLEAN DEFAULT false,
  sync_schedule TEXT,
  last_sync_at TIMESTAMP,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

-- LDAP 설정
CREATE TABLE ldap_sync_configs (
  id UUID PRIMARY KEY,
  company_id UUID NOT NULL,
  domain_id UUID NOT NULL,
  server_url TEXT NOT NULL,
  server_port INT DEFAULT 389,
  use_tls BOOLEAN DEFAULT false,
  bind_dn TEXT,
  bind_password_encrypted TEXT,
  user_search_base TEXT,
  user_search_filter TEXT,
  attribute_mapping JSONB,
  sync_enabled BOOLEAN DEFAULT true,
  sync_schedule TEXT,
  last_sync_at TIMESTAMP,
  last_sync_status TEXT,
  created_at TIMESTAMP,
  UNIQUE(company_id, domain_id)
);

-- External RDBMS 설정
CREATE TABLE external_rdbms_configs (
  id UUID PRIMARY KEY,
  company_id UUID NOT NULL,
  domain_id UUID NOT NULL,
  name TEXT,
  db_type TEXT NOT NULL,           -- 'postgresql', 'mysql', 'oracle'
  db_host TEXT NOT NULL,
  db_port INT,
  db_name TEXT NOT NULL,
  db_user TEXT NOT NULL,
  db_password_encrypted TEXT,
  user_query TEXT NOT NULL,
  user_mapping JSONB NOT NULL,
  org_query TEXT,
  org_mapping JSONB,
  sync_enabled BOOLEAN DEFAULT true,
  sync_schedule TEXT,
  last_sync_at TIMESTAMP,
  last_sync_status TEXT,
  created_at TIMESTAMP,
  UNIQUE(company_id, domain_id)
);

-- API 사용량
CREATE TABLE api_usage_daily (
  id UUID PRIMARY KEY,
  company_id UUID NOT NULL,
  date DATE NOT NULL,
  endpoint TEXT,
  call_count BIGINT,
  error_count BIGINT,
  avg_latency_ms FLOAT,
  UNIQUE(company_id, date, endpoint)
);

-- 통계 캐시 (대시보드)
CREATE TABLE admin_stats_cache (
  id UUID PRIMARY KEY,
  company_id UUID NOT NULL,
  stat_type TEXT,                  -- 'daily_mail_count', 'user_count'
  stat_date DATE,
  value JSONB,
  calculated_at TIMESTAMP,
  UNIQUE(company_id, stat_type, stat_date)
);
```

---

## API 스펙

### 인증 & 세션

```
POST   /admin/v1/auth/login
       Body: {email, password}
       Response: {token, expires_in, user}

POST   /admin/v1/auth/logout
GET    /admin/v1/auth/me             (현재 관리자 정보)
POST   /admin/v1/auth/refresh-token
```

### User Management

```
GET    /admin/v1/users              (list, 검색/필터)
POST   /admin/v1/users              (create)
GET    /admin/v1/users/{id}         (detail)
PUT    /admin/v1/users/{id}         (update)
DELETE /admin/v1/users/{id}         (soft delete)
POST   /admin/v1/users/{id}/reset-password
POST   /admin/v1/users/bulk-import  (CSV)
```

### Organization

```
GET    /admin/v1/organization/units
POST   /admin/v1/organization/units
GET    /admin/v1/organization/hierarchy
PUT    /admin/v1/organization/units/{id}
DELETE /admin/v1/organization/units/{id}
POST   /admin/v1/organization/members
DELETE /admin/v1/organization/members/{id}
```

### Identity & Directory

```
GET    /admin/v1/identity-config
PUT    /admin/v1/identity-config    (mode, config)

POST   /admin/v1/ldap-config
POST   /admin/v1/ldap-config/test-connection
POST   /admin/v1/ldap-config/{id}/sync-now
GET    /admin/v1/ldap-config/{id}/sync-status

POST   /admin/v1/external-rdbms-config
POST   /admin/v1/external-rdbms-config/validate-query
POST   /admin/v1/external-rdbms-config/{id}/sync-now
```

### Logs & Monitoring

```
GET    /admin/v1/logs/mail          (메일 로그)
GET    /admin/v1/logs/login         (로그인 로그)
GET    /admin/v1/logs/audit         (감시 로그)
GET    /admin/v1/logs/spam          (스팸 로그)

GET    /admin/v1/stats/dashboard    (대시보드)
GET    /admin/v1/stats/mail-volume
GET    /admin/v1/stats/users
GET    /admin/v1/stats/storage
GET    /admin/v1/api-usage
GET    /admin/v1/api-usage/daily
```

### Settings

```
GET    /admin/v1/companies/{id}/security/audit-policy
PUT    /admin/v1/companies/{id}/security/audit-policy  (audit level, retention, masking)

GET    /admin/v1/domain-settings
PUT    /admin/v1/domain-settings    (TLS, Quota, Security)

GET    /admin/v1/api-settings
POST   /admin/v1/api-keys            (생성, 회전, 삭제)
```

### Roles & Permissions

```
GET    /admin/v1/roles              (all roles)
POST   /admin/v1/roles              (create custom)
PUT    /admin/v1/roles/{id}         (update custom)
DELETE /admin/v1/roles/{id}         (delete custom)

POST   /admin/v1/users/{id}/roles/{role_id}  (assign)
DELETE /admin/v1/users/{id}/roles/{role_id}  (revoke)
```

---

## UI/UX 원칙

### 디자인 철학

**사용자 메일 (Webmail):**
- Notion Mail 영감
- 깔끔, 미니멀, 많은 여백
- 밝은 배경, 부드러운 색상
- 포커스: 메일 내용

**Admin Console:**
- AWS Console 스타일
- 조밀, 정보 최대화
- 다크 배경 (다크 테마 우선)
- 고대비 (#0F1419 배경, #E1E6ED 텍스트)
- 포커스: 데이터 제어

### 색상 팔레트

```
Primary:
- Background: #0F1419 (deep navy)
- Text: #E1E6ED (light gray)
- Accent: #FF9900 (AWS orange) or #0078D4 (Azure blue)

Status:
- Success: #10B981 (green)
- Warning: #F59E0B (amber)
- Error: #EF4444 (red)
- Info: #3B82F6 (blue)
```

### 레이아웃

```
┌─────────────────────────────────────────┐
│ Logo | Dashboard | Users | Org | Logs   │  ← Top Nav (최소화)
├──────────────────────────────────────────┤
│         │                                │
│  Side  │          Main Content          │
│  Nav   │         (Compact, Dense)       │
│ (220px)│                                │
│ (dark) │                                │
└───────────────────────────────────────────┘

Sidebar:
- 접이식 (collapse icon)
- 다크 배경, 밝은 텍스트
- 섹션 구분 (Main, Resources, Settings)

Main Content:
- 테이블: 컴팩트 (row height: 32px)
- 여백: 최소화
- 한 페이지에 더 많은 데이터
- 정렬, 필터, 검색 모두 가능
```

### 인터랙션

**테이블:**
- 정렬 (클릭 헤더)
- 필터 (필터 팔레트)
- 검색 (상단 검색 바)
- 다중 선택 (체크박스)
- 우클릭 컨텍스트 메뉴
- 인라인 편집 (더블클릭)
- 열 재정렬 (드래그)

**Form:**
- 인라인 폼 (가능하면)
- Modal은 최소화 (side panel 선호)
- 저장/취소 always visible

**차트:**
- 호버 시 상세 정보 (툴팁)
- 범위 선택 가능
- 축소/확대 (zoom)

**접근성:**
- 키보드 네비게이션 (Tab, Arrow keys)
- 스크린 리더 지원
- 높은 대비

---

## 감시 & 규정

### Audit Levels

#### Level 1: Admin Actions (기본)
```
기록 대상:
- User CRUD, Password Reset
- Organization Changes
- Domain Settings Changes
- LDAP/HR DB Config Changes
- Role Management

데이터: timestamp, admin_id, action, resource, changes, ip, user_agent
```

#### Level 2: + Security Events (권장)
```
Level 1 + 다음:
- Login (success/failure)
- Failed Login > 5 times/hour
- Login from unusual location
- API Rate Limit Exceeded
- API Authentication Failure

권장: 대부분 회사에 적합
```

#### Level 3: + User Actions (규제 필수시)
```
Level 2 + 다음:
- Mail Read/Delete/Move
- Attachment Download
- Share Link Creation
- Data Export
- API Calls (all)

주의:
- 프라이버시 정책 필수
- 스토리지 비용 매우 증가
- 규정 필요시만 (금융, 의료, 정부)
```

### Data Retention

```
최근 30일:  온라인 (빠른 조회)
30-90일:    압축 아카이브
90일+:      자동 삭제 (또는 cold storage)

설정: Domain Admin이 retention_days 조정 가능 (기본 90일)
```

### Log Masking

```
Level 3에서:
- Mail content: 저장 안함 (메타데이터만)
- Recipient emails: 마스킹 가능 (user@***)
- API request body: 민감 정보 제외
```

---

## 구현 로드맵

### Phase 8-A: Core Admin Console (2-3주)

```
TASK-063: Admin Console Architecture & Specification ✅
TASK-064: Schema + RBAC 모델 + Custom Roles
TASK-065: Auth & Session (로그인, JWT)
TASK-066: User CRUD + Role Assignment
TASK-067: Organization Management
TASK-068: Audit Logs (Level 1 + 2)
```

### Phase 8-B: Identity & Monitoring (2-3주)

```
TASK-069: Identity Provider Abstraction
TASK-070: Database Mode (기본)
TASK-071: LDAP Provider + Config UI
TASK-072: External RDBMS Provider + Config UI
TASK-073: Mail Log Queries & UI
TASK-074: Login/Security Audit Logs
```

### Phase 8-C: Analytics & Polish (2주)

```
TASK-075: Statistics & Dashboard Cache
TASK-076: API Metering
TASK-077: Audit Policy Config UI (Level 선택)
TASK-078: Export/Reports (CSV, PDF)
TASK-079: Admin Console Frontend (Notion-inspired UI)
```

### Phase 8-D: 추가 Identity Providers (미래)

```
TASK-080: Azure AD Provider
TASK-081: Google Workspace Provider
TASK-082: SCIM 2.0 Support (standard)
```

---

## 핵심 설계 결정

### Q1: 고정 역할 vs 동적 커스터마이징?
**A**: 동적 커스터마이징 (내장 역할은 고정, 커스텀 역할은 생성 가능)

### Q2: 감시 로그 범위?
**A**: Domain Admin이 선택 (Level 1/2/3)

### Q3: Identity 전략?
**A**: 추상화 + 플러그인 (Database, LDAP, Azure, External RDBMS)

### Q4: UI 스타일?
**A**: 사용자는 Notion, Admin은 AWS Console

### Q5: 다중 회사 격리?
**A**: company_id로 완전 격리 (cross-company access 불가)

### Q6: API 미터링 visibility?
**A**: Domain Admin은 자신 회사만, System Admin은 전체

### Q7: 권한 위임?
**A**: Domain Admin이 다른 사용자에게 역할 부여 가능 (감시 로그 기록)

---

## 다음 단계

1. **TASK-063 진행**: 이 스펙을 docs/ADMIN_CONSOLE_SPEC.md에 저장
2. **TASK-064부터 구현 시작**: Schema, RBAC, Auth
3. **주기적 업데이트**: 구현 중 스펙 개선사항 반영

---

**문서 버전 이력**:
- v1.0 (2026-05-10): 초안 완성
