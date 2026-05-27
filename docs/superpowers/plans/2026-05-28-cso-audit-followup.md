# CSO Audit Follow-up: Repo Hygiene, Settings Deduplication, Docs, Page Decomposition

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address the four priority areas from the CSO audit: repo hygiene, dead settings UI code, security docs restructuring, and large console page decomposition.

**Architecture:** Incremental — each task is independently committable. Tasks 1–2 are quick cleanup; Task 3 is doc-only; Tasks 4–5 are the main structural refactor of the largest console pages.

**Tech Stack:** Go (backend), Next.js + TypeScript (frontend), pnpm workspaces, Cloudscape Design System (console).

---

## Pre-flight: What the audit said vs. what's already done

The audit listed 8 security remediation items. All 8 are already implemented:

| Item | Status | Location |
|---|---|---|
| GOGOMAIL_ENV default = production | ✓ Done | `internal/config/config.go:372` |
| X-Gogomail-* strip middleware | ✓ Done | `internal/httpapi/admin_middleware.go:51`, wired at `internal/app/run.go:489` |
| Helm CHANGEME guard | ✓ Done | `helm/gogomail/templates/_helpers.tpl` `requireNotChangeme` |
| docker-compose.scale.yml sslmode=require | ✓ Done | Line 27 |
| RDBMS IdP SQL allowlist | ✓ Done | `internal/idprovider/rdbms/provider.go` `validateSourceQuery` |
| JWT golang-jwt/jwt/v5 migration | ✓ Done | `internal/auth/jwt.go:13` |
| Legacy password hash auto-upgrade | ✓ Done | `internal/maildb/user_auth.go:98` |
| CSP nonce | ✓ Done | `apps/webmail/src/middleware.ts:11`, `apps/console/src/middleware.ts:11` |

Tasks below address the remaining four gaps.

---

### Task 1: Repo hygiene — untrack build artifacts and runtime data

**Goal:** Remove `apps/console/tsconfig.tsbuildinfo` and the `.eml` file from git tracking, and add `.gitignore` patterns so they can never be re-tracked.

**Files:**
- Modify: `.gitignore` (root)
- Git: `git rm --cached apps/console/tsconfig.tsbuildinfo`
- Git: `git rm --cached apps/console/var/mailstore/mailstore/6106af4e-fc44-4a65-890d-55bb35741d6c/6049fa6e-d649-44d3-83d2-b548c7e787d5/f4b5a283-d1e6-47a9-a69a-e71e90f5343c/maildir/2026/05/1778486139520-5dead6c33f1ee8a3.eml`

**Acceptance Criteria:**
- [ ] `git ls-files | grep -E "tsbuildinfo|\.eml$"` returns empty
- [ ] Root `.gitignore` contains a pattern covering `apps/*/var/`
- [ ] `go test ./...` still passes (backend unaffected)

**Verify:** `git ls-files | grep -E "tsbuildinfo|\.eml$"` → empty output

**Steps:**

- [ ] **Step 1: Check current .gitignore for gap**

The root `.gitignore` already has `apps/*/tsconfig.tsbuildinfo` and `/var/`. The `/var/` entry is anchored to root so it does NOT cover `apps/console/var/`. Add the missing pattern:

```
# Runtime data under app subdirectories (mailstore, temp data)
apps/*/var/
```

Add this line after the existing `/var/` line in root `.gitignore`.

- [ ] **Step 2: Untrack the build artifact**

```bash
git rm --cached apps/console/tsconfig.tsbuildinfo
```

Expected: `rm 'apps/console/tsconfig.tsbuildinfo'`

- [ ] **Step 3: Untrack the .eml file**

```bash
git rm --cached "apps/console/var/mailstore/mailstore/6106af4e-fc44-4a65-890d-55bb35741d6c/6049fa6e-d649-44d3-83d2-b548c7e787d5/f4b5a283-d1e6-47a9-a69a-e71e90f5343c/maildir/2026/05/1778486139520-5dead6c33f1ee8a3.eml"
```

Expected: `rm 'apps/console/var/...'`

- [ ] **Step 4: Verify nothing is still tracked**

```bash
git ls-files | grep -E "tsbuildinfo|\.eml$"
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add .gitignore
git commit -m "chore: untrack build artifacts and runtime data, add apps/*/var/ gitignore"
```

---

### Task 2: Remove dead SettingsModal code from webmail Sidebar

**Goal:** Remove the never-shown `SettingsModal` from `Sidebar.tsx` and delete the now-unused `SettingsModal.tsx` / `SettingsModalContent.tsx` files.

**Context:** `useSidebarFolders.ts` exposes `showSettings`/`setShowSettings` but `setShowSettings(true)` is never called anywhere in the codebase — the state is always `false`. The actual settings flow in `mail/page.tsx` uses `setActiveApp('settings')` to show `SettingsView`. The modal is dead code that creates a risk of divergence if anyone accidentally calls `setShowSettings(true)` in the future.

**Files:**
- Modify: `apps/webmail/src/components/sidebar/useSidebarFolders.ts`
- Modify: `apps/webmail/src/components/Sidebar.tsx`
- Delete: `apps/webmail/src/components/SettingsModal.tsx`
- Delete: `apps/webmail/src/components/SettingsModalContent.tsx`

**Acceptance Criteria:**
- [ ] `SettingsModal` and `SettingsModalContent` files are gone
- [ ] `useSidebarFolders` no longer exports `showSettings`/`setShowSettings`
- [ ] `pnpm --dir apps/webmail type-check` passes with no errors

**Verify:** `pnpm --dir apps/webmail type-check` → exit 0, no type errors

**Steps:**

- [ ] **Step 1: Remove showSettings state from useSidebarFolders.ts**

Replace the full file content (removing `showSettings`/`setShowSettings`):

```typescript
'use client';
import { useState } from 'react';

export function useSidebarFolders() {
  const [dragOverFolderId, setDragOverFolderId] = useState<string | null>(null);
  const [newFolderInput, setNewFolderInput] = useState('');
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [renamingFolderId, setRenamingFolderId] = useState<string | null>(null);
  const [renamingValue, setRenamingValue] = useState('');
  const [hoveredFolderId, setHoveredFolderId] = useState<string | null>(null);

  return {
    dragOverFolderId,
    setDragOverFolderId,
    newFolderInput,
    setNewFolderInput,
    showNewFolder,
    setShowNewFolder,
    renamingFolderId,
    setRenamingFolderId,
    renamingValue,
    setRenamingValue,
    hoveredFolderId,
    setHoveredFolderId,
  };
}
```

- [ ] **Step 2: Remove SettingsModal import and render from Sidebar.tsx**

Remove the import line:
```typescript
import { SettingsModal } from '@/components/SettingsModal';
```

Remove the destructured values from `useSidebarFolders()`:
```typescript
    showSettings,
    setShowSettings,
```

Remove the JSX block at the end of the Sidebar return (before the closing `</>`):
```tsx
      {showSettings && (
        <SettingsModal onClose={() => setShowSettings(false)} userEmail={userEmailAddress} />
      )}
```

- [ ] **Step 3: Delete the unused files**

```bash
rm apps/webmail/src/components/SettingsModal.tsx
rm apps/webmail/src/components/SettingsModalContent.tsx
```

- [ ] **Step 4: Type-check**

```bash
pnpm --dir apps/webmail type-check
```

Expected: exit 0 with no errors.

- [ ] **Step 5: Commit**

```bash
git add apps/webmail/src/components/sidebar/useSidebarFolders.ts \
        apps/webmail/src/components/Sidebar.tsx
git rm apps/webmail/src/components/SettingsModal.tsx \
       apps/webmail/src/components/SettingsModalContent.tsx
git commit -m "refactor(webmail): remove dead SettingsModal from Sidebar, settings flow goes through SettingsView"
```

---

### Task 3: Restructure SECURITY_REVIEW.md to reflect actual implemented state

**Goal:** Update `docs/SECURITY_REVIEW.md` to accurately reflect what is and isn't implemented, using the structured sections the auditor recommended: Implemented / Accepted Risk / Post-Release Hardening.

**Files:**
- Modify: `docs/SECURITY_REVIEW.md`

**Acceptance Criteria:**
- [ ] Doc has sections: `## Implemented Controls`, `## Accepted Risk`, `## Post-Release Hardening`
- [ ] All 8 items from the audit's "1순위" list appear in `## Implemented Controls`
- [ ] IDOR remediation rounds are summarized in `## Implemented Controls`
- [ ] `## Post-Release Hardening` contains items currently in `## Remaining Follow-Ups`
- [ ] `Last updated` date is current (2026-05-28)

**Verify:** Manual review: grep for each of the 8 audit items in the Implemented section.

**Steps:**

- [ ] **Step 1: Rewrite docs/SECURITY_REVIEW.md**

Replace the entire file with this structure:

```markdown
# gogomail Security Review

Last updated: 2026-05-28

## Baseline

This review tracks OWASP Top 10 2025 oriented hardening across the Go backend,
admin console, and webmail frontend.

Primary risk areas covered in this pass:

- Broken access control (IDOR) and insecure defaults
- XSS in rendered email content
- SSRF through image proxy, webhooks, and remote HTTP integrations
- CSRF against cookie-backed Next.js API proxies
- Header, path, and download-response injection
- Internal header spoofing via trusted proxy headers
- Dependency and static security checks
- Password storage and upgrade path

## Implemented Controls

### Access Control

- Production bootstrap admin login using `admin@system / admin1234` is disabled
  unless `GOGOMAIL_ENV` is explicitly non-production. Default value is
  `"production"` (`internal/config/config.go`).
- IDOR sweep complete across all admin API handlers: every handler that operates
  on a company-scoped resource now calls `requiresCompanyAccess` before
  proceeding. Covered: admin_storage.go (drive/storage), admin_mail.go (push,
  DKIM, suppression), admin_ldap_sync.go, admin_rdbms_sync.go, admin_directory.go,
  admin_governance.go, admin_system.go, admin_company.go, admin_domain.go,
  admin_usage.go, admin_operations.go (mail-flow-logs).
- Company/domain security governance is explicit via `/security/governance`;
  platform invariants remain fixed, while approved operational exceptions such
  as private-network webhook targets are deny by default and can be enabled per
  tenant policy.

### Trusted Header Stripping

- `StripInternalHeadersMiddleware` (`internal/httpapi/admin_middleware.go`)
  removes all `X-Gogomail-*` internal routing headers from every inbound request
  before any handler runs. Wired unconditionally at `internal/app/run.go:489`.
  This prevents external clients from spoofing resolved user/tenant identity
  headers that the backend trusts for authentication decisions.

### SSRF and Outbound URL Safety

- Backend outbound URL guard rejects non-HTTP(S), localhost, loopback, private,
  link-local, multicast, unspecified, and metadata-service addresses after DNS
  resolution; guarded clients re-check redirects and cap redirect chains.
- Attachment scan webhooks use the outbound URL guard by default. Unit tests may
  opt into private-network endpoints for local `httptest` servers only.
- Admin company webhooks reject private URLs and do not expose stored webhook
  secrets in list responses; only a suffix is returned after storage.
- Webmail image proxy rejects SVG, private destinations, oversized images, and
  redirects to private destinations.

### XSS / Content Safety

- Webmail HTML email rendering removes high-risk active content tags and strips
  unsafe URL schemes before inserting sanitized HTML.
- Production CSP removes `unsafe-eval`, adds `upgrade-insecure-requests`, and
  both apps now set COOP/CORP plus DNS prefetch disabling.
- Both apps generate a per-request CSP nonce in Next.js middleware and forward
  it via `x-nonce` header to `layout.tsx`. The nonce is applied to inline
  `<script>` tags, removing the need for `unsafe-inline` on scripts.
  `style-src` still uses `unsafe-inline` pending nonce-based style bootstrapping
  (see Post-Release Hardening).

### CSRF

- Cookie-backed mutating API routes now require same-origin `Origin` or
  `Referer`; requests without browser provenance are rejected instead of treated
  as implicitly trusted.

### Proxy Security

- Console admin proxies are consolidated into a shared server helper that
  encodes path segments, checks same-origin mutating requests, forwards only
  allowlisted request headers, and returns `no-store` plus `nosniff`.
- Webmail mail proxy now encodes backend path segments, checks same-origin
  mutating requests, strips client-supplied credentials, and forwards only the
  required upload/download headers.
- Login/logout proxy responses set `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`; console demo credentials are hidden in
  production builds.
- Frontend server routes use server-only `GOGOMAIL_BACKEND_URL`; public browser
  configuration should use purpose-specific public origins such as
  `NEXT_PUBLIC_GOGOMAIL_PUBLIC_BASE_URL` for displayed SCIM endpoints.

### Secrets and Deployment

- Enterprise cookie posture uses `__Host-` token cookie names in production,
  with legacy cookie cleanup during migration.
- Helm chart `requireNotChangeme` helper (`helm/gogomail/templates/_helpers.tpl`)
  fails `helm install`/`helm upgrade` if any of `GOGOMAIL_DM_MASTER_KEY`,
  `GOGOMAIL_AUTH_JWT_SECRET`, or `GOGOMAIL_ADMIN_TOKEN` still contain the
  `CHANGEME` placeholder. Database URL in `docker-compose.scale.yml` uses
  `sslmode=require`.

### Authentication

- JWT library is `github.com/golang-jwt/jwt/v5` (`internal/auth/jwt.go`).
  Custom `iat`-in-future guard added on top because golang-jwt v5 does not
  enforce it by default.
- Password hashing uses PBKDF2-SHA256 (`internal/auth/password.go`). Plain and
  raw-SHA256 hashes are rejected in production. Legacy hashes are automatically
  upgraded to PBKDF2-SHA256 on next successful login
  (`internal/maildb/user_auth.go:upgradePasswordHash`).
- RDBMS identity provider validates all configured sync queries with
  `validateSourceQuery` before connecting: must start with SELECT, must not
  contain DML/DDL keywords, max 4096 bytes, no internal semicolons
  (`internal/idprovider/rdbms/provider.go`).

### SMTP / Mail Security

- Built-in SMTP spam filtering supports strict SPF/DKIM/DMARC scoring,
  policy-managed RBL/DNSBL zone registration, reject-on-listed-IP behavior,
  dangerous attachment extension scoring, and policy-driven bulk recipient
  thresholds.
- SMTP receive parsing keeps body extraction bounded to 64KB.
- ClamAV scan admission is bounded: concurrent scans capped, oversized streams
  tempfail, deadlines enforced, circuit breaker on repeated failures.
- Spam filter packs are tenant-scoped; `gogomail-core-*` system pack IDs cannot
  be overridden by tenant input.
- Admin console spam-filter management exposes built-in pack toggles and
  tenant-owned custom phrase packs for company defaults and domain overrides.

### Dependencies

- Go builds pinned to `go1.26.3`; both frontend apps override `postcss` to
  `^8.5.14` so production dependency audits are clean.

## Accepted Risk

- `style-src 'unsafe-inline'` is still present in production CSP. Removing it
  requires nonce-based style bootstrapping for theme injection, which causes
  visual flicker on first paint. Accepted until the style bootstrap can be
  refactored (see Post-Release Hardening).
- RBL lookup failures fail open to preserve receive availability. A failed RBL
  check does not cause the message to be rejected. Accepted as an availability
  trade-off; operators can disable RBL if a zone provider is unreliable.

## Post-Release Hardening

These items are not release blockers but should be addressed in subsequent milestones:

- **Nonce-based style bootstrapping**: Refactor theme injection so inline styles
  can be nonce-tagged, eliminating `unsafe-inline` from `style-src`.
- **Centralized security event logging**: Add structured logging for rejected
  same-origin checks, private URL guards, and oversized proxy attempts once the
  audit pipeline is finalized.
- **ClamAV operator monitoring**: Add admin console visibility into ClamAV health
  and signature freshness so operators can detect stale signatures before mail
  acceptance depends on them.
- **Filter-pack lifecycle**: Signed import/export bundles, staged rollout, hit-rate
  analytics, and emergency disable once production telemetry volume is available.
- **Deployment-specific webhook allowlists**: Narrower per-deployment internal
  webhook allowlists behind the governance setting if operators need tighter
  controls than the current tenant-level deny.

## Verification Commands

```bash
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
pnpm --dir apps/webmail test:security-helpers
pnpm --dir apps/webmail type-check
pnpm --dir apps/console exec vitest run src/lib/__tests__/adminProxy.test.ts
pnpm --dir apps/console type-check
pnpm --dir apps/webmail audit --prod
pnpm --dir apps/console audit --prod
```
```

- [ ] **Step 2: Commit**

```bash
git add docs/SECURITY_REVIEW.md
git commit -m "docs(security): restructure SECURITY_REVIEW.md — Implemented / Accepted Risk / Post-Release Hardening"
```

---

### Task 4: Decompose spam-filter page (1273 lines → orchestrator + modules)

**Goal:** Break `apps/console/src/app/companies/[id]/security/spam-filter/page.tsx` into focused modules by responsibility: types, data hook, policy editor, events table, and stats panel.

**Files:**
- Create: `apps/console/src/app/companies/[id]/security/spam-filter/spamFilterTypes.ts`
- Create: `apps/console/src/app/companies/[id]/security/spam-filter/useSpamFilter.ts`
- Create: `apps/console/src/app/companies/[id]/security/spam-filter/SpamFilterPolicyEditor.tsx`
- Create: `apps/console/src/app/companies/[id]/security/spam-filter/SpamFilterEventsTable.tsx`
- Create: `apps/console/src/app/companies/[id]/security/spam-filter/SpamFilterStats.tsx`
- Modify: `apps/console/src/app/companies/[id]/security/spam-filter/page.tsx` (orchestrator, target ~150 lines)

**Acceptance Criteria:**
- [ ] `page.tsx` is ≤ 200 lines
- [ ] Each new file has a single clear responsibility (types / data / policy edit / events / stats)
- [ ] `pnpm --dir apps/console type-check` passes
- [ ] Visual behavior is identical (no state regressions — same fetch paths, same UI structure)

**Verify:** `pnpm --dir apps/console type-check` → exit 0

**Steps:**

- [ ] **Step 1: Extract types to spamFilterTypes.ts**

Move all interface/type definitions out of `page.tsx` into a new file. These are the types defined at the top of the file (lines ~1-103 approximately):

```typescript
// apps/console/src/app/companies/[id]/security/spam-filter/spamFilterTypes.ts
export interface SpamFilterPolicy {
  enabled: boolean;
  spam_threshold: number;
  virus_scan_enabled: boolean;
  strict_auth_enabled: boolean;
  rbl_check_enabled: boolean;
  rbl_reject_enabled: boolean;
  rbl_zones: string[];
  blocked_extensions: string[];
  blocked_senders: string[];
  allowed_senders: string[];
  quarantine_enabled: boolean;
  max_attachment_mb: number;
  bulk_recipient_limit: number;
  filter_packs: FilterPackBundle;
}

export interface FilterPackBundle {
  enabled_pack_ids: string[];
  custom_packs: FilterPack[];
}

export interface FilterPack {
  id: string;
  version: string;
  name: string;
  description: string;
  category: string;
  source: 'system' | 'custom' | string;
  enabled: boolean;
  rules: FilterRule[];
}

export interface FilterRule {
  id: string;
  type: 'phrase' | 'attachment_extension' | 'bulk_recipient' | 'auth_failure' | 'sender_domain' | 'url_host' | 'header_anomaly' | string;
  target?: 'subject' | 'body' | 'subject_body' | string;
  patterns: string[];
  score: number;
  enabled: boolean;
  action?: 'quarantine' | 'reject' | string;
}

export interface SpamFilterEvent {
  id: string;
  created_at: string;
  from_addr?: string;
  mail_from?: string;
  rcpt_to?: string;
  subject?: string;
  flow_status: string;
  spam_score?: number;
  action?: string;
  rule_hits?: RuleHit[];
  details?: Record<string, unknown>;
}

export interface RuleHit {
  rule_id: string;
  score: number;
  matched?: string[];
}

export interface SpamFilterStats {
  total: number;
  passed: number;
  quarantined: number;
  rejected: number;
  spam_score_avg?: number;
  by_day?: DayCount[];
}

export interface DayCount {
  date: string;
  total: number;
  quarantined: number;
  rejected: number;
}

export interface DomainOption {
  value: string;
  label: string;
}

export type EventFilter = 'all' | 'quarantined' | 'rejected' | 'passed';
```

Also move `defaultPolicy()` and `builtinFilterPacks` (the large constant array) into this file or into a separate `spamFilterDefaults.ts`. For simplicity, move them to `spamFilterTypes.ts` as well (they are pure data, no React hooks):

```typescript
export const COMPANY_SCOPE_VALUE = '__company__';

export const defaultPolicy = (): SpamFilterPolicy => ({
  enabled: true,
  spam_threshold: 5,
  virus_scan_enabled: true,
  strict_auth_enabled: true,
  rbl_check_enabled: false,
  rbl_reject_enabled: true,
  rbl_zones: [],
  blocked_extensions: ['.exe', '.bat', '.cmd', '.scr', '.vbs', '.js', '.ps1', '.jar', '.docm', '.xlsm'],
  blocked_senders: [],
  allowed_senders: [],
  quarantine_enabled: true,
  max_attachment_mb: 25,
  bulk_recipient_limit: 50,
  filter_packs: {
    enabled_pack_ids: ['gogomail-core-auth', 'gogomail-core-malware', 'gogomail-core-phishing-ko', 'gogomail-core-bulk', 'gogomail-core-url', 'gogomail-core-sender'],
    custom_packs: [],
  },
});

// ... copy builtinFilterPacks array from page.tsx verbatim
```

- [ ] **Step 2: Extract data hook to useSpamFilter.ts**

The hook manages: policy fetch, domain fetch, events/stats fetch, save, and notifications. Extract all `useState` + `useEffect` + `useCallback` data logic from `page.tsx` into:

```typescript
// apps/console/src/app/companies/[id]/security/spam-filter/useSpamFilter.ts
'use client';

import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { SelectProps, FlashbarProps } from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import {
  SpamFilterPolicy, SpamFilterEvent, SpamFilterStats,
  DomainOption, EventFilter,
  COMPANY_SCOPE_VALUE, defaultPolicy,
} from './spamFilterTypes';

export function useSpamFilter() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const companyScopeOption = useMemo<SelectProps.Option>(() => ({
    label: t('pages.spam_filter_page.scope_company'),
    value: COMPANY_SCOPE_VALUE,
    description: t('pages.spam_filter_page.scope_company_desc'),
  }), [t]);

  const [policy, setPolicy] = useState<SpamFilterPolicy>(defaultPolicy());
  const [savedPolicyJson, setSavedPolicyJson] = useState('');
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const hasLoadedRef = useRef(false);
  const [saving, setSaving] = useState(false);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [events, setEvents] = useState<SpamFilterEvent[]>([]);
  const [stats, setStats] = useState<SpamFilterStats | null>(null);
  const [domains, setDomains] = useState<DomainOption[]>([]);
  const [loadingDomains, setLoadingDomains] = useState(false);
  const [selectedScope, setSelectedScope] = useState<SelectProps.Option>(companyScopeOption);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  // ... move all fetchDomains, fetchPolicy, handleSave, useEffects here
  // Return the full shape the page needs

  return {
    t, cid,
    policy, setPolicy, savedPolicyJson,
    loading, refreshing, saving,
    notifications, setNotifications,
    events, stats,
    domains, loadingDomains,
    selectedScope, setSelectedScope,
    scopeOptions,
    activeDomainId,
    lastUpdated,
    isDirty,
    fetchPolicy,
    handleSave,
    companyScopeOption,
  };
}
```

Copy the full implementations of `fetchDomains`, `fetchPolicy`, `handleSave` from `page.tsx` verbatim into this hook. Do NOT change any fetch paths or business logic — just move the code.

- [ ] **Step 3: Extract SpamFilterStats.tsx**

```typescript
// apps/console/src/app/companies/[id]/security/spam-filter/SpamFilterStats.tsx
'use client';

import { ColumnLayout, Box } from '@cloudscape-design/components';
import { SpamFilterStats as StatsType } from './spamFilterTypes';

interface Props {
  stats: StatsType | null;
  lastUpdated: Date | null;
  t: (key: string, ...args: unknown[]) => string;
}

export function SpamFilterStats({ stats, lastUpdated, t }: Props) {
  // Move the stats section JSX from page.tsx here verbatim
}
```

- [ ] **Step 4: Extract SpamFilterEventsTable.tsx**

```typescript
// apps/console/src/app/companies/[id]/security/spam-filter/SpamFilterEventsTable.tsx
'use client';

import { SpamFilterEvent, EventFilter } from './spamFilterTypes';
// ... Cloudscape imports

interface Props {
  events: SpamFilterEvent[];
  eventFilter: EventFilter;
  onFilterChange: (f: EventFilter) => void;
  eventFrom: string;
  onEventFromChange: (v: string) => void;
  eventTo: string;
  onEventToChange: (v: string) => void;
  eventMinScore: string;
  onEventMinScoreChange: (v: string) => void;
  detailEvent: SpamFilterEvent | null;
  onSelectEvent: (e: SpamFilterEvent | null) => void;
  locale: string;
  t: (key: string, ...args: unknown[]) => string;
}

export function SpamFilterEventsTable(props: Props) {
  // Move events table JSX from page.tsx verbatim
}
```

- [ ] **Step 5: Extract SpamFilterPolicyEditor.tsx**

This is the largest sub-component — it owns all the policy edit UI: thresholds, blocked lists, RBL zones, filter packs, custom pack rule editor:

```typescript
// apps/console/src/app/companies/[id]/security/spam-filter/SpamFilterPolicyEditor.tsx
'use client';

import { SpamFilterPolicy, FilterPackBundle } from './spamFilterTypes';
// ... Cloudscape imports, builtinFilterPacks

interface Props {
  policy: SpamFilterPolicy;
  onPolicyChange: (p: SpamFilterPolicy) => void;
  saving: boolean;
  isDirty: boolean;
  onSave: () => void;
  t: (key: string, ...args: unknown[]) => string;
}

export function SpamFilterPolicyEditor({ policy, onPolicyChange, saving, isDirty, onSave, t }: Props) {
  // Local state for form inputs (newBlockedExt, newBlockedSender, etc.)
  // Move all policy-editor state and JSX from page.tsx here
}
```

Policy editor local state (`newBlockedExt`, `newBlockedSender`, `newAllowedSender`, `newRBLZone`, `newPackId`, `newPackName`, `newPackPhrase`, `newPackScore`, `selectedCustomPackId`, `newRuleId`, `newRuleType`, `newRuleTarget`, `newRulePatterns`, `newRuleScore`, `newRuleAction`) all moves into this component since it's entirely local to the editor.

- [ ] **Step 6: Rewrite page.tsx as orchestrator**

```typescript
// apps/console/src/app/companies/[id]/security/spam-filter/page.tsx
'use client';

import { useState } from 'react';
import {
  ContentLayout, Header, SpaceBetween, Select, Flashbar, Spinner, Alert,
} from '@cloudscape-design/components';
import { useSpamFilter } from './useSpamFilter';
import { SpamFilterPolicyEditor } from './SpamFilterPolicyEditor';
import { SpamFilterEventsTable } from './SpamFilterEventsTable';
import { SpamFilterStats } from './SpamFilterStats';
import { SpamFilterEvent, EventFilter } from './spamFilterTypes';

export default function SpamFilterPage() {
  const sf = useSpamFilter();
  const [eventFilter, setEventFilter] = useState<EventFilter>('all');
  const [eventFrom, setEventFrom] = useState('');
  const [eventTo, setEventTo] = useState('');
  const [eventMinScore, setEventMinScore] = useState('');
  const [detailEvent, setDetailEvent] = useState<SpamFilterEvent | null>(null);

  if (sf.loading) return <Spinner />;

  return (
    <ContentLayout header={<Header>{sf.t('pages.spam_filter_page.title')}</Header>}>
      <SpaceBetween size="l">
        <Flashbar items={sf.notifications} />
        {/* scope selector */}
        <Select ... />
        <SpamFilterStats stats={sf.stats} lastUpdated={sf.lastUpdated} t={sf.t} />
        <SpamFilterPolicyEditor
          policy={sf.policy}
          onPolicyChange={sf.setPolicy}
          saving={sf.saving}
          isDirty={sf.isDirty}
          onSave={sf.handleSave}
          t={sf.t}
        />
        <SpamFilterEventsTable
          events={sf.events}
          eventFilter={eventFilter}
          onFilterChange={setEventFilter}
          eventFrom={eventFrom}
          onEventFromChange={setEventFrom}
          eventTo={eventTo}
          onEventToChange={setEventTo}
          eventMinScore={eventMinScore}
          onEventMinScoreChange={setEventMinScore}
          detailEvent={detailEvent}
          onSelectEvent={setDetailEvent}
          locale={sf.locale}
          t={sf.t}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
```

Adapt the actual JSX from the existing page — the above is a structural sketch, not verbatim. Preserve all existing Cloudscape markup exactly, just reorganize which file it lives in.

- [ ] **Step 7: Type-check and commit**

```bash
pnpm --dir apps/console type-check
```

Expected: exit 0.

```bash
git add apps/console/src/app/companies/\[id\]/security/spam-filter/
git commit -m "refactor(console): decompose spam-filter page into useSpamFilter hook + policy/events/stats components"
```

---

### Task 5: Decompose domains/[domainId] page (945 lines → orchestrator + tabs)

**Goal:** Break `apps/console/src/app/companies/[id]/domains/[domainId]/page.tsx` into a data hook and per-tab components matching the existing tabs: overview, users, settings, mail stats, and MCP policy.

**Files:**
- Create: `apps/console/src/app/companies/[id]/domains/[domainId]/domainDetailTypes.ts`
- Create: `apps/console/src/app/companies/[id]/domains/[domainId]/useDomainDetail.ts`
- Create: `apps/console/src/app/companies/[id]/domains/[domainId]/DomainOverviewTab.tsx`
- Create: `apps/console/src/app/companies/[id]/domains/[domainId]/DomainUsersTab.tsx`
- Create: `apps/console/src/app/companies/[id]/domains/[domainId]/DomainSettingsTab.tsx`
- Create: `apps/console/src/app/companies/[id]/domains/[domainId]/DomainStatsTab.tsx`
- Create: `apps/console/src/app/companies/[id]/domains/[domainId]/DomainMCPTab.tsx`
- Modify: `apps/console/src/app/companies/[id]/domains/[domainId]/page.tsx` (orchestrator, target ~120 lines)

**Acceptance Criteria:**
- [ ] `page.tsx` is ≤ 150 lines
- [ ] Each tab file contains only the JSX for that tab
- [ ] `useDomainDetail.ts` owns all fetch/mutation logic
- [ ] `pnpm --dir apps/console type-check` passes

**Verify:** `pnpm --dir apps/console type-check` → exit 0

**Steps:**

- [ ] **Step 1: Extract types to domainDetailTypes.ts**

Move all interface declarations from `page.tsx` (DomainDetail, User, DomainSetting, DailyCount, DomainMCPPolicy, DomainMCPPolicyConfig, etc.) plus helper functions `normalizeMCPPolicy` and `DEFAULT_MCP_POLICY` to:

```typescript
// apps/console/src/app/companies/[id]/domains/[domainId]/domainDetailTypes.ts
export interface DomainDetail { ... }
export interface User { ... }
export interface DomainSetting { ... }
export interface DailyCount { date: string; count: number; }
export interface DomainMCPPolicy { ... }
export interface DomainMCPPolicyConfig { ... }
export const DEFAULT_MCP_POLICY: DomainMCPPolicy = { ... };
export function normalizeMCPPolicy(raw: Partial<DomainMCPPolicy> | null): DomainMCPPolicy { ... }
```

Copy all definitions verbatim from `page.tsx`.

- [ ] **Step 2: Extract useDomainDetail.ts**

Move all state, effects, and mutations:

```typescript
// apps/console/src/app/companies/[id]/domains/[domainId]/useDomainDetail.ts
'use client';
import { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { DomainDetail, User, DomainSetting, DailyCount, DomainMCPPolicy, DomainMCPPolicyConfig, DEFAULT_MCP_POLICY, normalizeMCPPolicy } from './domainDetailTypes';

export function useDomainDetail() {
  const { t } = useI18n();
  const params = useParams();
  const router = useRouter();
  const companyId = params?.id as string;
  const domainId = params?.domainId as string;

  // All useState declarations
  // Initial useEffect (Promise.all fetch)
  // handleVerifyDNS
  // handleSaveEdit
  // handleDelete
  // Mail stats fetch (lazy, on tab activation)
  // MCP policy save

  return {
    t, companyId, domainId, router,
    domain, users, settings, loading, loadError,
    // edit modal
    showEdit, setShowEdit, editForm, setEditForm, saving, saveError, handleSaveEdit,
    // delete modal
    showDelete, setShowDelete, deleting, deleteError, handleDelete,
    // DNS verify
    verifying, handleVerifyDNS,
    // settings tab
    showAddSetting, setShowAddSetting, newSetting, setNewSetting, savingSetting, handleAddSetting,
    // stats tab
    mailStats, statsLoading, statsFetched, fetchMailStats,
    // MCP tab
    mcpPolicy, setMcpPolicy, mcpPolicyConfig, mcpPolicyLoading, mcpPolicySaving, mcpPolicyError, mcpPolicySaved, handleSaveMCPPolicy,
  };
}
```

Copy all function bodies verbatim from `page.tsx`. Do not change any fetch paths.

- [ ] **Step 3: Create DomainOverviewTab.tsx**

```typescript
// DomainOverviewTab.tsx
'use client';
import { Container, Header, ColumnLayout, Box, Button, StatusIndicator, SpaceBetween } from '@cloudscape-design/components';
import { DomainDetail } from './domainDetailTypes';

interface Props {
  domain: DomainDetail;
  verifying: boolean;
  onVerifyDNS: () => void;
  onEdit: () => void;
  onDelete: () => void;
  t: (key: string, ...args: unknown[]) => string;
}

export function DomainOverviewTab({ domain, verifying, onVerifyDNS, onEdit, onDelete, t }: Props) {
  // Move the overview Container JSX from page.tsx here verbatim
}
```

- [ ] **Step 4: Create DomainUsersTab.tsx**

```typescript
// DomainUsersTab.tsx — wraps the users DataTable for this domain
interface Props {
  users: User[];
  domainId: string;
  t: (key: string, ...args: unknown[]) => string;
}
export function DomainUsersTab({ users, domainId, t }: Props) { ... }
```

- [ ] **Step 5: Create DomainSettingsTab.tsx**

```typescript
// DomainSettingsTab.tsx — key/value config settings for this domain
interface Props {
  settings: DomainSetting[];
  showAddSetting: boolean;
  onShowAddSetting: (v: boolean) => void;
  newSetting: { key: string; value: string };
  onNewSettingChange: (s: { key: string; value: string }) => void;
  savingSetting: boolean;
  onAddSetting: () => void;
  t: (key: string, ...args: unknown[]) => string;
}
export function DomainSettingsTab(props: Props) { ... }
```

- [ ] **Step 6: Create DomainStatsTab.tsx**

```typescript
// DomainStatsTab.tsx — mail flow chart for this domain
interface Props {
  mailStats: DailyCount[];
  statsLoading: boolean;
  statsFetched: boolean;
  onFetchStats: () => void;
  t: (key: string, ...args: unknown[]) => string;
}
export function DomainStatsTab(props: Props) { ... }
```

- [ ] **Step 7: Create DomainMCPTab.tsx**

```typescript
// DomainMCPTab.tsx — MCP policy editor for this domain
interface Props {
  mcpPolicy: DomainMCPPolicy;
  mcpPolicyConfig: DomainMCPPolicyConfig | null;
  mcpPolicyLoading: boolean;
  mcpPolicySaving: boolean;
  mcpPolicyError: string;
  mcpPolicySaved: boolean;
  onPolicyChange: (p: DomainMCPPolicy) => void;
  onSave: () => void;
  t: (key: string, ...args: unknown[]) => string;
}
export function DomainMCPTab(props: Props) { ... }
```

- [ ] **Step 8: Rewrite page.tsx as orchestrator**

```typescript
// page.tsx — tab shell + modals only (~120 lines)
'use client';
import { useState } from 'react';
import { Tabs, Spinner, Alert, Modal, Button, FormField, Input, Select, SpaceBetween } from '@cloudscape-design/components';
import { useDomainDetail } from './useDomainDetail';
import { DomainOverviewTab } from './DomainOverviewTab';
import { DomainUsersTab } from './DomainUsersTab';
import { DomainSettingsTab } from './DomainSettingsTab';
import { DomainStatsTab } from './DomainStatsTab';
import { DomainMCPTab } from './DomainMCPTab';

export default function DomainDetailPage() {
  const d = useDomainDetail();
  const [activeTab, setActiveTab] = useState('overview');

  if (d.loading) return <Spinner />;
  if (d.loadError) return <Alert type="error">{d.loadError}</Alert>;
  if (!d.domain) return null;

  return (
    <>
      <Tabs
        activeTabId={activeTab}
        onChange={({ detail }) => {
          setActiveTab(detail.activeTabId);
          if (detail.activeTabId === 'stats' && !d.statsFetched) d.fetchMailStats();
        }}
        tabs={[
          { id: 'overview', label: d.t('...'), content: <DomainOverviewTab ... /> },
          { id: 'users',    label: d.t('...'), content: <DomainUsersTab ... /> },
          { id: 'settings', label: d.t('...'), content: <DomainSettingsTab ... /> },
          { id: 'stats',    label: d.t('...'), content: <DomainStatsTab ... /> },
          { id: 'mcp',      label: d.t('...'), content: <DomainMCPTab ... /> },
        ]}
      />
      {/* Edit modal */}
      {/* Delete confirmation modal */}
    </>
  );
}
```

The modals (Edit, Delete, AddSetting) stay in `page.tsx` since they are triggered by actions across multiple tabs.

- [ ] **Step 9: Type-check and commit**

```bash
pnpm --dir apps/console type-check
```

Expected: exit 0.

```bash
git add apps/console/src/app/companies/\[id\]/domains/\[domainId\]/
git commit -m "refactor(console): decompose domain detail page into useDomainDetail hook + per-tab components"
```

---

## Self-Review

**Spec coverage:**
- ✓ Repo hygiene: tsbuildinfo and .eml tracked files → Task 1
- ✓ Settings UI duplication: dead SettingsModal → Task 2
- ✓ SECURITY_REVIEW.md restructuring → Task 3
- ✓ Spam-filter page decomposition → Task 4
- ✓ Domains page decomposition → Task 5
- Note: `users/page.tsx` (705 lines) is a candidate for similar treatment but is lower priority. Omitted to keep this plan focused.

**Security remediation items:** All 8 items listed in the audit feedback are already implemented in the codebase. Task 3 ensures the documentation reflects this.

**Type consistency:** All component interfaces use the type names defined in the corresponding `*Types.ts` file. The `t` function type should match the actual `useI18n()` return type — verify during implementation.
