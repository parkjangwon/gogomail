# Webmail E2E Tests

End-to-end Playwright suite for the gogomail webmail (Next.js, port 3003).

## Running

```bash
# All projects (chromium, firefox, webkit)
pnpm test:e2e

# One browser (fast iteration)
pnpm test:e2e:chromium
pnpm test:e2e:firefox
pnpm test:e2e:webkit

# Headed browsers
pnpm test:e2e:headed

# Inspector UI
pnpm test:e2e:ui

# Single file
pnpm test:e2e auth.spec.ts

# Filter by name
pnpm test:e2e --grep "compose"
```

The Playwright config uses a `webServer` directive, so `pnpm dev` is launched
automatically if nothing is already listening on port 3003.

### Browser binaries

Browsers are downloaded by Playwright on first install. If a browser is
missing, run:

```bash
pnpm --dir apps/webmail exec playwright install chromium firefox webkit
# add --with-deps on Linux (may require sudo)
```

## Mock-based architecture

The webmail front-end calls Next.js proxy routes under `/api/mail/**` and
`/api/auth/**`. Every test installs a `page.route()` interceptor (see
`mocks.ts`) that returns canned JSON for these endpoints, so **no real
backend is required**.

Default fixture data is exported from `mocks.ts`:

- `DEFAULT_FOLDERS` — inbox/sent/drafts/spam/trash/archive
- `DEFAULT_MESSAGES` — three inbox messages
- `DEFAULT_USER`, `DEFAULT_PREFERENCES`, `DEFAULT_DRIVE_*`
- `makeMessage(id, overrides)`, `makeMessageDetail(id, overrides)` builders

### `setupAuthedPage(page, overrides?)`

Installs mocks, seeds localStorage auth flags, and navigates to `/mail`.
Use this in `beforeEach` for any authed test:

```ts
import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

test.beforeEach(async ({ page }) => {
  await setupAuthedPage(page);
});

test('inbox renders', async ({ page }) => {
  await expect(page.getByText('Welcome to gogomail')).toBeVisible();
});
```

Use `setupMocksOnly(page)` for tests that drive the login page or other
unauthenticated flows.

### Per-test mock overrides

Pass an `overrides` object to inject custom fixtures or extra route
handlers:

```ts
await setupAuthedPage(page, {
  messages: Array.from({ length: 50 }, (_, i) =>
    makeMessage(`m-${i}`, { subject: `Message ${i}` })
  ),
  extra: [
    {
      urlPattern: /\/api\/mail\/folders/,
      handler: (route) => route.fulfill({ status: 500, body: '{}' }),
    },
  ],
  unauthorized: true,         // makes every /api/mail/** return 401
});
```

Unmocked `/api/mail/**` routes return a 404 with a descriptive
`error_message` so missing mocks surface clearly in failures.

## Test files

| File                  | Coverage                                                                       |
|-----------------------|--------------------------------------------------------------------------------|
| `auth.spec.ts`        | login success / failure, forgot / reset password, redirects, 401 handling      |
| `mail-list.spec.ts`   | inbox render, folder nav, keyboard row navigation, bulk select, empty folder  |
| `compose.spec.ts`     | compose modal: open, To/Cc/Bcc, subject, body editor, send / close            |
| `message-view.spec.ts`| reading pane, HTML body, attachments, mark-read PATCH                          |
| `search.spec.ts`      | search input, typing, clear, network request, empty results                    |
| `drive.spec.ts`       | drive surface, listed nodes, upload-modal trigger, quota, persistence          |
| `drive-upload.spec.ts`| existing drive multi-file upload coverage (kept as-is)                         |
| `calendar.spec.ts`    | calendar surface, new-event flow (best-effort)                                 |
| `contacts.spec.ts`    | contacts surface (best-effort, skipped if entry not present)                   |
| `settings.spec.ts`    | settings panel open, nav items, theme/language reachable                       |
| `keyboard.spec.ts`    | `c`, `/`, `Esc`, `j`, `?` shortcuts (soft assertions)                          |
| `i18n.spec.ts`        | Korean default, locale preference plumbing                                     |
| `errors.spec.ts`      | 401 / 500 / network-abort resilience                                           |
| `responsive.spec.ts`  | desktop / tablet / mobile viewports, resize                                    |
| `features.spec.ts`    | top-level app-icon-bar navigation                                              |

## Adding a test

1. Create `<feature>.spec.ts` in `e2e/`.
2. Use `setupAuthedPage(page)` in `beforeEach`.
3. Prefer accessibility selectors (`getByRole`, `getByLabel`, `getByText`)
   over class- or DOM-structure-based ones.
4. For custom data, pass `overrides` to `setupAuthedPage`. For one-off
   endpoint behavior, add `extra: [{ urlPattern, handler }]`.
5. Avoid `waitForTimeout` for synchronization — use `expect(...).toBeVisible`
   with a generous timeout, or `page.waitForRequest(...)`.

## Configuration

See `playwright.config.ts`:

- Three browser projects: `chromium`, `firefox`, `webkit`.
- 30 s test timeout, 5 s default expect timeout.
- HTML report; `trace: 'on-first-retry'`; screenshots + video on failure.
- `webServer` auto-starts `pnpm dev` on port 3003.

## CI integration

```yaml
- run: pnpm install
- run: pnpm --dir apps/webmail exec playwright install --with-deps
- run: CI=true pnpm --dir apps/webmail test:e2e
```

In CI: `retries: 2`, `workers: 1`, server is started fresh (not reused).

## Troubleshooting

- **Unmocked endpoint** → look for `[mocks.ts] unmocked /api/mail route: ...`
  in the response body or trace; add the handler to `mocks.ts`.
- **Port 3003 already in use** → `lsof -i :3003 && kill <PID>`.
- **Flaky selectors** → switch to `getByRole` / `getByLabel`; widen the
  regex; add `.or(...)` fallbacks.
- **Test times out** → check the trace (HTML report has trace viewer).
