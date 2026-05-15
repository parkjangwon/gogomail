# TASK-076 Statistics & Dashboard Design

## Requirements Summary

TASK-076 is the first pass at a usable statistics layer for the admin console dashboard. The requested scope is intentionally compact: surface **mail volume**, **user activity**, and **storage** together so operators can judge tenant health without jumping between multiple pages.

The repository already has the raw sources for most of this data:

- mail flow aggregate queries and daily rollups in `internal/maildb/mail_flow_logs.go`
- company seat and storage usage in `internal/httpapi/admin.go` / `internal/admin`
- company security posture data in the console dashboard hook
- an existing dashboard page in `apps/console/src/app/companies/[id]/dashboard/page.tsx`
- a stubbed statistics hook in `apps/console/src/hooks/useStatistics.ts`

The best outcome is a single, reliable data composition path that the dashboard can use without introducing a large new API surface.

## Recommended Approach

### Hybrid summary composition

Use a small **dashboard statistics composer** in the console that pulls together the existing backend sources and normalizes them into one dashboard data object. Keep the backend unchanged unless a missing metric blocks the user-facing dashboard.

This gives us:

- one place to fix the currently stubbed statistics hook
- one shared contract for dashboard cards
- no duplication of query logic in the page component
- a natural seam for TASK-077 API metering later

### Why this is the best fit

The console already renders a dashboard, but its data is split across ad hoc calls and one stubbed hook. A composer keeps the implementation small, testable, and easy to evolve when TASK-078 reworks the dashboard UI.

## Alternatives Considered

### 1) Frontend-only ad hoc composition

Fetch each source directly inside the dashboard page.

- Pros: minimal backend work
- Cons: query logic spreads across the page, harder to test, stale stubbed statistics hook remains unused

### 2) New backend aggregate endpoint first

Add a dedicated `/admin/v1/companies/{id}/statistics` response and make the console consume it.

- Pros: clean contract, easy to cache server-side later
- Cons: more backend churn for a dashboard that already has the data sources, risk of overdesigning before TASK-077

### 3) Hybrid summary composition, recommended

Centralize request assembly and metric shaping in the console with existing endpoints, then upgrade to a backend aggregate endpoint only if the dashboard shows a real need.

- Pros: smallest safe change, testable, keeps the current roadmap unblocked
- Cons: multiple API calls still happen under the hood

## Data Model

The dashboard should present three first-class sections:

1. **Mail volume**
   - total messages in a recent time window
   - inbound / outbound / failed breakdown
   - daily trend for the same window

2. **User activity**
   - total users
   - active users
   - suspended users
   - optional login-audit signal if the existing route can provide it without extra backend complexity

3. **Storage**
   - used bytes
   - limit bytes
   - usage percentage
   - over-allocation flag

The existing health/security cards can stay as supporting information.

## Implementation Steps

1. Fix `apps/console/src/hooks/useStatistics.ts` so it uses the real mail-flow and seat-usage endpoints instead of the current stubbed return values.
2. Extract the dashboard composition logic into a focused helper module so the page stays thin and the transformation is unit-testable.
3. Update `apps/console/src/app/companies/[id]/dashboard/page.tsx` to show the three core stats sections clearly.
4. Add console-side unit tests for query construction and metric shaping.
5. Update translations for any new labels.
6. Refresh `docs/CURRENT_STATUS.md` and the roadmap/task checklist after implementation.

## Risks and Mitigations

- **Risk: too many API calls on dashboard load**
  - Mitigation: use a single query hook, sensible stale times, and keep the time window bounded.

- **Risk: user activity is too vague**
  - Mitigation: define it narrowly as seat usage first; only add login-audit counts if they can be done cheaply and reliably.

- **Risk: dashboard becomes visually crowded**
  - Mitigation: keep the first row to three cards, preserve the existing health/security row, and avoid new charts unless the data warrants it.

## Verification Plan

- `pnpm -C apps/console exec vitest run <new statistics test>`
- `pnpm -C apps/console type-check`
- `go test ./...`
- manual sanity check of the dashboard page for the default company context

## Decision

Proceed with the **hybrid summary composition** approach.

### Drivers

- minimal risk
- keeps the current dashboard usable
- avoids premature backend expansion
- aligns with the later dashboard/UI roadmap items

### Consequences

- the dashboard gets better immediately
- the data composition layer becomes reusable for later statistics screens
- we may still add a backend aggregate endpoint in a later task if the dashboard needs stricter caching or fewer round trips

### Follow-ups

- TASK-077 can build on the same metrics surface for metering
- TASK-078 can reuse the normalized dashboard data contract when refining the UI

