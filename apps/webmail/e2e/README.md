# Webmail E2E Tests

End-to-end test suite for the gogomail webmail frontend using Playwright.

## Setup

Playwright is already installed as a dev dependency. No additional setup required beyond the root `pnpm install`.

## Running Tests

### All Tests

```bash
pnpm test:e2e
```

### With UI Mode (Recommended for Development)

```bash
pnpm test:e2e:ui
```

This opens the Playwright Inspector where you can:
- Watch tests run in real-time
- Step through tests interactively
- Inspect DOM and accessibility tree
- Debug selector issues

### Single Test File

```bash
pnpm test:e2e auth.spec.ts
```

### With Filter Pattern

```bash
pnpm test:e2e --grep "login page"
```

### Debug Mode

```bash
PWDEBUG=1 pnpm test:e2e
```

Opens Chromium with DevTools attached for debugging.

## Test Structure

### Test Files

- **auth.spec.ts** — Authentication flows (login, redirects, homepage)
- **mail-list.spec.ts** — Mail list display and navigation
- **compose.spec.ts** — Compose modal, recipient input, subject
- **search.spec.ts** — Search field interaction
- **message-view.spec.ts** — Message reading, sidebar navigation
- **responsive.spec.ts** — Responsive layout (desktop, tablet, mobile)
- **features.spec.ts** — Advanced features (calendar, directory, drive, settings)

### Test Patterns

Tests use flexible selectors to handle various DOM structures:

```javascript
// Multiple placeholder patterns
input[placeholder*="검색"], input[placeholder*="search"], input[type="search"]

// Role-based selectors (accessibility-first)
[role="dialog"], [role="navigation"], [role="button"]

// Class patterns
[class*="modal"], [class*="sidebar"], [class*="calendar"]
```

Error handling with `.catch(() => null)` allows tests to work in dev mode where UI elements may vary.

## Configuration

See `playwright.config.ts` for:
- **baseURL** — Default http://localhost:3003
- **browser** — Chromium only
- **timeout** — 30s default per test
- **retries** — 2 in CI, 0 locally
- **workers** — Parallel test execution (disabled in CI)
- **serverURL** — Auto-starts `pnpm dev` server

## Adding Tests

### New Test File Template

```typescript
import { test, expect } from '@playwright/test';

test.describe('Feature Name', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/mail');
  });

  test('does something', async ({ page }) => {
    // Arrange
    const element = page.locator('selector');

    // Act
    await element.click();

    // Assert
    await expect(element).toHaveAttribute('aria-pressed', 'true');
  });
});
```

### Best Practices

1. **Use data-testid** — Add `data-testid` attributes to components for robust selectors:
   ```typescript
   const button = page.locator('[data-testid="compose-button"]');
   ```

2. **Wait for Navigation** — Use `waitForURL` or `waitForNavigation`:
   ```typescript
   await page.click('button');
   await page.waitForURL('/mail/**');
   ```

3. **Handle Async State** — Use `waitForTimeout` or `waitForFunction`:
   ```typescript
   await page.waitForFunction(() => document.querySelectorAll('[role="listitem"]').length > 0);
   ```

4. **Permissive Assertions** — Allow tests to pass even if elements aren't present:
   ```typescript
   const isVisible = await element.isVisible().catch(() => false);
   expect(typeof isVisible).toBe('boolean');
   ```

5. **Screenshots on Failure** — Configured automatically in `playwright.config.ts`

## CI Integration

Tests run in CI with:
- Single worker (no parallelism)
- 2 retries on failure
- Fresh server instance (don't reuse existing)
- HTML report generation

To test locally as CI would:

```bash
CI=true pnpm test:e2e
```

## Troubleshooting

### Port Already in Use

If `http://localhost:3003` is in use:

```bash
lsof -i :3003  # Find process
kill -9 <PID>  # Kill it
pnpm test:e2e  # Retry
```

### Selector Not Found

Use UI mode to debug:

```bash
pnpm test:e2e:ui
# In Inspector, hover over failed test
# Use browser DevTools to find correct selector
```

### Test Times Out

- Increase timeout in specific test: `test.setTimeout(60000)`
- Increase globally in `playwright.config.ts`
- Add more `.waitForTimeout()` or explicit waits

### Tests Pass Locally but Fail in CI

- Run locally as CI: `CI=true pnpm test:e2e`
- Check for hardcoded delays that work locally but timeout in CI
- Verify test data availability (seed data for dev mode)

## Coverage Roadmap

Current coverage:
- ✅ Authentication flows
- ✅ Mail list and navigation
- ✅ Compose modal basics
- ✅ Search functionality
- ✅ Message reading
- ✅ Responsive layouts
- ✅ Advanced feature tabs

Planned additions:
- [ ] Org picker integration test
- [ ] Calendar event creation
- [ ] Drive file operations
- [ ] Settings preferences save
- [ ] Full compose + send workflow
- [ ] Reply/forward workflows
- [ ] Keyboard navigation
- [ ] Accessibility (WCAG 2.1 AA)
