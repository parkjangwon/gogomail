# Admin console E2E tests (Playwright)

This suite exercises the admin console UI **without a real backend**.
All `/api/admin/**` calls are intercepted via `page.route()` and answered
with canned JSON from `mocks.ts`. Tests run in chromium, firefox, and
webkit projects (see `playwright.config.ts`).

## Running

From `apps/console/`:

```bash
pnpm test:e2e                # all browsers
pnpm test:e2e:chromium       # chromium only
pnpm test:e2e:firefox        # firefox only
pnpm test:e2e:webkit         # webkit only
pnpm test:e2e:headed         # open a real browser window
pnpm test:e2e:ui             # interactive Playwright UI runner
```

The dev server is launched automatically by Playwright via the
`webServer` config (`pnpm dev` on port 3001). On the first run it may
take ~30s for the Next.js dev server to warm up; subsequent runs reuse
the server when `CI` is not set.

If browsers aren't installed yet:

```bash
pnpm exec playwright install chromium firefox webkit
```

## How mocks work

- `mocks.ts` defines a single `installMocks(page, overrides?)` function
  that registers one big route handler matching `**/api/admin/**`.
- The handler dispatches by path segments and returns sensible defaults
  for every collection the admin console reads (companies, domains,
  users, mail flow logs, audit logs, alerts, storage, security policy,
  roles, etc.).
- Any unmocked path falls through to a generic `200 { ok:true, items:[],
  total:0 }` response so newly-added endpoints don't break the suite.
- Override defaults per-test by passing a `MockOverrides` object, e.g.
  `users: [...]`, `unauthorized: true`, or `extra: [{ urlPattern,
  handler }]` for one-off matchers.

## Writing a test

```ts
import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test('my page renders', async ({ page }) => {
  await setupAuthedAdminPage(page, {
    gotoPath: '/companies/default/my-page',
    users: [{ id: 'u1', email: 'x@y.com', /* ... */ }],
  });
  await expect(page.getByRole('heading').first()).toBeVisible();
});
```

`setupAuthedAdminPage(page, opts)` does three things:

1. Installs API mocks (with optional overrides)
2. Seeds `localStorage` + a session cookie so the app thinks it's authed
3. Navigates to `opts.gotoPath` (default: `/companies/default/dashboard`)

Use `setupMocksOnly(page)` instead when the test drives `/login` itself.

## CI

The `console-e2e` job in `.github/workflows/ci.yml` runs the full suite
on every push/PR. The HTML report is uploaded as an artifact, and on
failure the trace/screenshot bundle is uploaded too. Playwright browsers
are cached by `pnpm-lock.yaml` hash.

## File layout

| File                          | Coverage                                |
| ----------------------------- | --------------------------------------- |
| `mocks.ts`                    | Shared API mocks + fixture data         |
| `helpers.ts`                  | `setupAuthedAdminPage`, exports         |
| `auth.spec.ts`                | Login, MFA, logout, 401 redirects       |
| `companies.spec.ts`           | Companies list + detail                 |
| `company-dashboard.spec.ts`   | Per-company dashboard widgets           |
| `users.spec.ts`               | Users + admin-users lists               |
| `mail.spec.ts`                | Mail flow logs, outbox, trace, etc.     |
| `domains.spec.ts`             | Tenancy domain pages                    |
| `delivery.spec.ts`            | Delivery routes + trusted relays        |
| `audit-logs.spec.ts`          | Audit log list + filters                |
| `alerts.spec.ts`              | Alert rules pages                       |
| `storage.spec.ts`             | Quota + attachment + drive pages        |
| `security.spec.ts`            | All /security/* sub-pages               |
| `tenancy.spec.ts`             | All /tenancy/* sub-pages                |
| `compliance.spec.ts`          | Compliance + legal-holds                |
| `monitoring.spec.ts`          | Monitoring page                         |
| `roles.spec.ts`               | Roles list                              |
| `analytics.spec.ts`           | Analytics sub-pages                     |
| `reports.spec.ts`             | Reports                                 |
| `config.spec.ts`              | Company/domain/user config              |
| `organization.spec.ts`        | SSO, webhooks, SCIM, signature, etc.    |
| `system.spec.ts`              | Queue, backpressure, health             |
| `access.spec.ts`              | Directory, aliases, delegations, groups |
| `error-boundary.spec.ts`      | 404 / 401 / 500 resilience              |
| `i18n.spec.ts`                | ko/en/ja/zh-CN locale switching         |
| `responsive.spec.ts`          | Desktop / tablet / mobile viewports     |

The original `admin-console.spec.ts`, `e2e-admin-console.spec.ts`,
`menu-inventory.spec.ts`, and `verify-i18n.spec.ts` are preserved.
