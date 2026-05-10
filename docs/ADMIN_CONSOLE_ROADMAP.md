# GoGoMail Admin Console - Comprehensive Expansion Roadmap

**Objective**: Create a complete back-office system that provides full control over all GoGoMail and Admin backend functions

**Status**: Planning Phase - Ready for Implementation

---

## Architecture Overview

### Backend API Capabilities (Verified from AdminService)

The backend provides 150+ admin endpoints across these major categories:

#### 1. **Tenancy Management** (Companies & Domains)
- Companies: List, Get, Update Quota
- Domains: Create, List, Get, Verify DNS, Stats, Settings, Policy, Status, Quota
- Domain Checks: DNS verification, Health checks

#### 2. **User Management**
- Users: Create, List, Get, Update Status, Update Quota, Update Password
- Admin Users: Manage administrator accounts
- Directory: Principals, Aliases, Delegations, Group Memberships

#### 3. **Infrastructure & Delivery**
- Queue Stats: Real-time queue monitoring
- Delivery Routes: Create, List, Resolve, Status, Delete
- Trusted Relays: Create, List, Delete
- Delivery Attempts: List, Stats, Exhausted attempts
- Backpressure: Get, Update state

#### 4. **Data & Storage**
- Attachments: Cleanup, Stale uploads, Upload sessions
- Drive: Nodes, Upload sessions, Usage, Cleanup failures
- Quotas: Usage, Alerts, Reconciliation
- IMAP: UID backfill

#### 5. **Security & Compliance**
- DKIM Keys: Create, Deactivate, Verify DNS
- API Keys: Create, List, Rotate, Delete
- Suppression List: View, Delete entries
- Audit Logs: List, Get, Integrity checks
- Mail Flow Logs: List, Get, Stats, Daily stats

#### 6. **Operations & Analytics**
- API Usage: Daily, Monthly, Ledger, Export, Retention
- Push Notifications: Attempts, Devices, Stats
- Outbox Events: List, Get, Retry
- Alert Rules & Channels: Create, Update, Delete, List

#### 7. **Configuration**
- Company Config: Get, Set, Delete, List
- Domain Config: Get, Set, Delete, List
- User Config: Get, Set, Delete, List
- Config Propagation: Cascade changes across tenants

---

## Frontend Implementation Plan

### Phase 1: Navigation & Core Infrastructure (WEEK 1)

#### 1.1 Enhanced Sidebar Navigation
**Location**: `src/app/companies/[id]/layout.tsx`

**New Structure**:
```
Admin Console
├─ Dashboard
├─ System
│  ├─ Queue Stats
│  ├─ Backpressure
│  └─ API Health
├─ Tenancy
│  ├─ Companies
│  ├─ Domains
│  └─ Domain Settings
├─ Users & Access
│  ├─ Users
│  ├─ Admin Users
│  ├─ Directory
│  ├─ Aliases
│  └─ Delegations
├─ Delivery & Mail
│  ├─ Delivery Routes
│  ├─ Trusted Relays
│  ├─ Mail Flow Logs
│  ├─ Outbox Events
│  └─ Delivery Attempts
├─ Security
│  ├─ API Keys
│  ├─ DKIM Keys
│  ├─ Audit Logs
│  ├─ Suppression List
│  └─ Alert Rules
├─ Storage & Quotas
│  ├─ Quota Usage
│  ├─ Quota Alerts
│  ├─ Attachments
│  ├─ Drive
│  └─ Quota Reconciliation
├─ Analytics
│  ├─ API Usage
│  ├─ Push Notifications
│  └─ Reports
└─ Config Management
   ├─ Company Config
   ├─ Domain Config
   └─ User Config
```

#### 1.2 Create Reusable Components
- `<AdminTable>` - Enhanced table with filters, export, pagination
- `<AdminForm>` - Generic form builder for CRUD operations
- `<AdminModal>` - Standard modal for create/edit operations
- `<StatusBadge>` - Multi-status badge component
- `<QuotaBar>` - Storage/quota progress indicator
- `<StatsCard>` - Metric card with trends
- `<ConfigEditor>` - JSON config editor

#### 1.3 Create Reusable Hooks
- `useAdminData()` - Generic data fetching with caching
- `useMutation()` - Generic mutation handler
- `useTableState()` - Table pagination, filtering, sorting
- `useFormValidation()` - Form validation logic

---

### Phase 2: Core Pages (WEEKS 2-3)

#### 2.1 Tenancy Management

**Companies Page** (`/companies/[id]/management/companies`)
- List all companies with search/filter
- View company details and metrics
- Update company quota
- View company domains and users
- Real-time status indicators

**Domains Page** (`/companies/[id]/management/domains`)
- List domains with status indicators
- Create new domain
- DNS verification checker (live)
- Domain settings editor
- Domain quota management
- Policy editor
- API credentials management

#### 2.2 User & Access Management

**Enhanced Users Page** (`/companies/[id]/management/users`)
- Advanced filtering (status, domain, quota)
- User export (CSV/JSON)
- Bulk operations (enable/disable)
- Password reset trigger
- Quota adjustment
- User activity logs

**Directory Page** (`/companies/[id]/management/directory`)
- Principals search and management
- Aliases management (create, edit, delete)
- Delegations (roles, reassignment)
- Group memberships
- Bulk import/export

#### 2.3 Mail Infrastructure

**Delivery Routes Page** (`/companies/[id]/infrastructure/delivery-routes`)
- List active delivery routes
- Create/edit routes
- Route resolution tester
- Status management
- Route statistics

**Trusted Relays Page** (`/companies/[id]/infrastructure/relays`)
- List trusted relays
- Add/remove relays
- Configuration validation
- Usage statistics

**Outbox Events Page** (`/companies/[id]/infrastructure/outbox`)
- Real-time outbox event stream
- Event filtering by type/status
- Manual retry triggers
- Event detail inspector
- Success/failure statistics

#### 2.4 Mail Flow Logs

**Mail Logs Page** (`/companies/[id]/logs/mail-flow`)
- Mail flow log viewer with advanced filters
- Date range picker
- Status filters (delivered, failed, quarantine)
- Search by sender/recipient
- Delivery statistics
- Real-time stats dashboard
- Export capabilities

---

### Phase 3: Security & Compliance (WEEKS 4-5)

#### 3.1 Security Management

**API Keys Page** (`/companies/[id]/security/api-keys`)
- List API keys with metadata
- Create new keys
- Rotate keys with preview
- Delete keys with confirmation
- Usage statistics per key
- Last used timestamp

**DKIM Keys Page** (`/companies/[id]/security/dkim`)
- List DKIM keys by domain
- Key creation wizard
- DNS verification status
- Key rotation management
- DNS record copy/paste helper
- Deactivation management

**Suppression List** (`/companies/[id]/security/suppression`)
- View suppression entries
- Search by email/domain
- Manual entry addition
- Bulk import
- Export suppression list
- Suppress reasons (hard bounce, complaint, etc.)

#### 3.2 Audit & Compliance

**Enhanced Audit Logs** (`/companies/[id]/compliance/audit-logs`)
- Advanced filtering (user, action, resource, timestamp)
- Log integrity verification
- Export with signature
- Real-time log stream
- User activity timeline
- Change history visualization

---

### Phase 4: Storage & Quotas (WEEKS 6-7)

#### 4.1 Quota Management

**Quota Usage** (`/companies/[id]/storage/quota-usage`)
- Usage by tenant/domain/user
- Quota allocation visualization
- Usage trends chart
- Alert threshold configuration
- Quota alerts dashboard
- Reconciliation runner
- Correction workflow

**Attachments Page** (`/companies/[id]/storage/attachments`)
- Stale attachment detection
- Cleanup scheduler
- Cleanup results viewer
- Upload session management
- Stale upload cleanup
- Storage optimization tools

#### 4.2 Drive Storage

**Drive Management** (`/companies/[id]/storage/drive`)
- Drive node browser (tree view)
- Node details and metadata
- Upload session monitor
- Stale upload cleanup
- Cleanup failure resolution
- Usage summary per user

---

### Phase 5: Analytics & Operations (WEEKS 8-9)

#### 5.1 Analytics

**API Usage Analytics** (`/companies/[id]/analytics/api-usage`)
- Daily/Monthly/Ledger views
- Usage trends chart
- Per-endpoint breakdown
- Per-principal (user/api-key) breakdown
- Retention policy management
- Export batches creator
- API usage export manager

**Push Notifications** (`/companies/[id]/analytics/push`)
- Push attempt history
- Device management
- Delivery statistics
- Outcome tracking
- Push stats by type

#### 5.2 Operations

**Queue Monitor** (`/companies/[id]/operations/queue`)
- Real-time queue depth
- Message state breakdown
- Processing rate
- Error rate tracking
- Queue flush triggers
- Message state transitions

**Backpressure Control** (`/companies/[id]/operations/backpressure`)
- Current backpressure state
- Manual override controls
- Threshold management
- Historical state changes
- Impact analysis

---

### Phase 6: Configuration Management (WEEKS 10-11)

#### 6.1 Config Management

**Company Config** (`/companies/[id]/config/company`)
- Hierarchical config browser
- JSON editor with validation
- Version control/rollback
- Lock/unlock configs
- Propagation to domains/users

**Domain Config** (`/companies/[id]/config/domain`)
- Domain-level settings
- Configuration override editor
- Inheritance visualization
- Lock status management

**User Config** (`/companies/[id]/config/user`)
- Per-user settings
- Bulk configuration
- Template management

---

### Phase 7: Alerts & Monitoring (WEEKS 12)

#### 7.1 Alert Management

**Alert Rules** (`/companies/[id]/alerts/rules`)
- Alert rule builder
- Rule conditions (quota, delivery, mail flow)
- Alert actions (email, webhook, slack)
- Rule scheduling
- Rule testing

**Alert Channels** (`/companies/[id]/alerts/channels`)
- Email channel configuration
- Webhook setup
- Slack integration
- Alert event viewer
- Alert history

---

## Implementation Details

### Database Hooks to Implement

```typescript
// Hooks needed for each major feature
- useCompanies() - Company management
- useDomains() - Domain operations
- useUsers() - User management
- useDirectory() - Directory operations
- useAPIKeys() - API key management
- useDKIMKeys() - DKIM key management
- useDeliveryRoutes() - Delivery route operations
- useMailFlow Logs() - Mail flow log queries
- useAuditLogs() - Audit log operations
- useQuotaUsage() - Quota tracking
- useAttachments() - Attachment management
- useDriveNodes() - Drive storage operations
- useAPIUsage() - API usage analytics
- usePushNotifications() - Push analytics
- useAlerts() - Alert management
- useConfig() - Configuration management
- useBackpressure() - Backpressure control
```

### UI Patterns

**For each admin page:**
1. Header with page title and action buttons
2. Filter/search bar with advanced options
3. Data table with:
   - Sortable columns
   - Row selection for bulk actions
   - Pagination
   - Export buttons
4. Detail modal for viewing/editing
5. Confirmation dialogs for destructive actions
6. Success/error notifications
7. Loading states with skeleton
8. Empty state with helpful message

### API Integration Pattern

All pages will use the existing API proxy route:
```
POST /api/admin/[endpoint] → /admin/v1/[endpoint]
GET /api/admin/[endpoint] → /admin/v1/[endpoint]
PUT /api/admin/[endpoint] → /admin/v1/[endpoint]
DELETE /api/admin/[endpoint] → /admin/v1/[endpoint]
```

---

## Priority Tiers

### Tier 1: Critical (Must Have)
- [x] Dashboard & Metrics
- [ ] User Management
- [ ] Domains Management
- [ ] API Keys
- [ ] Audit Logs
- [ ] Delivery Routes
- [ ] Queue Stats

### Tier 2: Important (Should Have)
- [ ] Mail Flow Logs
- [ ] DKIM Keys
- [ ] Quota Management
- [ ] Alert Rules
- [ ] API Usage Analytics
- [ ] Suppression List

### Tier 3: Nice to Have (Could Have)
- [ ] Drive Storage Management
- [ ] Attachment Cleanup
- [ ] Configuration Editor
- [ ] Push Notifications
- [ ] DAV Sync Retention
- [ ] IMAP UID Backfill

---

## Timeline & Milestones

- **Week 1**: Navigation restructuring + Core components
- **Weeks 2-3**: Phase 2 pages (Tenancy, Users, Mail)
- **Weeks 4-5**: Phase 3 pages (Security & Compliance)
- **Weeks 6-7**: Phase 4 pages (Storage & Quotas)
- **Weeks 8-9**: Phase 5 pages (Analytics & Operations)
- **Weeks 10-11**: Phase 6 pages (Configuration)
- **Week 12**: Phase 7 pages (Alerts) + Polish & Testing
- **Week 13**: E2E Testing + Performance optimization
- **Week 14**: Documentation + Release preparation

---

## Success Criteria

✅ All 150+ admin endpoints have UI representations
✅ All Cloudscape Design System components properly used
✅ Web standards and accessibility compliance
✅ WCAG 2.1 Level AA accessibility
✅ Keyboard navigation fully functional
✅ Real-time data updates where applicable
✅ 95%+ E2E test coverage
✅ Zero console errors in production build
✅ Performance: Page load < 3s, API response < 1s
✅ Professional enterprise-grade appearance
✅ Complete documentation with screenshots

---

## Estimated Effort

- **Total Pages**: 35+
- **Total Components**: 50+
- **Total Hooks**: 20+
- **Estimated Coding Hours**: 200+
- **Estimated Testing Hours**: 80+
- **Total Timeline**: 14 weeks (2 developers at normal pace)

---

## Tools & Technologies

- **Frontend**: Next.js 15 + TypeScript
- **UI Library**: Cloudscape Design System
- **Data Fetching**: React Query
- **Forms**: Custom validation + Cloudscape inputs
- **Charts**: Recharts or Chart.js
- **Testing**: OpenChrome + Playwright
- **API**: Next.js API routes proxy pattern

---

## Risk Mitigation

1. **API Breaking Changes**: Implement API versioning early
2. **Performance**: Implement pagination from day 1
3. **Security**: Use secure cookie storage, CSRF tokens
4. **Complexity**: Modularize components, reuse patterns
5. **Testing**: Automate E2E tests for critical paths

---

## Next Steps

1. ✅ Approve this roadmap
2. Create enhanced navigation layout
3. Build reusable component library
4. Implement Phase 1 pages
5. Set up automated testing infrastructure
6. Begin Phase 2 implementation

**Ready to start implementation!**
