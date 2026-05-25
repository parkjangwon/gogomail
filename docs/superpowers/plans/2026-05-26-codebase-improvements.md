# Codebase Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all defects found in the Phase 3 comprehensive codebase evaluation — accumulated AI-agent docs, committed generated files, container security hardening, oversized TypeScript and Go source files, and type naming ambiguity.

**Architecture:** Fixes proceed in risk order: data/config hygiene first (docs, .gitignore, Docker security), then mechanical TypeScript file splits (MCP tools, webmail API layer, UI components), then Go package refactoring within existing package boundaries. All Go splits stay within the same package so compilation remains trivially verified throughout.

**Tech Stack:** Go · TypeScript · Next.js · Docker Compose · @modelcontextprotocol/sdk · Zod

---

## File Map

| File | Action |
|------|--------|
| `docs/CURRENT_STATUS.md` | Truncate to compact summary (~100 lines) |
| `docs/NEXT_STEPS.md` | Truncate to compact summary (~50 lines) |
| `.gitignore` | Add `clients/typescript/index.ts` |
| `clients/typescript/README.md` | Create — documents generation workflow |
| `docker/docker-compose.monitoring.yml` | Harden Promtail: security_opt + read_only |
| `docker/docker-compose.dev.yml` | Same Promtail hardening |
| `apps/gogomail-user-mcp/src/tools/schemas.ts` | Create — shared Zod primitives |
| `apps/gogomail-user-mcp/src/tools/account.ts` | Create — account/preferences/misc tools |
| `apps/gogomail-user-mcp/src/tools/notifications.ts` | Create — push notification tools |
| `apps/gogomail-user-mcp/src/tools/mail.ts` | Create — mail + spam tools |
| `apps/gogomail-user-mcp/src/tools/dm.ts` | Create — DM tools |
| `apps/gogomail-user-mcp/src/tools/drive.ts` | Create — Drive tools |
| `apps/gogomail-user-mcp/src/tools/calendar.ts` | Create — Calendar tools |
| `apps/gogomail-user-mcp/src/tools/contacts.ts` | Create — Contacts + directory tools |
| `apps/gogomail-user-mcp/src/tools/index.ts` | Create — combines all domain modules |
| `apps/gogomail-user-mcp/src/tools.ts` | Modify — becomes re-export shim |
| `apps/gogomail-manage-mcp/src/tools/users.ts` | Create — user + principal tools |
| `apps/gogomail-manage-mcp/src/tools/company.ts` | Create — company + domain tools |
| `apps/gogomail-manage-mcp/src/tools/mail-ops.ts` | Create — delivery, queue, DLQ tools |
| `apps/gogomail-manage-mcp/src/tools/security.ts` | Create — spam, DKIM, policy tools |
| `apps/gogomail-manage-mcp/src/tools/system.ts` | Create — health, audit, sessions tools |
| `apps/gogomail-manage-mcp/src/tools/org.ts` | Create — org unit + membership tools |
| `apps/gogomail-manage-mcp/src/tools/gogomail.ts` | Modify — becomes re-export shim |
| `apps/webmail/src/lib/api/types.ts` | Create — shared interfaces (Folder, MessageSummary, etc.) |
| `apps/webmail/src/lib/api/auth.ts` | Create — auth operations |
| `apps/webmail/src/lib/api/mail.ts` | Create — mail operations (MailSendRequest replaces SendMessageRequest) |
| `apps/webmail/src/lib/api/dm.ts` | Create — DM operations |
| `apps/webmail/src/lib/api/drive.ts` | Create — Drive operations |
| `apps/webmail/src/lib/api/calendar.ts` | Create — Calendar operations |
| `apps/webmail/src/lib/api/contacts.ts` | Create — Contacts operations |
| `apps/webmail/src/lib/api/index.ts` | Create — re-exports all + backward compat aliases |
| `apps/webmail/src/lib/api.ts` | Modify — becomes re-export shim |
| `apps/webmail/src/components/compose/ComposeEditorToolbar.tsx` | Create — editor format buttons |
| `apps/webmail/src/components/compose/ComposeAttachmentPanel.tsx` | Create — attachment list + actions |
| `apps/webmail/src/components/ComposeModal.tsx` | Modify — import extracted sub-components |
| `apps/webmail/src/components/dm/DMRoomList.tsx` | Create — room list sidebar |
| `apps/webmail/src/components/dm/DMMessageList.tsx` | Create — messages + reaction strip |
| `apps/webmail/src/components/dm/DMComposer.tsx` | Create — message input area |
| `apps/webmail/src/components/DMPanel.tsx` | Modify — import extracted sub-components |
| `internal/httpapi/admin_middleware.go` | Create — HTTP middleware functions |
| `internal/httpapi/admin_types.go` | Create — interfaces, config types, option funcs |
| `internal/httpapi/admin_user.go` | Create — user/principal handlers |
| `internal/httpapi/admin_company.go` | Create — company/domain/org handlers |
| `internal/httpapi/admin_mail.go` | Create — mail flow/delivery/DLQ handlers |
| `internal/httpapi/admin_policy.go` | Create — policy config handlers (org, IP, retention, auth, session, rate-limit, DMARC, spam, routing, SSO, SMTP, webhook) |
| `internal/httpapi/admin_system.go` | Create — health, queue, audit, quota handlers |
| `internal/httpapi/admin.go` | Modify — keep only RegisterAdminRoutes + route wiring |
| `internal/app/admin_service_user.go` | Create — user/auth/session service methods |
| `internal/app/admin_service_directory.go` | Create — directory/delegation/group service methods |
| `internal/app/admin_service_storage.go` | Create — attachment/drive upload cleanup methods |
| `internal/app/admin_service_delivery.go` | Create — delivery/DLQ/backpressure service methods |
| `internal/app/admin_service_config.go` | Create — config/policy service methods |
| `internal/app/admin_service.go` | Modify — keep only adminService struct + constructor |

---

### Task 1: Reset accumulated AI-agent documentation

**Goal:** Truncate `docs/CURRENT_STATUS.md` (9119 lines) and `docs/NEXT_STEPS.md` (3392 lines) to compact summaries that reflect only the present state of the project.

**Files:**
- Modify: `docs/CURRENT_STATUS.md`
- Modify: `docs/NEXT_STEPS.md`
- Modify: `docs/ACTIVE_TASK.md`

**Acceptance Criteria:**
- [ ] `docs/CURRENT_STATUS.md` is ≤ 150 lines
- [ ] `docs/NEXT_STEPS.md` is ≤ 100 lines
- [ ] Both files open cleanly in a text editor without lag
- [ ] The last completed major milestone (DM + monitoring stack, May 2026) is documented in CURRENT_STATUS.md

**Verify:** `wc -l docs/CURRENT_STATUS.md docs/NEXT_STEPS.md` → both numbers ≤ 150 and ≤ 100 respectively.

**Steps:**

- [ ] **Step 1: Replace CURRENT_STATUS.md**

Replace the entire file with a compact summary of the current state. The full content of the new file:

```markdown
# gogomail current status

Last updated: 2026-05-26

## Platform summary

GoGoMail is a production-grade self-hosted email platform written in Go
(single binary, 24 runtime modes). Key capabilities:

- **Protocols**: SMTP (inbound/outbound), IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP
- **Mail security**: SPF, DKIM, DMARC, ARC, MTA-STS, TLS-RPT, DANE
- **Storage**: PostgreSQL (multi-tenant), Redis Streams (outbox), S3-compatible object storage
- **Auth**: JWT + TOTP MFA + refresh token rotation
- **Frontend**: Next.js webmail + admin console (TypeScript/TSX)
- **AI interface**: User MCP (123 tools) + Manage MCP (admin tools)
- **Monitoring**: Prometheus + Loki + Promtail + Grafana (provisioned dashboards)

## Recent milestones (2026-05)

| Date | Feature |
|------|---------|
| 2026-05-25 | DM search candidate limit lowered 10000 → 1000; system messages injectable |
| 2026-05-25 | Grafana default password removed; metrics interface{} replaced with typed interfaces |
| 2026-05-24 | User MCP DM tools (18 tools): rooms, messages, attachments, reactions, search |
| 2026-05-23 | Web push notifications + push device management |
| 2026-05-23 | Monitoring stack: Prometheus, Loki, Promtail, Grafana with provisioned dashboard |
| 2026-05-22 | Admin console MFA enforcement |
| 2026-05-21 | Multiple query sargability improvements, LDAP batching, i18n cleanup |

## Architecture

See `docs/backend-roadmap.md` for the full feature roadmap.
See `docs/openapi.yaml` for the REST API spec.
See `PROJECT_HARNESS.md` for development workflow.
```

```bash
# Replace the file
cat > docs/CURRENT_STATUS.md << 'HEREDOC'
# gogomail current status
...
HEREDOC
```

Write the exact content above to `docs/CURRENT_STATUS.md`.

- [ ] **Step 2: Replace NEXT_STEPS.md**

Replace the entire file:

```markdown
# gogomail next steps

Last updated: 2026-05-26

## Current focus

Code quality improvements per Phase 3 evaluation:
- TypeScript file splits (MCP tools, webmail API layer, UI components)
- Go package refactoring (httpapi/admin.go, app/admin_service.go)
- Documentation hygiene

## Backlog (priority order)

1. **OpenSearch integration** — replace in-memory mail search with OpenSearch FTS
2. **DM search scalability** — index encrypted tokens or move to separate unencrypted index
3. **SMTP rate limiting per recipient domain** — outbound throttle per MX
4. **Attachment virus scanning** — ClamAV or external scan hook
5. **Admin audit log retention** — configurable auto-expiry
6. **Mobile app** — React Native or PWA wrapper

## Out of scope (current sprint)

See `docs/backend-roadmap.md` for items intentionally deferred.
```

Write the exact content above to `docs/NEXT_STEPS.md`.

- [ ] **Step 3: Update ACTIVE_TASK.md**

Set `docs/ACTIVE_TASK.md` status to reflect the current improvement sprint (or `COMPLETE` if no active task).

- [ ] **Step 4: Commit**

```bash
git add docs/CURRENT_STATUS.md docs/NEXT_STEPS.md docs/ACTIVE_TASK.md
git commit -m "docs: reset accumulated AI-agent docs to compact summaries"
```

---

### Task 2: Add generated TypeScript client to .gitignore

**Goal:** Stop committing the auto-generated `clients/typescript/index.ts` (537 KB, 17,504 lines) to git and document how to regenerate it.

**Files:**
- Modify: `.gitignore`
- Create: `clients/typescript/README.md`

**Acceptance Criteria:**
- [ ] `git check-ignore -v clients/typescript/index.ts` outputs a match (the file is ignored)
- [ ] `make gen-ts-client` still generates the file correctly from `docs/openapi.yaml`
- [ ] `clients/typescript/README.md` explains how to regenerate
- [ ] `apps/console` still type-checks after `make gen-ts-client && pnpm --dir apps/console type-check`

**Verify:** `git check-ignore -v clients/typescript/index.ts` → prints `.gitignore:N:clients/typescript/index.ts` (non-empty output)

**Steps:**

- [ ] **Step 1: Add to .gitignore**

Add the following line to `.gitignore` (after the existing `clients/` section if present, or at the end):

```
# Auto-generated TypeScript client — regenerate with: make gen-ts-client
clients/typescript/index.ts
```

- [ ] **Step 2: Remove from git tracking**

```bash
git rm --cached clients/typescript/index.ts
```

If the file is not tracked, this is a no-op (exit 0 is acceptable).

- [ ] **Step 3: Create clients/typescript/README.md**

```markdown
# @gogomail/api-types

Auto-generated TypeScript types from the GoGoMail OpenAPI spec.

**This package's `index.ts` is not committed to git.** Generate it before building:

```bash
make gen-ts-client
```

This runs:
```bash
npx openapi-typescript docs/openapi.yaml \
  -o clients/typescript/index.ts \
  --enum \
  --export-type \
  --alphabetize
```

## CI / first-time setup

Run `make gen-ts-client` before `pnpm install` or any TypeScript build that
imports from `@gogomail/api-types`. The `apps/console` Next.js config already
transpiles this workspace package via `transpilePackages`.
```

- [ ] **Step 4: Commit**

```bash
git add .gitignore clients/typescript/README.md
git commit -m "chore: gitignore generated TS client, add regeneration docs"
```

---

### Task 3: Harden Promtail container security

**Goal:** Add `no-new-privileges` and read-only filesystem constraints to the Promtail container, which mounts the Docker socket (a root-equivalent capability), and document the security trade-off.

**Files:**
- Modify: `docker/docker-compose.monitoring.yml`
- Modify: `docker/docker-compose.dev.yml`

**Acceptance Criteria:**
- [ ] `promtail` service in `docker-compose.monitoring.yml` has `security_opt: ["no-new-privileges:true"]`
- [ ] `promtail` service has `read_only: true`
- [ ] A `tmpfs` mount covers `/tmp` (Promtail needs a writable temp dir)
- [ ] `docker compose -f docker/docker-compose.monitoring.yml config` exits 0 (valid YAML)
- [ ] Same constraints applied in `docker-compose.dev.yml`

**Verify:** `docker compose -f docker/docker-compose.monitoring.yml config 2>&1 | grep -c "no-new-privileges"` → `1`

**Steps:**

- [ ] **Step 1: Harden Promtail in docker-compose.monitoring.yml**

Find the `promtail:` service block in `docker/docker-compose.monitoring.yml`:

```yaml
  promtail:
    image: grafana/promtail:3.1.0
    container_name: gogomail-promtail
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - promtail-positions:/var/lib/promtail
      - ./promtail-config.yml:/etc/promtail/config.yml:ro
    command: -config.file=/etc/promtail/config.yml
    depends_on:
      - loki
    networks:
      - gogomail-monitoring
```

Replace with:

```yaml
  # Promtail — ships Docker container logs to Loki via Docker socket (read-only).
  # Security notes:
  #   - Docker socket grants root-equivalent read access to container metadata and logs.
  #   - Mitigations applied: read-only socket mount, no-new-privileges, read-only root FS.
  #   - Production alternative: use the Loki Docker log driver plugin so each container
  #     pushes its own logs and Promtail is not needed.
  #     Install with: docker plugin install grafana/loki-docker-driver:latest --alias loki
  promtail:
    image: grafana/promtail:3.1.0
    container_name: gogomail-promtail
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - promtail-positions:/var/lib/promtail
      - ./promtail-config.yml:/etc/promtail/config.yml:ro
    command: -config.file=/etc/promtail/config.yml
    depends_on:
      - loki
    networks:
      - gogomail-monitoring
```

- [ ] **Step 2: Apply the same change to docker-compose.dev.yml**

Find the `promtail:` block in `docker/docker-compose.dev.yml` and apply the same `security_opt`, `read_only`, and `tmpfs` additions plus the security comment.

- [ ] **Step 3: Validate compose files**

```bash
docker compose -f docker/docker-compose.monitoring.yml config > /dev/null
docker compose -f docker/docker-compose.dev.yml config > /dev/null
```

Both must exit 0.

- [ ] **Step 4: Commit**

```bash
git add docker/docker-compose.monitoring.yml docker/docker-compose.dev.yml
git commit -m "security: harden Promtail container (no-new-privileges, read-only FS)"
```

---

### Task 4: Split user-mcp tools.ts into domain modules

**Goal:** Break `apps/gogomail-user-mcp/src/tools.ts` (1,039 lines, 123 tools) into 7 domain files + 1 shared schema file + 1 index combiner, keeping `src/tools.ts` as a one-line re-export for backward compatibility.

**Files:**
- Create: `apps/gogomail-user-mcp/src/tools/schemas.ts`
- Create: `apps/gogomail-user-mcp/src/tools/account.ts` (~10 tools)
- Create: `apps/gogomail-user-mcp/src/tools/notifications.ts` (~9 tools)
- Create: `apps/gogomail-user-mcp/src/tools/mail.ts` (~35 tools incl. spam)
- Create: `apps/gogomail-user-mcp/src/tools/dm.ts` (~19 tools)
- Create: `apps/gogomail-user-mcp/src/tools/drive.ts` (~20 tools)
- Create: `apps/gogomail-user-mcp/src/tools/calendar.ts` (~9 tools)
- Create: `apps/gogomail-user-mcp/src/tools/contacts.ts` (~8 tools incl. directory)
- Create: `apps/gogomail-user-mcp/src/tools/index.ts`
- Modify: `apps/gogomail-user-mcp/src/tools.ts`

**Acceptance Criteria:**
- [ ] `apps/gogomail-user-mcp/src/tools.ts` is ≤ 5 lines (re-export only)
- [ ] Each domain file is ≤ 200 lines
- [ ] `npm run type-check` in `apps/gogomail-user-mcp` passes
- [ ] `npm test` in `apps/gogomail-user-mcp` passes
- [ ] `npm run build` in `apps/gogomail-user-mcp` passes

**Verify:** `npm run type-check && npm test && npm run build` in `apps/gogomail-user-mcp` → all exit 0

**Steps:**

- [ ] **Step 1: Create shared schemas file**

Create `apps/gogomail-user-mcp/src/tools/schemas.ts` with all shared Zod primitives extracted from the top of `tools.ts` (lines 8-41):

```typescript
import { z } from "zod";

export const id = z.string().trim().min(1).max(200).regex(/^[^\r\n]+$/);
export const optionalID = id.optional();
export const email = z.string().trim().email().max(320);
export const limit = z.number().int().min(1).max(200).optional();
export const address = z.object({ email, name: z.string().max(200).optional() });
export const confirm = z.string().max(300).optional();
export const storageBackend = z.string().trim().min(1).max(64).regex(/^[^\r\n]+$/);
export const contractName = z.string().min(1).max(200);
export const nameOrLegacyDisplayName = z.object({
  name: contractName.optional(),
  display_name: contractName.optional(),
  description: z.string().max(1000).optional(),
}).refine((value) => value.name || value.display_name, { message: "name is required" });
export const outputPath = z.string().trim().min(1).max(4096).regex(/^[^\r\n]+$/);
export const mailFlag = z.enum(["read", "starred", "answered", "forwarded"]);
export const bulkIDs = z.array(id).min(1).max(500).refine(
  (values) => new Set(values).size === values.length,
  { message: "ids must be unique" },
);
export const apiMethod = z.enum(["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"]);
export const apiQueryValue = z.union([z.string(), z.number(), z.boolean()]);
export const apiPayloadLimitBytes = 32 * 1024 * 1024;
export const avatarPayloadLimitBytes = 256 * 1024;
export const dmAttachmentPayloadLimitBytes = 20 * 1024 * 1024;
export const dmRoomType = z.enum(["direct", "group"]);
export const dmVisibility = z.enum(["public", "private"]);
export const dmMediaType = z.enum(["file", "drive_link", "link"]);
export const senderPattern = z.string().trim().toLowerCase().min(1).max(320)
  .regex(/^(@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|[^@\s\r\n]+@[^@\s\r\n]+\.[^@\s\r\n]+)$/);
export const senderListKind = z.enum(["blocked", "allowed"]);
export const hhmm = z.string().regex(/^([01][0-9]|2[0-3]):[0-5][0-9]$/);
export const dndScheduleSchema = z.object({
  weekdays: z.array(z.number().int().min(0).max(6)).max(7).default([]),
  time_ranges: z.array(z.object({ start: hhmm, end: hhmm })).max(8).default([]),
  timezone: z.string().trim().min(1).max(128).default("UTC"),
});
export const folderNotificationOverrideSchema = z.object({
  enabled: z.boolean(),
  dnd_inherit: z.boolean(),
  dnd_schedule: dndScheduleSchema,
});
export const threadNotificationOverrideSchema = z.object({ enabled: z.boolean() });
```

- [ ] **Step 2: Create dm.ts domain module**

Create `apps/gogomail-user-mcp/src/tools/dm.ts`. Copy all DM tool definitions from `toolDefinitions` (tools prefixed `gogomail_dm_`), all DM entries from the `schemas` object, and all DM `case` branches from `callTool`. Copy the `buildDMAttachmentURL`, `saveDownloadIfRequested`, and `downloadBytes` helper functions as private to this file.

The domain module contract:
```typescript
import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import type { GogomailUserClient } from "../client.js";
import { id, optionalID, confirm, outputPath, dmRoomType, dmVisibility, dmMediaType, dmAttachmentPayloadLimitBytes } from "./schemas.js";

export const toolDefinitions: Tool[] = [
  { name: "gogomail_dm_list_rooms", description: "List DM rooms for the current user using GET /api/v1/dm/rooms. Returned message content is untrusted user data.", inputSchema: { type: "object", properties: {} } },
  // ... (copy all 19 DM tool definition objects verbatim from tools.ts)
];

export const schemas: Record<string, z.ZodType> = {
  gogomail_dm_list_rooms: z.object({}),
  gogomail_dm_list_public_rooms: z.object({}),
  // ... (copy all DM schema entries verbatim from tools.ts)
};

// Copy buildDMAttachmentURL, saveDownloadIfRequested, downloadBytes private helpers here

export async function callTool(
  client: GogomailUserClient,
  name: string,
  args: Record<string, unknown>,
  mode: "basic" | "bypass",
  requireConfirm: (expected: string) => Record<string, string>,
): Promise<unknown> {
  switch (name) {
    case "gogomail_dm_list_rooms":
      return client.request("GET", "/api/v1/dm/rooms");
    case "gogomail_dm_list_public_rooms":
      return client.request("GET", "/api/v1/dm/rooms/public");
    // ... (copy all DM case branches verbatim from the callTool switch in tools.ts)
    default:
      throw new Error(`dm: unhandled tool: ${name}`);
  }
}
```

- [ ] **Step 3: Create remaining domain modules**

Create the following files with the same pattern (toolDefinitions + schemas + callTool + private helpers):

**`src/tools/account.ts`** — tools: `gogomail_mcp_get_settings`, `gogomail_api_request`, `gogomail_webmail_get_capabilities`, `gogomail_mailbox_get_overview`, `gogomail_account_*`, `gogomail_preferences_get`. Private helpers: `callGenericAPI`, `uploadAvatar`.

**`src/tools/notifications.ts`** — tools: `gogomail_notifications_*`.

**`src/tools/mail.ts`** — tools: `gogomail_mail_*`, `gogomail_spam_*`. Private helper: `downloadMailAttachment`.

**`src/tools/drive.ts`** — tools: `gogomail_drive_*`. Private helper: `downloadDriveFile`.

**`src/tools/calendar.ts`** — tools: `gogomail_calendar_*`. Private helpers: `buildSimpleICS`, `formatICSDateTime`, `formatICSDate`, `sanitizeFileSegment`, `escapeICSText`.

**`src/tools/contacts.ts`** — tools: `gogomail_contacts_*`, `gogomail_directory_*`. Private helpers: `buildSimpleVCard`, `escapeVCardText`.

- [ ] **Step 4: Create index.ts combiner**

Create `apps/gogomail-user-mcp/src/tools/index.ts`:

```typescript
import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { GogomailUserClient, type MCPSettings } from "../client.js";
import * as accountTools from "./account.js";
import * as notifTools from "./notifications.js";
import * as mailTools from "./mail.js";
import * as dmTools from "./dm.js";
import * as driveTools from "./drive.js";
import * as calendarTools from "./calendar.js";
import * as contactsTools from "./contacts.js";

const domainModules = [
  accountTools,
  notifTools,
  mailTools,
  dmTools,
  driveTools,
  calendarTools,
  contactsTools,
] as const;

export const toolDefinitions: Tool[] = domainModules.flatMap((m) => m.toolDefinitions);

const allSchemas: Record<string, z.ZodType> = Object.assign(
  {},
  ...domainModules.map((m) => m.schemas),
);

export async function callTool(
  client: GogomailUserClient,
  name: string,
  rawArgs: Record<string, unknown>,
  envMode: "basic" | "bypass",
): Promise<unknown> {
  const schema = allSchemas[name];
  if (!schema) throw new Error(`Unknown tool: ${name}`);
  const args = schema.parse(rawArgs) as Record<string, unknown>;
  const settings: MCPSettings = await client.settings().catch(() => ({}));
  const mode = settings.permission_mode ?? envMode;
  const requireConfirm = (expected: string): Record<string, string> => {
    if (mode === "bypass") return {};
    if (args.confirm !== expected)
      throw new Error(`confirmation required: confirm must equal "${expected}"`);
    return { "X-Gogomail-MCP-Confirm": expected };
  };

  for (const domain of domainModules) {
    if (name in domain.schemas) {
      return domain.callTool(client, name, args, mode, requireConfirm);
    }
  }
  throw new Error(`Unhandled tool: ${name}`);
}
```

- [ ] **Step 5: Update tools.ts to re-export**

Replace `apps/gogomail-user-mcp/src/tools.ts` entirely with:

```typescript
// Domain modules are in ./tools/ — this file re-exports for backward compatibility.
export { toolDefinitions, callTool } from "./tools/index.js";
```

- [ ] **Step 6: Type-check and test**

```bash
cd apps/gogomail-user-mcp
npm run type-check
npm test
npm run build
```

All must exit 0. Fix any import path errors or missing exports until they do.

- [ ] **Step 7: Commit**

```bash
git add apps/gogomail-user-mcp/src/tools/
git add apps/gogomail-user-mcp/src/tools.ts
git commit -m "refactor(user-mcp): split tools.ts into domain modules"
```

---

### Task 5: Split manage-mcp tools/gogomail.ts into domain modules

**Goal:** Break `apps/gogomail-manage-mcp/src/tools/gogomail.ts` (1,579 lines) into 6 domain files + index combiner, keeping `gogomail.ts` as a re-export shim.

**Files:**
- Create: `apps/gogomail-manage-mcp/src/tools/users.ts`
- Create: `apps/gogomail-manage-mcp/src/tools/company.ts`
- Create: `apps/gogomail-manage-mcp/src/tools/mail-ops.ts`
- Create: `apps/gogomail-manage-mcp/src/tools/security.ts`
- Create: `apps/gogomail-manage-mcp/src/tools/org.ts`
- Create: `apps/gogomail-manage-mcp/src/tools/system.ts`
- Create: `apps/gogomail-manage-mcp/src/tools/gogomail-index.ts`
- Modify: `apps/gogomail-manage-mcp/src/tools/gogomail.ts`

**Acceptance Criteria:**
- [ ] `apps/gogomail-manage-mcp/src/tools/gogomail.ts` is ≤ 5 lines (re-export only)
- [ ] Each domain file is ≤ 300 lines
- [ ] `npm run type-check` in `apps/gogomail-manage-mcp` passes
- [ ] `npm test` in `apps/gogomail-manage-mcp` passes

**Verify:** `npm run type-check && npm test` in `apps/gogomail-manage-mcp` → both exit 0

**Steps:**

- [ ] **Step 1: Identify tool groupings**

Read `apps/gogomail-manage-mcp/src/tools/gogomail.ts` and group the tools by domain:

| Domain file | Tool name prefixes / tool names |
|-------------|--------------------------------|
| `users.ts` | `gogomail_search_principals`, `gogomail_list_users`, `gogomail_get_user`, `gogomail_get_user_quota`, `gogomail_create_user`, `gogomail_delete_user`, `gogomail_update_user_*`, `gogomail_send_invite_email` |
| `company.ts` | `gogomail_list_companies`, `gogomail_get_company`, `gogomail_list_domains`, `gogomail_get_domain_settings`, `gogomail_check_domain_dns`, `gogomail_update_domain_settings` |
| `mail-ops.ts` | `gogomail_list_mail_flow_logs`, `gogomail_get_mail_flow_stats`, `gogomail_list_delivery_attempts`, `gogomail_list_exhausted_deliveries`, `gogomail_list_dlq`, `gogomail_delete_dlq_entry`, `gogomail_retry_outbox`, `gogomail_list_suppression_list`, `gogomail_remove_suppression_entry`, `gogomail_list_quota_usage`, `gogomail_list_quota_alerts` |
| `security.ts` | `gogomail_get_spam_filter`, `gogomail_get_spam_filter_events`, `gogomail_list_dkim_keys`, `gogomail_get_spam_filter_policy`, `gogomail_update_spam_filter_policy`, `gogomail_get_spam_filter_stats`, `gogomail_list_spam_filter_events`, `gogomail_get_security_policy`, `gogomail_update_security_policy` |
| `org.ts` | `gogomail_list_org_units`, `gogomail_get_org_hierarchy`, `gogomail_list_user_org_memberships`, `gogomail_assign_user_org_membership`, `gogomail_update_org_membership`, `gogomail_remove_org_membership` |
| `system.ts` | `gogomail_check_health`, `gogomail_get_queue_stats`, `gogomail_admin_api_request`, `gogomail_get_alert_events`, `gogomail_get_audit_logs`, `gogomail_list_company_sessions`, `gogomail_revoke_company_session` |

- [ ] **Step 2: Create each domain module**

Each domain file exports `toolDefinitions: Tool[]`, `callTool(client, suppo, name, args): Promise<unknown>`. Private helpers stay in their domain file.

```typescript
// users.ts
import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import type { OptionalSuppo } from "./gogomail.js";
// ... shared helpers (singleLine, etc.) live in a tools/shared.ts or are copied per file

export const toolDefinitions: Tool[] = [
  // copy gogomail_search_principals, gogomail_list_users, etc.
];

export const schemas: Record<string, z.ZodType> = {
  gogomail_search_principals: z.object({...}),
  // ...
};

export async function callTool(
  client: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  switch (name) {
    case "gogomail_search_principals":
      return client.request("GET", `/admin/v1/principals`, args);
    // ...
    default:
      throw new Error(`users: unhandled tool: ${name}`);
  }
}
```

- [ ] **Step 3: Create gogomail-index.ts combiner**

```typescript
// apps/gogomail-manage-mcp/src/tools/gogomail-index.ts
import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import type { OptionalSuppo } from "./gogomail.js";
import * as usersTools from "./users.js";
import * as companyTools from "./company.js";
import * as mailOpsTools from "./mail-ops.js";
import * as securityTools from "./security.js";
import * as orgTools from "./org.js";
import * as systemTools from "./system.js";

const domainModules = [usersTools, companyTools, mailOpsTools, securityTools, orgTools, systemTools] as const;

export const toolDefinitions: Tool[] = domainModules.flatMap((m) => m.toolDefinitions);
const allSchemas: Record<string, z.ZodType> = Object.assign({}, ...domainModules.map((m) => m.schemas));

export async function callTool(
  client: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  rawArgs: Record<string, unknown>,
): Promise<unknown> {
  const schema = allSchemas[name];
  if (!schema) throw new Error(`Unknown tool: ${name}`);
  const args = schema.parse(rawArgs) as Record<string, unknown>;
  for (const domain of domainModules) {
    if (name in domain.schemas) {
      return domain.callTool(client, suppo, name, args);
    }
  }
  throw new Error(`Unhandled tool: ${name}`);
}
```

- [ ] **Step 4: Update gogomail.ts to re-export**

```typescript
// Re-exports domain modules — actual implementations in ./users.ts, ./company.ts, etc.
export type { OptionalSuppo } from "./gogomail.js"; // keep type if needed
export { toolDefinitions, callTool } from "./gogomail-index.js";
```

Wait — `OptionalSuppo` is defined in the original `gogomail.ts`. Move its definition to `gogomail-index.ts` or a `types.ts` file so there's no circular dependency. The `gogomail.ts` re-export shim becomes:

```typescript
export { toolDefinitions, callTool } from "./gogomail-index.js";
export type { OptionalSuppo } from "./gogomail-index.js";
```

And `gogomail-index.ts` exports `OptionalSuppo`:
```typescript
import type { SuppoClient } from "../clients/suppo.js";
export type OptionalSuppo = SuppoClient | null;
```

- [ ] **Step 5: Verify the index.ts still works**

Check `apps/gogomail-manage-mcp/src/index.ts` — it imports `* as gogomailTools from "./tools/gogomail.js"` and calls `gogomailTools.toolDefinitions` and `gogomailTools.callTool`. The re-export shim preserves these names, so no change needed in `index.ts`.

- [ ] **Step 6: Type-check and test**

```bash
cd apps/gogomail-manage-mcp
npm run type-check
npm test
```

Both must exit 0.

- [ ] **Step 7: Commit**

```bash
git add apps/gogomail-manage-mcp/src/tools/
git commit -m "refactor(manage-mcp): split gogomail.ts into domain modules"
```

---

### Task 6: Split webmail/src/lib/api.ts by domain + fix naming

**Goal:** Break `apps/webmail/src/lib/api.ts` (2,116 lines) into 7 domain files + index re-exporter. Rename `SendMessageRequest` → `MailSendRequest` to eliminate ambiguity when `dm.ts` also has message-send types in the same API layer.

**Files:**
- Create: `apps/webmail/src/lib/api/types.ts`
- Create: `apps/webmail/src/lib/api/auth.ts`
- Create: `apps/webmail/src/lib/api/mail.ts`
- Create: `apps/webmail/src/lib/api/dm.ts`
- Create: `apps/webmail/src/lib/api/drive.ts`
- Create: `apps/webmail/src/lib/api/calendar.ts`
- Create: `apps/webmail/src/lib/api/contacts.ts`
- Create: `apps/webmail/src/lib/api/index.ts`
- Modify: `apps/webmail/src/lib/api.ts`

**Acceptance Criteria:**
- [ ] `apps/webmail/src/lib/api.ts` is ≤ 5 lines
- [ ] `MailSendRequest` is the canonical name (exported from `api/mail.ts` and re-exported as `SendMessageRequest` for backward compat from `api/index.ts`)
- [ ] `pnpm --dir apps/webmail type-check` passes
- [ ] No file in `apps/webmail/src/lib/api/` exceeds 400 lines

**Verify:** `pnpm --dir apps/webmail type-check` → exit 0

**Steps:**

- [ ] **Step 1: Create api/types.ts with shared interfaces**

Create `apps/webmail/src/lib/api/types.ts`:

```typescript
// Shared types used across multiple API domains
export interface Folder {
  id: string;
  parent_id?: string;
  name: string;
  full_path: string;
  type: string;
  system_type?: string;
  order_index: number;
  total: number;
  unread: number;
  starred: number;
}

// Copy MessageAddress, MessageSummary, MessageDetail, Attachment, TrackingEvent interfaces verbatim
// Copy ComposeIntent, UIComposeIntent type aliases
// Copy FolderStats interface
// Copy AuthTokenResponse, MFAVerifyResponse, MFAStatus, MFASetupResponse interfaces
```

- [ ] **Step 2: Create api/mail.ts**

Create `apps/webmail/src/lib/api/mail.ts`. Key rename: `SendMessageRequest` → `MailSendRequest`:

```typescript
import type { Folder, MessageSummary, MessageDetail, Attachment, TrackingEvent, FolderStats, UIComposeIntent, ComposeIntent } from './types.js';

// RENAMED: was SendMessageRequest — avoids ambiguity with DM send operations
export interface MailSendRequest {
  to?: MessageAddress[];
  cc?: MessageAddress[];
  bcc?: MessageAddress[];
  subject?: string;
  text_body?: string;
  html_body?: string;
  intent?: ComposeIntent;
  source_message_id?: string;
  attachment_ids?: string[];
}
// Copy SendMessageResult, SendMessageEnvelope, SearchParams, GetMessagesOptions, DraftData
// Copy all mail functions: searchMessages, getMessages, sendMessage, saveDraft, etc.
```

- [ ] **Step 3: Create remaining domain files**

**`api/auth.ts`**: `AuthTokenResponse`, `MFAVerifyResponse`, `MFAStatus`, `MFASetupResponse` + auth/MFA functions.

**`api/dm.ts`**: `DMUser`, `DMRoom`, `DMMessage`, `DMReaction`, `DMMediaItem` + DM functions (`createDMRoom`, `listDMRooms`, `sendDMMessage`, etc.).

**`api/drive.ts`**: `DriveNode` + drive functions.

**`api/calendar.ts`**: Calendar types + calendar functions.

**`api/contacts.ts`**: `ContactSuggestion` + contacts/directory functions.

- [ ] **Step 4: Create api/index.ts with backward compat alias**

Create `apps/webmail/src/lib/api/index.ts`:

```typescript
export * from './types.js';
export * from './auth.js';
export * from './mail.js';
export * from './dm.js';
export * from './drive.js';
export * from './calendar.js';
export * from './contacts.js';

// Backward compat alias — remove once all call sites use MailSendRequest
export type { MailSendRequest as SendMessageRequest } from './mail.js';
```

- [ ] **Step 5: Replace api.ts with re-export shim**

```typescript
// Split into domain modules — see lib/api/*.ts
// This file re-exports everything for backward compatibility.
export * from './api/index.js';
```

Note: Next.js imports `@/lib/api` with a file extension match. The re-export preserves all existing import paths (`import { ... } from '@/lib/api'`) without changes.

- [ ] **Step 6: Update ComposeModal.tsx import**

In `apps/webmail/src/components/ComposeModal.tsx`, change:

```typescript
// Before
import type { ..., SendMessageRequest, ... } from '@/lib/api';
// After (use the canonical name)
import type { ..., MailSendRequest, ... } from '@/lib/api';
```

Update all uses of `SendMessageRequest` in that file to `MailSendRequest`.

- [ ] **Step 7: Type-check**

```bash
pnpm --dir apps/webmail type-check
```

Fix any import resolution errors. Typical issue: relative imports within the new `api/` files must use `.js` extensions for ESM compatibility if the project uses `"moduleResolution": "bundler"` or `"node16"`.

- [ ] **Step 8: Commit**

```bash
git add apps/webmail/src/lib/api/ apps/webmail/src/lib/api.ts apps/webmail/src/components/ComposeModal.tsx
git commit -m "refactor(webmail): split api.ts by domain, rename SendMessageRequest→MailSendRequest"
```

---

### Task 7: Split ComposeModal.tsx — extract editor toolbar and attachment panel

**Goal:** Reduce `apps/webmail/src/components/ComposeModal.tsx` (1,455 lines) by extracting the rich text editor toolbar buttons and the attachment list/actions into dedicated components.

**Files:**
- Create: `apps/webmail/src/components/compose/ComposeEditorToolbar.tsx`
- Create: `apps/webmail/src/components/compose/ComposeAttachmentPanel.tsx`
- Modify: `apps/webmail/src/components/ComposeModal.tsx`

**Acceptance Criteria:**
- [ ] `ComposeModal.tsx` is ≤ 900 lines
- [ ] `ComposeEditorToolbar.tsx` renders the format buttons (bold, italic, underline, align, link, lists, image)
- [ ] `ComposeAttachmentPanel.tsx` renders the attachment list with remove/progress UI
- [ ] `pnpm --dir apps/webmail type-check` passes
- [ ] The compose window works correctly in the browser (visually: toolbar buttons fire, attachments list/remove)

**Verify:** `pnpm --dir apps/webmail type-check` → exit 0

**Steps:**

- [ ] **Step 1: Read ComposeModal.tsx and identify extractable sections**

Read the file. Find:
1. The editor format toolbar JSX (look for `<button onClick={() => editor?.chain().focus().toggleBold()...>` patterns — these are the formatting buttons)
2. The attachment list JSX (look for `attachments.map(...)` render section)

Note the props these sections need from the parent.

- [ ] **Step 2: Create ComposeEditorToolbar.tsx**

The toolbar needs the TipTap `editor` instance. Create:

```typescript
'use client';

import { useTranslations } from 'next-intl';
import type { Editor } from '@tiptap/react';
import {
  // Import the heroicons used by toolbar buttons
} from '@heroicons/react/24/outline';

interface ComposeEditorToolbarProps {
  editor: Editor | null;
  disabled?: boolean;
}

export function ComposeEditorToolbar({ editor, disabled = false }: ComposeEditorToolbarProps) {
  const t = useTranslations('composeFull');
  if (!editor) return null;

  return (
    <div className="flex items-center gap-1 px-2 py-1 border-t border-gray-200 dark:border-gray-700 flex-wrap">
      {/* Bold */}
      <button
        type="button"
        title={t('bold')}
        disabled={disabled}
        onClick={() => editor.chain().focus().toggleBold().run()}
        className={`p-1 rounded text-sm ${editor.isActive('bold') ? 'bg-gray-200 dark:bg-gray-600' : 'hover:bg-gray-100 dark:hover:bg-gray-700'}`}
      >
        <strong>B</strong>
      </button>
      {/* ... copy all other toolbar buttons from ComposeModal.tsx */}
    </div>
  );
}
```

Move all toolbar button JSX from `ComposeModal.tsx` into this component.

- [ ] **Step 3: Create ComposeAttachmentPanel.tsx**

```typescript
'use client';

import { useTranslations } from 'next-intl';
import { PaperClipIcon, XMarkIcon } from '@heroicons/react/24/outline';

interface Attachment {
  id: string;
  name: string;
  size: number;
  uploading?: boolean;
  progress?: number;
  error?: string;
}

interface ComposeAttachmentPanelProps {
  attachments: Attachment[];
  onRemove: (id: string) => void;
  disabled?: boolean;
}

export function ComposeAttachmentPanel({ attachments, onRemove, disabled = false }: ComposeAttachmentPanelProps) {
  const t = useTranslations('composeFull');
  if (attachments.length === 0) return null;

  return (
    <div className="px-3 py-2 border-t border-gray-200 dark:border-gray-700">
      <ul className="space-y-1">
        {attachments.map((att) => (
          <li key={att.id} className="flex items-center gap-2 text-sm">
            <PaperClipIcon className="h-4 w-4 text-gray-400 shrink-0" />
            <span className="flex-1 truncate">{att.name}</span>
            {att.uploading && (
              <span className="text-xs text-gray-500">{att.progress ?? 0}%</span>
            )}
            {att.error && (
              <span className="text-xs text-red-500">{att.error}</span>
            )}
            <button
              type="button"
              disabled={disabled || att.uploading}
              onClick={() => onRemove(att.id)}
              className="p-0.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700"
              aria-label={t('removeAttachment', { name: att.name })}
            >
              <XMarkIcon className="h-3.5 w-3.5" />
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
```

Match the `Attachment` type fields to what `ComposeModal.tsx` actually uses internally.

- [ ] **Step 4: Import and use in ComposeModal.tsx**

Add imports:
```typescript
import { ComposeEditorToolbar } from './compose/ComposeEditorToolbar';
import { ComposeAttachmentPanel } from './compose/ComposeAttachmentPanel';
```

Replace the inline toolbar JSX with `<ComposeEditorToolbar editor={editor} disabled={sending} />`.

Replace the inline attachment list JSX with `<ComposeAttachmentPanel attachments={attachments} onRemove={removeAttachment} disabled={sending} />`.

- [ ] **Step 5: Type-check**

```bash
pnpm --dir apps/webmail type-check
```

Fix prop type mismatches (use the exact internal state types from ComposeModal.tsx for attachment).

- [ ] **Step 6: Commit**

```bash
git add apps/webmail/src/components/compose/ComposeEditorToolbar.tsx
git add apps/webmail/src/components/compose/ComposeAttachmentPanel.tsx
git add apps/webmail/src/components/ComposeModal.tsx
git commit -m "refactor(webmail): extract ComposeEditorToolbar and ComposeAttachmentPanel"
```

---

### Task 8: Split DMPanel.tsx — extract room list, message list, and composer

**Goal:** Reduce `apps/webmail/src/components/DMPanel.tsx` (1,114 lines) by extracting the room list sidebar, message list, and message input area into dedicated sub-components.

**Files:**
- Create: `apps/webmail/src/components/dm/DMRoomList.tsx`
- Create: `apps/webmail/src/components/dm/DMMessageList.tsx`
- Create: `apps/webmail/src/components/dm/DMComposer.tsx`
- Modify: `apps/webmail/src/components/DMPanel.tsx`

**Acceptance Criteria:**
- [ ] `DMPanel.tsx` is ≤ 500 lines
- [ ] `DMRoomList.tsx` renders the room list sidebar
- [ ] `DMMessageList.tsx` renders message bubbles + reaction strip
- [ ] `DMComposer.tsx` renders the text input, send button, and attachment button
- [ ] `pnpm --dir apps/webmail type-check` passes

**Verify:** `pnpm --dir apps/webmail type-check` → exit 0

**Steps:**

- [ ] **Step 1: Read DMPanel.tsx and identify sections**

Read `apps/webmail/src/components/DMPanel.tsx`. Identify:
1. Room list rendering (left sidebar — room names, avatars, unread counts)
2. Message list rendering (right pane — message bubbles, timestamps, reactions)
3. Message composer (bottom of right pane — text input, emoji picker, attachment, send button)

Note which state and callbacks each section needs.

- [ ] **Step 2: Create DMRoomList.tsx**

```typescript
'use client';

import type { DMRoom, DMUser } from '@/lib/api';

interface DMRoomListProps {
  rooms: DMRoom[];
  activeRoomId: string | null;
  currentUserId: string;
  selfAvatarUrl: string;
  onSelectRoom: (room: DMRoom) => void;
}

export function DMRoomList({ rooms, activeRoomId, currentUserId, selfAvatarUrl, onSelectRoom }: DMRoomListProps) {
  // Copy room list rendering JSX from DMPanel
  // Copy helper functions: roomTitle, avatarColor (or import from shared location)
  return (
    <div className="...">
      {rooms.map((room) => (
        <button key={room.id} onClick={() => onSelectRoom(room)} className={...}>
          {/* room avatar, title, last message preview */}
        </button>
      ))}
    </div>
  );
}
```

Copy the room list rendering JSX section from `DMPanel.tsx`. Copy `roomTitle` and any avatar-related helper functions used exclusively by the room list.

- [ ] **Step 3: Create DMMessageList.tsx**

```typescript
'use client';

import type { DMMessage, DMUser } from '@/lib/api';

interface DMMessageListProps {
  messages: DMMessage[];
  currentUserId: string;
  selfAvatarUrl: string;
  reactionEmoji: string[];
  onToggleReaction: (messageId: string, emoji: string) => void;
  onEditMessage: (message: DMMessage) => void;
  onDeleteMessage: (messageId: string) => void;
  listRef: React.RefObject<HTMLDivElement>;
}

export function DMMessageList({ messages, currentUserId, onToggleReaction, onEditMessage, onDeleteMessage, listRef }: DMMessageListProps) {
  // Copy message list + message bubble rendering JSX from DMPanel
  return (
    <div ref={listRef} className="flex-1 overflow-y-auto ...">
      {messages.map((msg) => (
        <div key={msg.id}>
          {/* message bubble, timestamp, reactions */}
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 4: Create DMComposer.tsx**

```typescript
'use client';

interface DMComposerProps {
  value: string;
  onChange: (value: string) => void;
  onSend: () => void;
  onAttach: (files: FileList) => void;
  disabled?: boolean;
  placeholder?: string;
}

export function DMComposer({ value, onChange, onSend, onAttach, disabled, placeholder }: DMComposerProps) {
  // Copy the text input + send button + attachment button JSX from DMPanel
  return (
    <div className="flex items-end gap-2 p-3 border-t ...">
      <textarea value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} className="..." />
      <button type="button" onClick={onSend} disabled={disabled || !value.trim()}>Send</button>
    </div>
  );
}
```

- [ ] **Step 5: Update DMPanel.tsx**

Import the three new components and replace the extracted JSX sections. Pass state and callbacks as props.

```typescript
import { DMRoomList } from './dm/DMRoomList';
import { DMMessageList } from './dm/DMMessageList';
import { DMComposer } from './dm/DMComposer';
```

- [ ] **Step 6: Type-check**

```bash
pnpm --dir apps/webmail type-check
```

Fix prop type errors.

- [ ] **Step 7: Commit**

```bash
git add apps/webmail/src/components/dm/
git add apps/webmail/src/components/DMPanel.tsx
git commit -m "refactor(webmail): extract DMRoomList, DMMessageList, DMComposer from DMPanel"
```

---

### Task 9: Split internal/httpapi/admin.go by route group

**Goal:** Break `internal/httpapi/admin.go` (8,901 lines) into focused files within the same `httpapi` package. All files share the package namespace, so no import changes are needed — this is purely mechanical file reorganization.

**Files:**
- Create: `internal/httpapi/admin_types.go`
- Create: `internal/httpapi/admin_middleware.go`
- Create: `internal/httpapi/admin_user.go`
- Create: `internal/httpapi/admin_company.go`
- Create: `internal/httpapi/admin_mail.go`
- Create: `internal/httpapi/admin_policy.go`
- Create: `internal/httpapi/admin_system.go`
- Modify: `internal/httpapi/admin.go`

**Acceptance Criteria:**
- [ ] `internal/httpapi/admin.go` is ≤ 1,500 lines (contains `RegisterAdminRoutes` + route wiring)
- [ ] `go build ./internal/httpapi/...` exits 0
- [ ] `go test ./internal/httpapi/...` passes (all existing tests still pass)
- [ ] No file in `internal/httpapi/` exceeds 2,000 lines

**Verify:** `go test ./internal/httpapi/... -count=1` → PASS

**Steps:**

- [ ] **Step 1: Create admin_types.go**

Create `internal/httpapi/admin_types.go` with `package httpapi`. Move to this file:
- `adminRouteConfig` struct (lines ~41-55)
- `AdminRouteOption` type and all `With*` option functions (lines 56-120)
- `AdminBackpressureService`, `AdminService`, and all other interface definitions
- `RequestIDKey` context key and related constants
- `BackgroundTracker` type if defined in admin.go

```go
package httpapi

import (
    // same imports as needed
)

type adminRouteConfig struct {
    // ...
}

type AdminRouteOption func(*adminRouteConfig)

func WithRouteCounters(c *delivery.RouteCounters) AdminRouteOption { ... }
// ... all With* functions
```

- [ ] **Step 2: Create admin_middleware.go**

Create `internal/httpapi/admin_middleware.go` with `package httpapi`. Move to this file:
- `RequestIDFromContext`, `ContextWithRequestID`, `RequestIDAttr`, `RequestContextAttrs`
- `RequestIDMiddleware`, `newRequestID`
- `SecurityHeadersMiddleware`
- `CORSMiddleware`, `parseCORSOrigins`, `matchCORSOrigin`
- `AdminIPRateLimiter` struct and its methods
- `MaxRequestBodyMiddleware`
- `RealIPMiddleware` (if present)
- `authMiddleware` (if present)

```go
package httpapi

// HTTP middleware: RequestID, SecurityHeaders, CORS, rate limiting, body limits.
```

- [ ] **Step 3: Create admin_user.go**

Create `internal/httpapi/admin_user.go` with `package httpapi`. Move all user/admin-user handler functions:
- `handleListUsers`, `handleGetUser`, `handleCreateUser`, `handleDeleteUser`
- `handleUpdateUserStatus`, `handleUpdateUserQuota`, `handleUpdateUserRole`, `handleUpdateUserRecoveryEmail`
- `handleListCompanySessions`, `handleRevokeCompanySession`
- `handleSendInviteEmail`
- Any helpers used exclusively by user handlers

```go
package httpapi

// User, admin-user, and session management handlers.
```

- [ ] **Step 4: Create admin_company.go**

Move company and domain handlers:
- `handleListCompanies`, `handleGetCompany`
- `handleListDomains`, `handleGetDomainSettings`, `handleUpdateDomainSettings`, `handleCheckDomainDNS`
- `handleListOrgUnits`, `handleGetOrgHierarchy`, org membership handlers
- `handleListDKIMKeys`

```go
package httpapi

// Company, domain, org structure, and DKIM handlers.
```

- [ ] **Step 5: Create admin_mail.go**

Move mail delivery/flow handlers:
- `handleListMailFlowLogs`, `handleGetMailFlowStats`
- `handleListDeliveryAttempts`, `handleListExhaustedDeliveries`
- `handleListDLQ`, `handleDeleteDLQEntry`, `handleRetryOutbox`
- `handleListSuppressionList`, `handleRemoveSuppressionEntry`
- `handleListQuotaUsage`, `handleListQuotaAlerts`
- `handleListSpamFilterEvents`, `handleGetSpamFilterStats`, `handleGetSpamFilter`, `handleGetSpamFilterEvents`
- `handleGetAlertEvents`

```go
package httpapi

// Mail delivery, flow, spam filter, DLQ, and quota handlers.
```

- [ ] **Step 6: Create admin_policy.go**

Move all policy config handlers (lines 5479+):
- Org settings handlers
- IP access policy handlers
- Retention policy handlers
- Auth policy handlers
- Audit policy handlers
- Security governance policy handlers
- Session policy handlers
- Rate limit policy handlers
- DMARC/SPF policy handlers
- Spam filter policy handlers
- Routing rules handlers
- SSO/SAML config handlers
- Outbound SMTP policy handlers
- Webhook handlers
- All associated `const *Key = "..."` and JSON helper types

```go
package httpapi

// Configuration policy handlers: org, IP, retention, auth, session, rate-limit, DMARC, spam, routing, SSO, SMTP, webhooks.
```

- [ ] **Step 7: Create admin_system.go**

Move health/queue/audit handlers:
- `handleCheckHealth`
- `handleGetQueueStats`
- `handleGetAuditLogs`
- `handleAdminAPIRequest`
- Any metrics or queue-management handlers not yet moved

```go
package httpapi

// System health, queue stats, audit log, and admin API request handlers.
```

- [ ] **Step 8: Trim admin.go**

After moving functions to the new files, `admin.go` should contain only:
- Package declaration and imports
- `RegisterAdminRoutes(mux *http.ServeMux, ...)` function with its route registration table
- Any inline helper functions that are used exclusively within `RegisterAdminRoutes`

- [ ] **Step 9: Build and test**

```bash
go build ./internal/httpapi/...
go test ./internal/httpapi/... -count=1
```

Both must pass. If the build fails with "undefined: X", the function X was referenced before being moved — find it and move it to the appropriate file.

- [ ] **Step 10: Commit**

```bash
git add internal/httpapi/admin_types.go internal/httpapi/admin_middleware.go \
        internal/httpapi/admin_user.go internal/httpapi/admin_company.go \
        internal/httpapi/admin_mail.go internal/httpapi/admin_policy.go \
        internal/httpapi/admin_system.go internal/httpapi/admin.go
git commit -m "refactor(httpapi): split admin.go into focused route-group files"
```

---

### Task 10: Split internal/app/admin_service.go into domain files

**Goal:** Break `internal/app/admin_service.go` (1,759 lines, 121 methods on `adminService`) into focused files within the same `app` package.

**Files:**
- Create: `internal/app/admin_service_user.go`
- Create: `internal/app/admin_service_directory.go`
- Create: `internal/app/admin_service_storage.go`
- Create: `internal/app/admin_service_delivery.go`
- Create: `internal/app/admin_service_config.go`
- Modify: `internal/app/admin_service.go`

**Acceptance Criteria:**
- [ ] `internal/app/admin_service.go` is ≤ 200 lines (struct definition + constructor only)
- [ ] `go build ./internal/app/...` exits 0
- [ ] `go test ./internal/app/...` passes

**Verify:** `go test ./internal/app/... -count=1` → PASS

**Steps:**

- [ ] **Step 1: Create admin_service_delivery.go**

Create `internal/app/admin_service_delivery.go` with `package app`. Move:
- `(s adminService) GetBackpressure`
- `(s adminService) UpdateBackpressure`
- `backpressureAuditDetail`, `backpressureAuditStateFromState`, `truncateBackpressureAuditString`
- All DLQ and delivery attempt related methods on `adminService`
- `(s adminService) RetryOutbox` and related helpers

```go
package app

// Delivery, backpressure, DLQ, and outbox retry methods on adminService.
```

- [ ] **Step 2: Create admin_service_storage.go**

Move:
- `(s adminService) RunAttachmentCleanup`
- `(s adminService) CountStaleAttachmentUploads`, `ListStaleAttachmentUploads`
- `(s adminService) RunAttachmentUploadSessionCleanup`
- `(s adminService) CountStaleAttachmentUploadSessions`, `ListStaleAttachmentUploadSessions`
- `(s adminService) ListDriveUploadSessions`
- `attachmentCleanupAuditDetail`, `attachmentAuditIDs`, `attachmentSessionAuditIDs`, `sampleAttachmentCleanupIDs`

```go
package app

// Attachment and drive upload storage cleanup methods on adminService.
```

- [ ] **Step 3: Create admin_service_directory.go**

Move:
- `(s adminService) ListDirectoryDelegations`, `CreateDirectoryDelegation`, `DeleteDirectoryDelegation`, `UpdateDirectoryDelegationRole`, `ReassignDirectoryDelegation`
- `(s adminService) CreateDirectoryGroupMembership`, `ListDirectoryGroupMemberships`, `DeleteDirectoryGroupMembership`, `UpdateDirectoryGroupMembershipRole`
- `(s adminService) ListOrgUnits`, `GetOrgHierarchy`, `ListUserOrgMemberships`, `AssignUserOrgMembership`, `UpdateOrgMembership`, `RemoveOrgMembership`

```go
package app

// Directory delegation, group membership, and org unit methods on adminService.
```

- [ ] **Step 4: Create admin_service_user.go**

Move:
- `(s adminService) GetUser`, `ListUsers`, `CreateUser`, `DeleteUser`
- `(s adminService) UpdateUserStatus`, `UpdateUserQuota`, `UpdateUserRole`, `UpdateUserRecoveryEmail`
- `(s adminService) ListCompanySessions`, `RevokeCompanySession`, `RevokeAllCompanySessions`
- `(s adminService) SendInviteEmail` and related helpers

```go
package app

// User management and session methods on adminService.
```

- [ ] **Step 5: Create admin_service_config.go**

Move:
- All `(s adminService) Get*Policy`, `Update*Policy`, `Get*Config`, `Update*Config` methods
- `(s adminService) GetBackpressureConfig`, `UpdateDomainSettings`, etc.
- Policy-related helper functions used exclusively by config methods

```go
package app

// Policy configuration and domain settings methods on adminService.
```

- [ ] **Step 6: Trim admin_service.go**

After moves, `admin_service.go` should contain only:
- `type adminService struct { ... }` definition
- `func newAdminService(...) *adminService { ... }` constructor
- Any methods that don't clearly belong to a single domain file

- [ ] **Step 7: Build and test**

```bash
go build ./internal/app/...
go test ./internal/app/... -count=1
```

Both must pass. Undefined references mean a helper function was left behind — move it to the file that uses it.

- [ ] **Step 8: Commit**

```bash
git add internal/app/admin_service*.go
git commit -m "refactor(app): split admin_service.go into domain-focused files"
```
