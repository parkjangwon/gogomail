# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ⏳ TASK-084: Alerts & Notifications

**STATUS: IN_PROGRESS**

### Progress
- ⏳ Next: Backend alert system, database schema, service layer, frontend

### 제목
Alerts & Notifications — Admin Console 임계값 기반 자동 알림 시스템

### 배경
Phase 8-D (UI/UX & Settings)에서 정의한 Alert & Notification 기능:
- 스토리지 사용량 > 80% 알림
- 로그인 실패 > 10회/시간 알림
- API 오류율 > 5% 알림
- 알림 채널: 이메일, 웹훅, 대시보드 팝업

기본적인 임계값 모니터링과 다채널 알림 전송 시스템 구현.

### 구현 대상

#### 1. 데이터베이스 스키마
- 새 테이블: `alert_rules` (임계값 정의)
  ```sql
  CREATE TABLE alert_rules (
    id UUID PRIMARY KEY,
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    alert_type TEXT NOT NULL, -- 'storage', 'login_failures', 'api_errors'
    name TEXT NOT NULL,
    description TEXT,
    threshold NUMERIC NOT NULL, -- percentage or count
    check_interval_minutes INT NOT NULL DEFAULT 5,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    CONSTRAINT valid_threshold CHECK (threshold > 0)
  );
  ```

- 새 테이블: `alert_channels` (알림 채널 구성)
  ```sql
  CREATE TABLE alert_channels (
    id UUID PRIMARY KEY,
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    channel_type TEXT NOT NULL, -- 'email', 'webhook', 'dashboard'
    name TEXT NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    -- Channel-specific config (JSON)
    config JSONB NOT NULL, -- email: {recipients: []}, webhook: {url, auth_header?}
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES admin_users(id) ON DELETE SET NULL
  );
  ```

- 새 테이블: `alert_rule_channels` (Rule → Channel 매핑)
  ```sql
  CREATE TABLE alert_rule_channels (
    id UUID PRIMARY KEY,
    alert_rule_id UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    alert_channel_id UUID NOT NULL REFERENCES alert_channels(id) ON DELETE CASCADE,
    UNIQUE(alert_rule_id, alert_channel_id)
  );
  ```

- 새 테이블: `alert_events` (발생한 알림 기록)
  ```sql
  CREATE TABLE alert_events (
    id UUID PRIMARY KEY,
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    alert_rule_id UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    current_value NUMERIC NOT NULL,
    threshold NUMERIC NOT NULL,
    message TEXT,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    INDEX idx_company_triggered (company_id, triggered_at DESC)
  );
  ```

#### 2. 서비스 계층 (`internal/admin/service.go`)
```go
type AlertRule struct {
  ID                   string
  CompanyID            string
  AlertType            string // 'storage', 'login_failures', 'api_errors'
  Name                 string
  Description          string
  Threshold            float64
  CheckIntervalMinutes int
  IsEnabled            bool
  CreatedAt            time.Time
  CreatedBy            string
}

type AlertChannel struct {
  ID          string
  CompanyID   string
  ChannelType string // 'email', 'webhook', 'dashboard'
  Name        string
  IsEnabled   bool
  Config      json.RawMessage
  CreatedAt   time.Time
  CreatedBy   string
}

type AlertEvent struct {
  ID          string
  CompanyID   string
  AlertRuleID string
  CurrentValue float64
  Threshold   float64
  Message     string
  TriggeredAt time.Time
  ResolvedAt  *time.Time
}

// Service methods
func (svc *Service) CreateAlertRule(ctx context.Context, rule *AlertRule) error
func (svc *Service) UpdateAlertRule(ctx context.Context, rule *AlertRule) error
func (svc *Service) DeleteAlertRule(ctx context.Context, ruleID string) error
func (svc *Service) ListAlertRules(ctx context.Context, companyID string) ([]AlertRule, error)
func (svc *Service) GetAlertRule(ctx context.Context, ruleID string) (*AlertRule, error)

func (svc *Service) CreateAlertChannel(ctx context.Context, channel *AlertChannel) error
func (svc *Service) UpdateAlertChannel(ctx context.Context, channel *AlertChannel) error
func (svc *Service) DeleteAlertChannel(ctx context.Context, channelID string) error
func (svc *Service) ListAlertChannels(ctx context.Context, companyID string) ([]AlertChannel, error)

func (svc *Service) ListAlertEvents(ctx context.Context, companyID string, filter AlertEventFilter) ([]AlertEvent, error)
```

#### 3. 백엔드 API (`internal/httpapi/admin.go`)
- `POST /admin/v1/companies/{id}/alert-rules` — Create alert rule
- `PUT /admin/v1/alert-rules/{id}` — Update alert rule
- `DELETE /admin/v1/alert-rules/{id}` — Delete alert rule
- `GET /admin/v1/companies/{id}/alert-rules` — List alert rules
- `GET /admin/v1/alert-rules/{id}` — Get alert rule detail

- `POST /admin/v1/companies/{id}/alert-channels` — Create channel
- `PUT /admin/v1/alert-channels/{id}` — Update channel
- `DELETE /admin/v1/alert-channels/{id}` — Delete channel
- `GET /admin/v1/companies/{id}/alert-channels` — List channels

- `GET /admin/v1/companies/{id}/alert-events` — List alert events (with filters)

#### 4. 프론트엔드 (`apps/admin/src/`)
- **Page**: `(console)/companies/[id]/alerts/page.tsx`
  - Alert Rules 목록 및 생성/수정/삭제 폼
  - Alert Channels 목록 및 설정
  - Alert Events 기록 (타임라인)
  
- **Hook**: `hooks/useAlerts.ts`
  - `useAlertRules(companyId)` — GET rules
  - `useCreateAlertRule()` — POST rule
  - `useUpdateAlertRule()` — PUT rule
  - `useDeleteAlertRule()` — DELETE rule
  - `useAlertChannels(companyId)` — GET channels
  - `useAlertEvents(companyId, filter)` — GET events

- **컴포넌트**:
  - `AlertRulesList` — Rules 테이블, CRUD 버튼
  - `AlertRuleForm` — 규칙 설정 폼
  - `AlertChannelsList` — Channels 테이블, 설정 폼
  - `AlertEventsList` — Events 타임라인
  - `ChannelConfigModal` — Channel별 설정 (Email recipients, Webhook URL 등)

- **권한**: Company Admin 이상만 접근 가능

### 완료 조건

- [ ] `go test ./...` 통과
- [ ] 데이터베이스 마이그레이션 작성 (alert_rules, alert_channels, alert_events)
- [ ] AlertRule, AlertChannel, AlertEvent 모델 정의
- [ ] Repository 인터페이스 및 PostgreSQL 구현
- [ ] Service 메서드 구현 (CRUD, 조회)
- [ ] API endpoints 구현 및 핸들러 작성
- [ ] 폼 검증 (Threshold > 0, Channel config 유효성)
- [ ] 에러 처리
- [ ] OpenAPI 스키마 업데이트
- [ ] React Query hooks 구현
- [ ] 프론트엔드 page.tsx 구현
- [ ] Alert Rules 관리 UI
- [ ] Alert Channels 관리 UI
- [ ] Alert Events 조회 UI
- [ ] docs/CURRENT_STATUS.md 갱신
- [ ] docs/backend-roadmap.md TASK-084 체크

### 다음 태스크
TASK-085: Admin Console Frontend (Phase 1)

### 루프 절차

```
1. 이 파일 읽기 ✓
2. 데이터베이스 마이그레이션 작성
3. 모델 정의 (AlertRule, AlertChannel, AlertEvent)
4. Repository 인터페이스 및 구현
5. Service 메서드 구현
6. API 핸들러 구현
7. OpenAPI 문서 작성
8. 단위 테스트 작성 및 통과
9. go test ./... 실행 (모두 통과)
10. React Query hooks 구현
11. 프론트엔드 페이지 구현
12. docs 업데이트
13. git add + commit + push
14. 이 파일을 TASK-085로 교체
```
