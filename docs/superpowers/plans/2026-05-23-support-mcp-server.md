# Support MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** TypeScript MCP server at `apps/mcp-support` that exposes 34 tools for autonomous AI-driven customer support across Suppo, GoGoMail Admin API, and GitHub.

**Architecture:** Stateless Node.js process; clients in `src/clients/` handle HTTP I/O, tools in `src/tools/` define MCP schemas and dispatch to clients. `src/index.ts` wires everything together and supports stdio (Claude Desktop) or HTTP+SSE (remote agent) transport.

**Tech Stack:** `@modelcontextprotocol/sdk` · `@octokit/rest` · `zod` · TypeScript · Node 20+

---

## File Map

| File | Responsibility |
|---|---|
| `apps/mcp-support/package.json` | deps, scripts |
| `apps/mcp-support/tsconfig.json` | TS compiler config |
| `apps/mcp-support/src/config.ts` | env var parsing, fail-fast |
| `apps/mcp-support/src/clients/suppo.ts` | Suppo REST API client |
| `apps/mcp-support/src/clients/gogomail.ts` | GoGoMail Admin API client |
| `apps/mcp-support/src/clients/github.ts` | GitHub client via @octokit/rest |
| `apps/mcp-support/src/tools/suppo.ts` | 10 Suppo MCP tools |
| `apps/mcp-support/src/tools/gogomail.ts` | 18 GoGoMail MCP tools |
| `apps/mcp-support/src/tools/github.ts` | 6 GitHub MCP tools |
| `apps/mcp-support/src/index.ts` | server entrypoint, transport switch |

---

### Task 1: Project Scaffold

**Goal:** Create `apps/mcp-support` with working `package.json`, `tsconfig.json`, and `src/config.ts` so `npm install && npm run type-check` passes.

**Files:**
- Create: `apps/mcp-support/package.json`
- Create: `apps/mcp-support/tsconfig.json`
- Create: `apps/mcp-support/src/config.ts`

**Acceptance Criteria:**
- [ ] `npm install` succeeds with no peer-dep errors
- [ ] `npm run type-check` exits 0 with just `config.ts` present
- [ ] Missing env var causes process to exit with a clear error message (verified manually)

**Verify:** `cd apps/mcp-support && npm install && npm run type-check` → exit 0

**Steps:**

- [ ] **Step 1: Create package.json**

```json
{
  "name": "@gogomail/mcp-support",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "tsx src/index.ts",
    "type-check": "tsc --noEmit"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.12.0",
    "@octokit/rest": "^20.1.1",
    "zod": "^3.22.4"
  },
  "devDependencies": {
    "@types/node": "^20.14.0",
    "tsx": "^4.15.0",
    "typescript": "^5.4.0"
  }
}
```

- [ ] **Step 2: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true
  },
  "include": ["src"],
  "exclude": ["node_modules", "dist"]
}
```

- [ ] **Step 3: Create src/config.ts**

```typescript
function requireEnv(name: string): string {
  const val = process.env[name];
  if (!val) {
    console.error(`[mcp-support] Missing required env var: ${name}`);
    process.exit(1);
  }
  return val;
}

export const config = {
  gogomail: {
    adminUrl: requireEnv("GOGOMAIL_ADMIN_URL"),
    adminKey: requireEnv("GOGOMAIL_ADMIN_KEY"),
  },
  suppo: {
    apiUrl: requireEnv("SUPPO_API_URL"),
    apiKey: requireEnv("SUPPO_API_KEY"),
  },
  github: {
    token: requireEnv("GITHUB_TOKEN"),
    repo: process.env["GITHUB_REPO"] ?? "parkjangwon/gogomail",
  },
  transport: (process.env["MCP_TRANSPORT"] ?? "stdio") as "stdio" | "sse",
  port: parseInt(process.env["MCP_PORT"] ?? "3100", 10),
};
```

- [ ] **Step 4: Run npm install and type-check**

```bash
cd apps/mcp-support
npm install
npm run type-check
```

Expected: no errors (config.ts alone has no type issues with strict mode).

- [ ] **Step 5: Commit**

```bash
git add apps/mcp-support/
git commit -m "feat(mcp-support): scaffold project with config and tsconfig"
```

---

### Task 2: Suppo API Client

**Goal:** `src/clients/suppo.ts` implementing typed fetch wrappers for all Suppo endpoints used by the 10 Suppo tools.

**Files:**
- Create: `apps/mcp-support/src/clients/suppo.ts`

**Acceptance Criteria:**
- [ ] All methods compile without errors
- [ ] `type-check` still passes
- [ ] All 10 Suppo tool operations have a corresponding client method

**Verify:** `npm run type-check` → exit 0

**Steps:**

- [ ] **Step 1: Create src/clients/suppo.ts**

```typescript
export interface SuppoTicket {
  id: string;
  subject: string;
  status: string;
  priority: string;
  customerName: string;
  customerEmail: string;
  assigneeId: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface SuppoComment {
  id: string;
  ticketId: string;
  body: string;
  internal: boolean;
  authorId: string | null;
  createdAt: string;
}

export interface SuppoTicketDetail extends SuppoTicket {
  comments: SuppoComment[];
}

export interface SuppoAgent {
  id: string;
  name: string;
  email: string;
}

export interface SuppoKbArticle {
  id: string;
  title: string;
  content: string;
  createdAt: string;
}

export class SuppoClient {
  private readonly baseUrl: string;
  private readonly apiKey: string;

  constructor(baseUrl: string, apiKey: string) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.apiKey = apiKey;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const url = `${this.baseUrl}/api/public${path}`;
    const res = await fetch(url, {
      method,
      headers: {
        Authorization: `Bearer ${this.apiKey}`,
        "Content-Type": "application/json",
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(`Suppo API ${method} ${path} → ${res.status}: ${text}`);
    }
    return res.json() as Promise<T>;
  }

  async listTickets(params: {
    status?: string;
    priority?: string;
    limit?: number;
  }): Promise<SuppoTicket[]> {
    const q = new URLSearchParams();
    if (params.status) q.set("status", params.status);
    if (params.priority) q.set("priority", params.priority);
    if (params.limit) q.set("limit", String(params.limit));
    return this.request<SuppoTicket[]>("GET", `/tickets?${q}`);
  }

  async getTicket(ticketId: string): Promise<SuppoTicketDetail> {
    return this.request<SuppoTicketDetail>("GET", `/tickets/${ticketId}`);
  }

  async searchTickets(params: {
    customerEmail?: string;
    query?: string;
  }): Promise<SuppoTicket[]> {
    const q = new URLSearchParams();
    if (params.customerEmail) q.set("customerEmail", params.customerEmail);
    if (params.query) q.set("q", params.query);
    return this.request<SuppoTicket[]>("GET", `/tickets?${q}`);
  }

  async createTicket(data: {
    customerName: string;
    customerEmail: string;
    subject: string;
    description: string;
    priority?: string;
  }): Promise<SuppoTicket> {
    return this.request<SuppoTicket>("POST", "/tickets", data);
  }

  async updateTicket(
    ticketId: string,
    data: { status?: string; priority?: string },
  ): Promise<SuppoTicket> {
    return this.request<SuppoTicket>("PATCH", `/tickets/${ticketId}`, data);
  }

  async addComment(
    ticketId: string,
    data: { body: string; internal?: boolean },
  ): Promise<SuppoComment> {
    return this.request<SuppoComment>(
      "POST",
      `/tickets/${ticketId}/comments`,
      data,
    );
  }

  async assignTicket(
    ticketId: string,
    assigneeId: string,
  ): Promise<SuppoTicket> {
    return this.request<SuppoTicket>("PATCH", `/tickets/${ticketId}`, {
      assigneeId,
    });
  }

  async listAgents(): Promise<SuppoAgent[]> {
    return this.request<SuppoAgent[]>("GET", "/agents");
  }

  async searchKb(query: string): Promise<SuppoKbArticle[]> {
    const q = new URLSearchParams({ q: query });
    return this.request<SuppoKbArticle[]>("GET", `/kb/articles/search?${q}`);
  }

  async createKbArticle(data: {
    title: string;
    content: string;
  }): Promise<SuppoKbArticle> {
    return this.request<SuppoKbArticle>("POST", "/kb/articles", data);
  }
}
```

- [ ] **Step 2: Verify type-check**

```bash
cd apps/mcp-support && npm run type-check
```

Expected: exit 0

- [ ] **Step 3: Commit**

```bash
git add apps/mcp-support/src/clients/suppo.ts
git commit -m "feat(mcp-support): add Suppo API client"
```

---

### Task 3: Suppo MCP Tools

**Goal:** `src/tools/suppo.ts` exporting a `toolDefinitions` array (for ListTools) and a `callTool` handler covering all 10 Suppo tools.

**Files:**
- Create: `apps/mcp-support/src/tools/suppo.ts`

**Acceptance Criteria:**
- [ ] All 10 tools listed in `toolDefinitions`
- [ ] `callTool` handles each tool name; unknown name throws
- [ ] `type-check` passes

**Verify:** `npm run type-check` → exit 0

**Steps:**

- [ ] **Step 1: Create src/tools/suppo.ts**

```typescript
import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { SuppoClient } from "../clients/suppo.js";

export const toolDefinitions: Tool[] = [
  {
    name: "suppo_list_tickets",
    description: "List helpdesk tickets. Filter by status (open/pending/closed/resolved) and/or priority (low/normal/high/urgent).",
    inputSchema: {
      type: "object",
      properties: {
        status: { type: "string", description: "Filter by ticket status" },
        priority: { type: "string", description: "Filter by priority" },
        limit: { type: "number", description: "Max results (default 20)" },
      },
    },
  },
  {
    name: "suppo_get_ticket",
    description: "Get full ticket detail including all comment history.",
    inputSchema: {
      type: "object",
      properties: {
        ticketId: { type: "string" },
      },
      required: ["ticketId"],
    },
  },
  {
    name: "suppo_search_tickets",
    description: "Search tickets by customer email or keyword.",
    inputSchema: {
      type: "object",
      properties: {
        customerEmail: { type: "string" },
        query: { type: "string" },
      },
    },
  },
  {
    name: "suppo_create_ticket",
    description: "Create a new helpdesk ticket (e.g. for internally-discovered issues).",
    inputSchema: {
      type: "object",
      properties: {
        customerName: { type: "string" },
        customerEmail: { type: "string" },
        subject: { type: "string" },
        description: { type: "string" },
        priority: { type: "string", description: "low | normal | high | urgent" },
      },
      required: ["customerName", "customerEmail", "subject", "description"],
    },
  },
  {
    name: "suppo_update_ticket",
    description: "Change a ticket's status or priority.",
    inputSchema: {
      type: "object",
      properties: {
        ticketId: { type: "string" },
        status: { type: "string" },
        priority: { type: "string" },
      },
      required: ["ticketId"],
    },
  },
  {
    name: "suppo_add_comment",
    description: "Add a customer reply or internal memo to a ticket. Set internal=true for audit memos not visible to the customer.",
    inputSchema: {
      type: "object",
      properties: {
        ticketId: { type: "string" },
        body: { type: "string" },
        internal: { type: "boolean", description: "true = internal memo, false = customer-visible reply" },
      },
      required: ["ticketId", "body"],
    },
  },
  {
    name: "suppo_assign_ticket",
    description: "Assign ticket to a support agent by their agent ID.",
    inputSchema: {
      type: "object",
      properties: {
        ticketId: { type: "string" },
        assigneeId: { type: "string" },
      },
      required: ["ticketId", "assigneeId"],
    },
  },
  {
    name: "suppo_list_agents",
    description: "List all available support agents that tickets can be assigned to.",
    inputSchema: { type: "object", properties: {} },
  },
  {
    name: "suppo_search_kb",
    description: "Search the knowledge base for existing articles.",
    inputSchema: {
      type: "object",
      properties: {
        query: { type: "string" },
      },
      required: ["query"],
    },
  },
  {
    name: "suppo_create_kb_article",
    description: "Create a new knowledge base article from a resolved support case.",
    inputSchema: {
      type: "object",
      properties: {
        title: { type: "string" },
        content: { type: "string" },
      },
      required: ["title", "content"],
    },
  },
];

const ListTicketsSchema = z.object({
  status: z.string().optional(),
  priority: z.string().optional(),
  limit: z.number().optional(),
});

const TicketIdSchema = z.object({ ticketId: z.string() });

const SearchTicketsSchema = z.object({
  customerEmail: z.string().optional(),
  query: z.string().optional(),
});

const CreateTicketSchema = z.object({
  customerName: z.string(),
  customerEmail: z.string(),
  subject: z.string(),
  description: z.string(),
  priority: z.string().optional(),
});

const UpdateTicketSchema = z.object({
  ticketId: z.string(),
  status: z.string().optional(),
  priority: z.string().optional(),
});

const AddCommentSchema = z.object({
  ticketId: z.string(),
  body: z.string(),
  internal: z.boolean().optional(),
});

const AssignTicketSchema = z.object({
  ticketId: z.string(),
  assigneeId: z.string(),
});

const SearchKbSchema = z.object({ query: z.string() });

const CreateKbArticleSchema = z.object({
  title: z.string(),
  content: z.string(),
});

export async function callTool(
  client: SuppoClient,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  switch (name) {
    case "suppo_list_tickets": {
      const p = ListTicketsSchema.parse(args);
      return client.listTickets(p);
    }
    case "suppo_get_ticket": {
      const { ticketId } = TicketIdSchema.parse(args);
      return client.getTicket(ticketId);
    }
    case "suppo_search_tickets": {
      const p = SearchTicketsSchema.parse(args);
      return client.searchTickets(p);
    }
    case "suppo_create_ticket": {
      const p = CreateTicketSchema.parse(args);
      return client.createTicket(p);
    }
    case "suppo_update_ticket": {
      const { ticketId, ...rest } = UpdateTicketSchema.parse(args);
      return client.updateTicket(ticketId, rest);
    }
    case "suppo_add_comment": {
      const { ticketId, body, internal } = AddCommentSchema.parse(args);
      return client.addComment(ticketId, { body, internal });
    }
    case "suppo_assign_ticket": {
      const { ticketId, assigneeId } = AssignTicketSchema.parse(args);
      return client.assignTicket(ticketId, assigneeId);
    }
    case "suppo_list_agents": {
      return client.listAgents();
    }
    case "suppo_search_kb": {
      const { query } = SearchKbSchema.parse(args);
      return client.searchKb(query);
    }
    case "suppo_create_kb_article": {
      const p = CreateKbArticleSchema.parse(args);
      return client.createKbArticle(p);
    }
    default:
      throw new Error(`Unknown Suppo tool: ${name}`);
  }
}
```

- [ ] **Step 2: Type-check**

```bash
cd apps/mcp-support && npm run type-check
```

Expected: exit 0

- [ ] **Step 3: Commit**

```bash
git add apps/mcp-support/src/tools/suppo.ts
git commit -m "feat(mcp-support): add 10 Suppo MCP tools"
```

---

### Task 4: GoGoMail Admin API Client

**Goal:** `src/clients/gogomail.ts` with typed methods for all 18 GoGoMail operations.

**Files:**
- Create: `apps/mcp-support/src/clients/gogomail.ts`

**Acceptance Criteria:**
- [ ] All 18 tool operations have a corresponding client method
- [ ] `type-check` passes

**Verify:** `npm run type-check` → exit 0

**Steps:**

- [ ] **Step 1: Create src/clients/gogomail.ts**

```typescript
export interface GogomailUser {
  id: string;
  email: string;
  name: string;
  status: "active" | "suspended" | "disabled";
  role: string;
  companyId: string;
  quotaBytes: number;
  createdAt: string;
}

export interface GogomailQuota {
  userId: string;
  allocatedBytes: number;
  usedBytes: number;
  updatedAt: string;
}

export interface GogomailMailLog {
  id: string;
  userId: string;
  messageId: string;
  direction: "inbound" | "outbound";
  status: string;
  from: string;
  to: string;
  subject: string;
  timestamp: string;
}

export interface GogomailMessageTrace {
  messageId: string;
  hops: Array<{
    server: string;
    timestamp: string;
    action: string;
    status: string;
  }>;
}

export interface GogomailDeliveryAttempt {
  id: string;
  messageId: string;
  attemptedAt: string;
  status: string;
  errorCode: string | null;
  errorMessage: string | null;
  nextRetryAt: string | null;
}

export interface GogomailAuditLog {
  id: string;
  actorId: string | null;
  targetId: string | null;
  action: string;
  meta: Record<string, unknown>;
  createdAt: string;
}

export interface GogomailSession {
  id: string;
  userId: string;
  userAgent: string;
  ip: string;
  createdAt: string;
  lastSeenAt: string;
}

export interface GogomailHealth {
  status: "ok" | "degraded" | "down";
  queueDepth: number;
  components: Record<string, string>;
}

export interface GogomailCompany {
  id: string;
  name: string;
  domains: string[];
  plan: string;
  createdAt: string;
}

export interface GogomailDomainSettings {
  domainId: string;
  domain: string;
  catchAll: boolean;
  spfEnabled: boolean;
  dkimEnabled: boolean;
  dmarcEnabled: boolean;
  maxMessageSize: number;
}

export interface GogomailAlertEvent {
  id: string;
  companyId: string;
  type: string;
  severity: "info" | "warning" | "critical";
  message: string;
  createdAt: string;
}

export class GogomailClient {
  private readonly baseUrl: string;
  private readonly apiKey: string;

  constructor(baseUrl: string, apiKey: string) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.apiKey = apiKey;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const url = `${this.baseUrl}/api/admin${path}`;
    const res = await fetch(url, {
      method,
      headers: {
        Authorization: `Bearer ${this.apiKey}`,
        "Content-Type": "application/json",
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(
        `GoGoMail Admin API ${method} ${path} → ${res.status}: ${text}`,
      );
    }
    return res.json() as Promise<T>;
  }

  // ── Read operations ────────────────────────────────────────────

  async findUser(email: string): Promise<GogomailUser[]> {
    const q = new URLSearchParams({ email });
    return this.request<GogomailUser[]>("GET", `/users?${q}`);
  }

  async getUser(userId: string): Promise<GogomailUser> {
    return this.request<GogomailUser>("GET", `/users/${userId}`);
  }

  async getUserQuota(userId: string): Promise<GogomailQuota> {
    return this.request<GogomailQuota>("GET", `/users/${userId}/quota`);
  }

  async getMailLogs(params: {
    userId: string;
    direction?: string;
    status?: string;
    from?: string;
    to?: string;
  }): Promise<GogomailMailLog[]> {
    const { userId, ...filters } = params;
    const q = new URLSearchParams();
    if (filters.direction) q.set("direction", filters.direction);
    if (filters.status) q.set("status", filters.status);
    if (filters.from) q.set("from", filters.from);
    if (filters.to) q.set("to", filters.to);
    return this.request<GogomailMailLog[]>(
      "GET",
      `/users/${userId}/maillogs?${q}`,
    );
  }

  async traceMessage(messageId: string): Promise<GogomailMessageTrace> {
    return this.request<GogomailMessageTrace>(
      "GET",
      `/messages/${messageId}/trace`,
    );
  }

  async getDeliveryAttempts(
    messageId: string,
  ): Promise<GogomailDeliveryAttempt[]> {
    return this.request<GogomailDeliveryAttempt[]>(
      "GET",
      `/messages/${messageId}/delivery-attempts`,
    );
  }

  async getAuditLogs(params: {
    userId?: string;
    companyId?: string;
    from?: string;
    to?: string;
  }): Promise<GogomailAuditLog[]> {
    const q = new URLSearchParams();
    if (params.userId) q.set("userId", params.userId);
    if (params.companyId) q.set("companyId", params.companyId);
    if (params.from) q.set("from", params.from);
    if (params.to) q.set("to", params.to);
    return this.request<GogomailAuditLog[]>("GET", `/audit-logs?${q}`);
  }

  async listUserSessions(userId: string): Promise<GogomailSession[]> {
    return this.request<GogomailSession[]>("GET", `/users/${userId}/sessions`);
  }

  async checkHealth(): Promise<GogomailHealth> {
    return this.request<GogomailHealth>("GET", "/health");
  }

  // ── Action operations ──────────────────────────────────────────

  async resetPassword(userId: string): Promise<{ sent: boolean }> {
    return this.request<{ sent: boolean }>(
      "POST",
      `/users/${userId}/reset-password`,
    );
  }

  async updateUserStatus(
    userId: string,
    status: "active" | "suspended" | "disabled",
  ): Promise<GogomailUser> {
    return this.request<GogomailUser>("PATCH", `/users/${userId}/status`, {
      status,
    });
  }

  async updateUserQuota(
    userId: string,
    quotaBytes: number,
  ): Promise<GogomailQuota> {
    return this.request<GogomailQuota>("PATCH", `/users/${userId}/quota`, {
      quotaBytes,
    });
  }

  async revokeSessions(userId: string): Promise<{ revoked: number }> {
    return this.request<{ revoked: number }>(
      "DELETE",
      `/users/${userId}/sessions`,
    );
  }

  async updateUserRole(userId: string, role: string): Promise<GogomailUser> {
    return this.request<GogomailUser>("PATCH", `/users/${userId}/role`, {
      role,
    });
  }

  async getCompany(companyId: string): Promise<GogomailCompany> {
    return this.request<GogomailCompany>("GET", `/companies/${companyId}`);
  }

  async getDomainSettings(domainId: string): Promise<GogomailDomainSettings> {
    return this.request<GogomailDomainSettings>(
      "GET",
      `/domains/${domainId}/settings`,
    );
  }

  async updateDomainSettings(
    domainId: string,
    settings: Partial<GogomailDomainSettings>,
  ): Promise<GogomailDomainSettings> {
    return this.request<GogomailDomainSettings>(
      "PATCH",
      `/domains/${domainId}/settings`,
      settings,
    );
  }

  async getAlertEvents(params: {
    companyId: string;
    limit?: number;
  }): Promise<GogomailAlertEvent[]> {
    const q = new URLSearchParams();
    if (params.limit) q.set("limit", String(params.limit));
    return this.request<GogomailAlertEvent[]>(
      "GET",
      `/companies/${params.companyId}/alerts?${q}`,
    );
  }
}
```

- [ ] **Step 2: Type-check**

```bash
cd apps/mcp-support && npm run type-check
```

Expected: exit 0

- [ ] **Step 3: Commit**

```bash
git add apps/mcp-support/src/clients/gogomail.ts
git commit -m "feat(mcp-support): add GoGoMail Admin API client"
```

---

### Task 5: GoGoMail MCP Tools (all 18)

**Goal:** `src/tools/gogomail.ts` with all 18 GoGoMail tool definitions and a `callTool` handler. Action tools (9 of 18) must auto-write an internal Suppo audit comment after every successful execution.

**Files:**
- Create: `apps/mcp-support/src/tools/gogomail.ts`

**Acceptance Criteria:**
- [ ] All 18 tools in `toolDefinitions`
- [ ] Every action tool call auto-posts an internal Suppo comment when a `ticketId` is passed in context
- [ ] When `ticketId` is absent from context, action tools create a standalone audit ticket instead
- [ ] `type-check` passes

**Verify:** `npm run type-check` → exit 0

**Steps:**

- [ ] **Step 1: Create src/tools/gogomail.ts**

```typescript
import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import type { SuppoClient } from "../clients/suppo.js";

// ── Tool definitions ────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
  // Read tools
  {
    name: "gogomail_find_user",
    description: "Find a GoGoMail user by email address.",
    inputSchema: {
      type: "object",
      properties: { email: { type: "string" } },
      required: ["email"],
    },
  },
  {
    name: "gogomail_get_user",
    description: "Get full user details including status, role, and quota.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string" } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_get_user_quota",
    description: "Get storage quota allocation and usage for a user.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string" } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_get_mail_logs",
    description: "Get mail flow logs for a user. Filter by direction, status, and time range.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        direction: { type: "string", description: "inbound | outbound" },
        status: { type: "string" },
        from: { type: "string", description: "ISO 8601 start time" },
        to: { type: "string", description: "ISO 8601 end time" },
      },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_trace_message",
    description: "Trace the delivery path of a specific message by its ID.",
    inputSchema: {
      type: "object",
      properties: { messageId: { type: "string" } },
      required: ["messageId"],
    },
  },
  {
    name: "gogomail_get_delivery_attempts",
    description: "Get all delivery attempts and error details for a message.",
    inputSchema: {
      type: "object",
      properties: { messageId: { type: "string" } },
      required: ["messageId"],
    },
  },
  {
    name: "gogomail_get_audit_logs",
    description: "Get system audit logs for a user or company, optionally filtered by time range.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        companyId: { type: "string" },
        from: { type: "string" },
        to: { type: "string" },
      },
    },
  },
  {
    name: "gogomail_list_user_sessions",
    description: "List all active sessions for a user.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string" } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_check_health",
    description: "Check GoGoMail system health and mail queue status.",
    inputSchema: { type: "object", properties: {} },
  },
  // Action tools
  {
    name: "gogomail_reset_password",
    description: "Send a password reset invitation email to the user. PREREQUISITE: call gogomail_get_user first to verify the current account state. Requires ticketId for audit trail.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        ticketId: { type: "string", description: "Suppo ticket ID for audit memo" },
      },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_update_user_status",
    description: "Change account status. PREREQUISITE: call gogomail_get_user to confirm current status first. status must be 'active' | 'suspended' | 'disabled'. Requires ticketId for audit trail.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        status: { type: "string", description: "active | suspended | disabled" },
        ticketId: { type: "string" },
      },
      required: ["userId", "status"],
    },
  },
  {
    name: "gogomail_update_user_quota",
    description: "Adjust user storage quota in bytes. PREREQUISITE: call gogomail_get_user_quota first. Requires ticketId for audit trail.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        quotaBytes: { type: "number" },
        ticketId: { type: "string" },
      },
      required: ["userId", "quotaBytes"],
    },
  },
  {
    name: "gogomail_revoke_sessions",
    description: "Force-logout all active sessions for a user. PREREQUISITE: call gogomail_list_user_sessions first. Requires ticketId for audit trail.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        ticketId: { type: "string" },
      },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_update_user_role",
    description: "Change a user's role. PREREQUISITE: call gogomail_get_user first. Requires ticketId for audit trail.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        role: { type: "string" },
        ticketId: { type: "string" },
      },
      required: ["userId", "role"],
    },
  },
  {
    name: "gogomail_get_company",
    description: "Get company and domain information by company ID.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string" } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_get_domain_settings",
    description: "Get domain configuration (SPF, DKIM, DMARC, catch-all, message size).",
    inputSchema: {
      type: "object",
      properties: { domainId: { type: "string" } },
      required: ["domainId"],
    },
  },
  {
    name: "gogomail_update_domain_settings",
    description: "Update domain configuration. PREREQUISITE: call gogomail_get_domain_settings first. Requires ticketId for audit trail.",
    inputSchema: {
      type: "object",
      properties: {
        domainId: { type: "string" },
        settings: {
          type: "object",
          description: "Partial domain settings to update",
        },
        ticketId: { type: "string" },
      },
      required: ["domainId", "settings"],
    },
  },
  {
    name: "gogomail_get_alert_events",
    description: "Get recent alert events for a company.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string" },
        limit: { type: "number" },
      },
      required: ["companyId"],
    },
  },
];

// ── Audit helper ────────────────────────────────────────────────

async function writeAuditComment(
  suppo: SuppoClient,
  ticketId: string | undefined,
  toolName: string,
  targetInfo: string,
  change: string,
): Promise<void> {
  const body = [
    `[자동 실행] ${toolName}`,
    `- 대상: ${targetInfo}`,
    `- 변경: ${change}`,
    `- 실행 시각: ${new Date().toISOString()}`,
  ].join("\n");

  if (ticketId) {
    await suppo.addComment(ticketId, { body, internal: true });
  } else {
    await suppo.createTicket({
      customerName: "System",
      customerEmail: "system@gogomail.io",
      subject: `[감사 기록] ${toolName}`,
      description: body,
      priority: "low",
    });
  }
}

// ── Zod schemas ────────────────────────────────────────────────

const EmailSchema = z.object({ email: z.string() });
const UserIdSchema = z.object({ userId: z.string() });
const MailLogsSchema = z.object({
  userId: z.string(),
  direction: z.string().optional(),
  status: z.string().optional(),
  from: z.string().optional(),
  to: z.string().optional(),
});
const MessageIdSchema = z.object({ messageId: z.string() });
const AuditLogsSchema = z.object({
  userId: z.string().optional(),
  companyId: z.string().optional(),
  from: z.string().optional(),
  to: z.string().optional(),
});
const ResetPasswordSchema = z.object({
  userId: z.string(),
  ticketId: z.string().optional(),
});
const UpdateStatusSchema = z.object({
  userId: z.string(),
  status: z.enum(["active", "suspended", "disabled"]),
  ticketId: z.string().optional(),
});
const UpdateQuotaSchema = z.object({
  userId: z.string(),
  quotaBytes: z.number(),
  ticketId: z.string().optional(),
});
const RevokeSessionsSchema = z.object({
  userId: z.string(),
  ticketId: z.string().optional(),
});
const UpdateRoleSchema = z.object({
  userId: z.string(),
  role: z.string(),
  ticketId: z.string().optional(),
});
const CompanyIdSchema = z.object({ companyId: z.string() });
const DomainIdSchema = z.object({ domainId: z.string() });
const UpdateDomainSchema = z.object({
  domainId: z.string(),
  settings: z.record(z.unknown()),
  ticketId: z.string().optional(),
});
const AlertEventsSchema = z.object({
  companyId: z.string(),
  limit: z.number().optional(),
});

// ── callTool dispatcher ─────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: SuppoClient,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  switch (name) {
    // Read tools
    case "gogomail_find_user": {
      const { email } = EmailSchema.parse(args);
      return gogomail.findUser(email);
    }
    case "gogomail_get_user": {
      const { userId } = UserIdSchema.parse(args);
      return gogomail.getUser(userId);
    }
    case "gogomail_get_user_quota": {
      const { userId } = UserIdSchema.parse(args);
      return gogomail.getUserQuota(userId);
    }
    case "gogomail_get_mail_logs": {
      const p = MailLogsSchema.parse(args);
      return gogomail.getMailLogs(p);
    }
    case "gogomail_trace_message": {
      const { messageId } = MessageIdSchema.parse(args);
      return gogomail.traceMessage(messageId);
    }
    case "gogomail_get_delivery_attempts": {
      const { messageId } = MessageIdSchema.parse(args);
      return gogomail.getDeliveryAttempts(messageId);
    }
    case "gogomail_get_audit_logs": {
      const p = AuditLogsSchema.parse(args);
      return gogomail.getAuditLogs(p);
    }
    case "gogomail_list_user_sessions": {
      const { userId } = UserIdSchema.parse(args);
      return gogomail.listUserSessions(userId);
    }
    case "gogomail_check_health": {
      return gogomail.checkHealth();
    }
    // Action tools
    case "gogomail_reset_password": {
      const { userId, ticketId } = ResetPasswordSchema.parse(args);
      const result = await gogomail.resetPassword(userId);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_reset_password",
        `userId: ${userId}`,
        "비밀번호 재설정 메일 발송",
      );
      return result;
    }
    case "gogomail_update_user_status": {
      const { userId, status, ticketId } = UpdateStatusSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserStatus(userId, status);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_status",
        `${before.email} (userId: ${userId})`,
        `${before.status} → ${status}`,
      );
      return result;
    }
    case "gogomail_update_user_quota": {
      const { userId, quotaBytes, ticketId } = UpdateQuotaSchema.parse(args);
      const before = await gogomail.getUserQuota(userId);
      const result = await gogomail.updateUserQuota(userId, quotaBytes);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_quota",
        `userId: ${userId}`,
        `${before.allocatedBytes} → ${quotaBytes} bytes`,
      );
      return result;
    }
    case "gogomail_revoke_sessions": {
      const { userId, ticketId } = RevokeSessionsSchema.parse(args);
      const result = await gogomail.revokeSessions(userId);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_revoke_sessions",
        `userId: ${userId}`,
        `${result.revoked}개 세션 강제 종료`,
      );
      return result;
    }
    case "gogomail_update_user_role": {
      const { userId, role, ticketId } = UpdateRoleSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserRole(userId, role);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_role",
        `${before.email} (userId: ${userId})`,
        `${before.role} → ${role}`,
      );
      return result;
    }
    case "gogomail_get_company": {
      const { companyId } = CompanyIdSchema.parse(args);
      return gogomail.getCompany(companyId);
    }
    case "gogomail_get_domain_settings": {
      const { domainId } = DomainIdSchema.parse(args);
      return gogomail.getDomainSettings(domainId);
    }
    case "gogomail_update_domain_settings": {
      const { domainId, settings, ticketId } = UpdateDomainSchema.parse(args);
      const result = await gogomail.updateDomainSettings(domainId, settings);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_domain_settings",
        `domainId: ${domainId}`,
        JSON.stringify(settings),
      );
      return result;
    }
    case "gogomail_get_alert_events": {
      const p = AlertEventsSchema.parse(args);
      return gogomail.getAlertEvents(p);
    }
    default:
      throw new Error(`Unknown GoGoMail tool: ${name}`);
  }
}
```

- [ ] **Step 2: Type-check**

```bash
cd apps/mcp-support && npm run type-check
```

Expected: exit 0

- [ ] **Step 3: Commit**

```bash
git add apps/mcp-support/src/tools/gogomail.ts
git commit -m "feat(mcp-support): add 18 GoGoMail MCP tools with audit trail"
```

---

### Task 6: GitHub Client + Tools

**Goal:** `src/clients/github.ts` wrapping `@octokit/rest` and `src/tools/github.ts` with all 6 GitHub tools.

**Files:**
- Create: `apps/mcp-support/src/clients/github.ts`
- Create: `apps/mcp-support/src/tools/github.ts`

**Acceptance Criteria:**
- [ ] All 6 GitHub tools defined and dispatched
- [ ] `type-check` passes

**Verify:** `npm run type-check` → exit 0

**Steps:**

- [ ] **Step 1: Create src/clients/github.ts**

```typescript
import { Octokit } from "@octokit/rest";

export interface GithubIssue {
  number: number;
  title: string;
  body: string | null;
  state: string;
  labels: string[];
  url: string;
  createdAt: string;
  updatedAt: string;
}

export interface GithubIssueDetail extends GithubIssue {
  comments: Array<{
    id: number;
    body: string;
    author: string;
    createdAt: string;
  }>;
}

export class GithubClient {
  private readonly octokit: Octokit;
  private readonly owner: string;
  private readonly repo: string;

  constructor(token: string, repoSlug: string) {
    this.octokit = new Octokit({ auth: token });
    const [owner, repo] = repoSlug.split("/");
    if (!owner || !repo) throw new Error(`Invalid GITHUB_REPO: ${repoSlug}`);
    this.owner = owner;
    this.repo = repo;
  }

  private mapIssue(issue: {
    number: number;
    title: string;
    body?: string | null;
    state: string;
    labels: Array<{ name?: string } | string>;
    html_url: string;
    created_at: string;
    updated_at: string;
  }): GithubIssue {
    return {
      number: issue.number,
      title: issue.title,
      body: issue.body ?? null,
      state: issue.state,
      labels: issue.labels.map((l) =>
        typeof l === "string" ? l : (l.name ?? ""),
      ),
      url: issue.html_url,
      createdAt: issue.created_at,
      updatedAt: issue.updated_at,
    };
  }

  async searchIssues(params: {
    query: string;
    labels?: string[];
    state?: string;
  }): Promise<GithubIssue[]> {
    const parts = [
      `repo:${this.owner}/${this.repo}`,
      params.query,
    ];
    if (params.labels) {
      parts.push(...params.labels.map((l) => `label:"${l}"`));
    }
    if (params.state) parts.push(`state:${params.state}`);
    const { data } = await this.octokit.search.issuesAndPullRequests({
      q: parts.join(" "),
    });
    return data.items.map((i) => this.mapIssue(i));
  }

  async getIssue(issueNumber: number): Promise<GithubIssueDetail> {
    const [{ data: issue }, { data: comments }] = await Promise.all([
      this.octokit.issues.get({
        owner: this.owner,
        repo: this.repo,
        issue_number: issueNumber,
      }),
      this.octokit.issues.listComments({
        owner: this.owner,
        repo: this.repo,
        issue_number: issueNumber,
      }),
    ]);
    return {
      ...this.mapIssue(issue),
      comments: comments.map((c) => ({
        id: c.id,
        body: c.body ?? "",
        author: c.user?.login ?? "",
        createdAt: c.created_at,
      })),
    };
  }

  async listIssues(params: {
    labels?: string[];
    milestone?: string;
    state?: string;
  }): Promise<GithubIssue[]> {
    const { data } = await this.octokit.issues.listForRepo({
      owner: this.owner,
      repo: this.repo,
      labels: params.labels?.join(","),
      milestone: params.milestone,
      state: (params.state as "open" | "closed" | "all") ?? "open",
    });
    return data.map((i) => this.mapIssue(i));
  }

  async createIssue(params: {
    title: string;
    body: string;
    labels?: string[];
  }): Promise<GithubIssue> {
    const { data } = await this.octokit.issues.create({
      owner: this.owner,
      repo: this.repo,
      title: params.title,
      body: params.body,
      labels: params.labels,
    });
    return this.mapIssue(data);
  }

  async addComment(
    issueNumber: number,
    body: string,
  ): Promise<{ id: number; url: string }> {
    const { data } = await this.octokit.issues.createComment({
      owner: this.owner,
      repo: this.repo,
      issue_number: issueNumber,
      body,
    });
    return { id: data.id, url: data.html_url };
  }

  async updateIssue(
    issueNumber: number,
    params: { labels?: string[]; state?: string },
  ): Promise<GithubIssue> {
    const { data } = await this.octokit.issues.update({
      owner: this.owner,
      repo: this.repo,
      issue_number: issueNumber,
      labels: params.labels,
      state: params.state as "open" | "closed" | undefined,
    });
    return this.mapIssue(data);
  }
}
```

- [ ] **Step 2: Create src/tools/github.ts**

```typescript
import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GithubClient } from "../clients/github.js";

export const toolDefinitions: Tool[] = [
  {
    name: "github_search_issues",
    description: "Search GitHub issues by keyword, label, and state.",
    inputSchema: {
      type: "object",
      properties: {
        query: { type: "string" },
        labels: { type: "array", items: { type: "string" } },
        state: { type: "string", description: "open | closed | all" },
      },
      required: ["query"],
    },
  },
  {
    name: "github_get_issue",
    description: "Get a GitHub issue with its full comment thread.",
    inputSchema: {
      type: "object",
      properties: {
        issueNumber: { type: "number" },
      },
      required: ["issueNumber"],
    },
  },
  {
    name: "github_list_issues",
    description: "List GitHub issues filtered by label and/or milestone.",
    inputSchema: {
      type: "object",
      properties: {
        labels: { type: "array", items: { type: "string" } },
        milestone: { type: "string" },
        state: { type: "string", description: "open | closed | all" },
      },
    },
  },
  {
    name: "github_create_issue",
    description: "Create a new bug report or feature request on GitHub.",
    inputSchema: {
      type: "object",
      properties: {
        title: { type: "string" },
        body: { type: "string" },
        labels: { type: "array", items: { type: "string" } },
      },
      required: ["title", "body"],
    },
  },
  {
    name: "github_add_comment",
    description: "Add a comment to an existing GitHub issue.",
    inputSchema: {
      type: "object",
      properties: {
        issueNumber: { type: "number" },
        body: { type: "string" },
      },
      required: ["issueNumber", "body"],
    },
  },
  {
    name: "github_update_issue",
    description: "Update a GitHub issue's labels or state (open/closed).",
    inputSchema: {
      type: "object",
      properties: {
        issueNumber: { type: "number" },
        labels: { type: "array", items: { type: "string" } },
        state: { type: "string", description: "open | closed" },
      },
      required: ["issueNumber"],
    },
  },
];

const SearchSchema = z.object({
  query: z.string(),
  labels: z.array(z.string()).optional(),
  state: z.string().optional(),
});
const IssueNumberSchema = z.object({ issueNumber: z.number() });
const ListSchema = z.object({
  labels: z.array(z.string()).optional(),
  milestone: z.string().optional(),
  state: z.string().optional(),
});
const CreateSchema = z.object({
  title: z.string(),
  body: z.string(),
  labels: z.array(z.string()).optional(),
});
const AddCommentSchema = z.object({
  issueNumber: z.number(),
  body: z.string(),
});
const UpdateSchema = z.object({
  issueNumber: z.number(),
  labels: z.array(z.string()).optional(),
  state: z.string().optional(),
});

export async function callTool(
  client: GithubClient,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  switch (name) {
    case "github_search_issues": {
      const p = SearchSchema.parse(args);
      return client.searchIssues(p);
    }
    case "github_get_issue": {
      const { issueNumber } = IssueNumberSchema.parse(args);
      return client.getIssue(issueNumber);
    }
    case "github_list_issues": {
      const p = ListSchema.parse(args);
      return client.listIssues(p);
    }
    case "github_create_issue": {
      const p = CreateSchema.parse(args);
      return client.createIssue(p);
    }
    case "github_add_comment": {
      const { issueNumber, body } = AddCommentSchema.parse(args);
      return client.addComment(issueNumber, body);
    }
    case "github_update_issue": {
      const { issueNumber, ...rest } = UpdateSchema.parse(args);
      return client.updateIssue(issueNumber, rest);
    }
    default:
      throw new Error(`Unknown GitHub tool: ${name}`);
  }
}
```

- [ ] **Step 3: Type-check**

```bash
cd apps/mcp-support && npm run type-check
```

Expected: exit 0

- [ ] **Step 4: Commit**

```bash
git add apps/mcp-support/src/clients/github.ts apps/mcp-support/src/tools/github.ts
git commit -m "feat(mcp-support): add GitHub client and 6 MCP tools"
```

---

### Task 7: Server Entrypoint + Transport

**Goal:** `src/index.ts` that initializes all clients, registers all 34 tools, and connects either stdio or HTTP+SSE transport depending on `MCP_TRANSPORT`.

**Files:**
- Create: `apps/mcp-support/src/index.ts`

**Acceptance Criteria:**
- [ ] `npm run build` (tsc) exits 0 — dist/index.js produced
- [ ] `GOGOMAIL_ADMIN_URL=x GOGOMAIL_ADMIN_KEY=x SUPPO_API_URL=x SUPPO_API_KEY=x GITHUB_TOKEN=x node dist/index.js` starts without immediate crash (waits for MCP input on stdio)
- [ ] Tool count: running with `MCP_TRANSPORT=stdio` and sending a `tools/list` JSON-RPC request returns 34 tools

**Verify:**

```bash
cd apps/mcp-support && npm run build
```

Expected: `dist/index.js` exists, exit 0

**Steps:**

- [ ] **Step 1: Create src/index.ts**

```typescript
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { config } from "./config.js";
import { GogomailClient } from "./clients/gogomail.js";
import { SuppoClient } from "./clients/suppo.js";
import { GithubClient } from "./clients/github.js";
import * as suppoTools from "./tools/suppo.js";
import * as gogomailTools from "./tools/gogomail.js";
import * as githubTools from "./tools/github.js";

const gogomailClient = new GogomailClient(
  config.gogomail.adminUrl,
  config.gogomail.adminKey,
);
const suppoClient = new SuppoClient(config.suppo.apiUrl, config.suppo.apiKey);
const githubClient = new GithubClient(config.github.token, config.github.repo);

const allTools = [
  ...suppoTools.toolDefinitions,
  ...gogomailTools.toolDefinitions,
  ...githubTools.toolDefinitions,
];

const server = new Server(
  { name: "gogomail-support", version: "1.0.0" },
  { capabilities: { tools: {} } },
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: allTools,
}));

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args = {} } = request.params;
  const typedArgs = args as Record<string, unknown>;

  try {
    let result: unknown;

    if (name.startsWith("suppo_")) {
      result = await suppoTools.callTool(suppoClient, name, typedArgs);
    } else if (name.startsWith("gogomail_")) {
      result = await gogomailTools.callTool(
        gogomailClient,
        suppoClient,
        name,
        typedArgs,
      );
    } else if (name.startsWith("github_")) {
      result = await githubTools.callTool(githubClient, name, typedArgs);
    } else {
      throw new Error(`Unknown tool: ${name}`);
    }

    return {
      content: [{ type: "text", text: JSON.stringify(result, null, 2) }],
    };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return {
      content: [{ type: "text", text: `Error: ${message}` }],
      isError: true,
    };
  }
});

async function main(): Promise<void> {
  if (config.transport === "sse") {
    // HTTP+SSE transport for remote autonomous agent
    const { SSEServerTransport } = await import(
      "@modelcontextprotocol/sdk/server/sse.js"
    );
    const { createServer } = await import("http");

    const sessions = new Map<string, InstanceType<typeof SSEServerTransport>>();

    const httpServer = createServer((req, res) => {
      if (req.method === "GET" && req.url === "/sse") {
        const transport = new SSEServerTransport("/messages", res);
        sessions.set(transport.sessionId, transport);
        server.connect(transport).catch(console.error);
        req.on("close", () => sessions.delete(transport.sessionId));
        return;
      }

      if (req.method === "POST" && req.url?.startsWith("/messages")) {
        const url = new URL(req.url, `http://localhost`);
        const sessionId = url.searchParams.get("sessionId") ?? "";
        const transport = sessions.get(sessionId);
        if (!transport) {
          res.writeHead(404).end("Session not found");
          return;
        }
        let body = "";
        req.on("data", (chunk: Buffer) => (body += chunk));
        req.on("end", () => {
          transport.handlePostMessage(req, res, body).catch(console.error);
        });
        return;
      }

      res.writeHead(404).end();
    });

    httpServer.listen(config.port, () => {
      console.error(
        `[mcp-support] SSE transport listening on port ${config.port}`,
      );
    });
  } else {
    const transport = new StdioServerTransport();
    await server.connect(transport);
    console.error("[mcp-support] stdio transport ready");
  }
}

main().catch((err) => {
  console.error("[mcp-support] Fatal error:", err);
  process.exit(1);
});
```

- [ ] **Step 2: Build**

```bash
cd apps/mcp-support && npm run build
```

Expected: `dist/index.js` created, exit 0. Fix any type errors before continuing.

- [ ] **Step 3: Smoke test — tool count**

Send a `tools/list` request via stdin and verify 34 tools are returned:

```bash
cd apps/mcp-support
GOGOMAIL_ADMIN_URL=https://example.com \
GOGOMAIL_ADMIN_KEY=test \
SUPPO_API_URL=https://example.com \
SUPPO_API_KEY=test \
GITHUB_TOKEN=test \
node dist/index.js <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
EOF
```

Expected: JSON response contains `"tools"` array with 34 items.

Count check:
```bash
# Parse the response and count tools
... | grep '"name"' | wc -l
```

Expected: 34

- [ ] **Step 4: Commit**

```bash
git add apps/mcp-support/src/index.ts
git commit -m "feat(mcp-support): add server entrypoint with stdio/SSE transport"
```

---

### Task 8: Claude Desktop Config + README

**Goal:** Add a working Claude Desktop JSON snippet and minimal README so the server can be wired up immediately.

**Files:**
- Create: `apps/mcp-support/README.md`

**Acceptance Criteria:**
- [ ] README contains the Claude Desktop JSON config snippet with all required env vars
- [ ] README contains the SSE remote agent startup command
- [ ] README contains the list of all 34 tool names grouped by system

**Verify:** Human review — README is accurate and complete.

**Steps:**

- [ ] **Step 1: Create apps/mcp-support/README.md**

```markdown
# GoGoMail Support MCP Server

Autonomous AI support agent MCP server for GoGoMail. Exposes 34 tools across Suppo (helpdesk), GoGoMail Admin API, and GitHub Issues.

## Quick Start

### Build

```bash
cd apps/mcp-support
npm install
npm run build
```

### Claude Desktop (stdio)

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "gogomail-support": {
      "command": "node",
      "args": ["/absolute/path/to/apps/mcp-support/dist/index.js"],
      "env": {
        "GOGOMAIL_ADMIN_URL": "https://admin.gogomail.io",
        "GOGOMAIL_ADMIN_KEY": "...",
        "SUPPO_API_URL": "https://support.gogomail.io",
        "SUPPO_API_KEY": "crn_live_...",
        "GITHUB_TOKEN": "ghp_...",
        "GITHUB_REPO": "parkjangwon/gogomail"
      }
    }
  }
}
```

### Remote Autonomous Agent (HTTP+SSE)

```bash
GOGOMAIL_ADMIN_URL=... \
GOGOMAIL_ADMIN_KEY=... \
SUPPO_API_URL=... \
SUPPO_API_KEY=... \
GITHUB_TOKEN=... \
MCP_TRANSPORT=sse \
MCP_PORT=3100 \
node apps/mcp-support/dist/index.js
```

## Tools (34 total)

### Suppo (10)
- `suppo_list_tickets` — list tickets with optional status/priority filter
- `suppo_get_ticket` — get ticket detail + comment history
- `suppo_search_tickets` — search by customer email or keyword
- `suppo_create_ticket` — create new ticket
- `suppo_update_ticket` — change status/priority
- `suppo_add_comment` — add customer reply or internal memo
- `suppo_assign_ticket` — assign to an agent
- `suppo_list_agents` — list assignable agents
- `suppo_search_kb` — search knowledge base
- `suppo_create_kb_article` — create KB article from resolved case

### GoGoMail Admin (18)
**Read (9):** `gogomail_find_user` · `gogomail_get_user` · `gogomail_get_user_quota` · `gogomail_get_mail_logs` · `gogomail_trace_message` · `gogomail_get_delivery_attempts` · `gogomail_get_audit_logs` · `gogomail_list_user_sessions` · `gogomail_check_health`

**Action (9):** `gogomail_reset_password` · `gogomail_update_user_status` · `gogomail_update_user_quota` · `gogomail_revoke_sessions` · `gogomail_update_user_role` · `gogomail_get_company` · `gogomail_get_domain_settings` · `gogomail_update_domain_settings` · `gogomail_get_alert_events`

### GitHub (6)
- `github_search_issues` · `github_get_issue` · `github_list_issues` · `github_create_issue` · `github_add_comment` · `github_update_issue`

## Audit Trail

All GoGoMail action tools auto-write an internal Suppo comment after execution. Pass `ticketId` to attach the audit memo to the active ticket. Without `ticketId`, a standalone audit ticket is created.
```

- [ ] **Step 2: Commit**

```bash
git add apps/mcp-support/README.md
git commit -m "docs(mcp-support): add README with Claude Desktop config and tool list"
```

---

## Suppo New API Endpoints Required

These endpoints must be added to the Suppo project (`parkjangwon/suppo`) before the MCP server can be used end-to-end. They are already designed in the spec at `docs/superpowers/specs/2026-05-23-support-mcp-server-design.md` §6.

| Method | Path | Used by |
|---|---|---|
| `POST` | `/api/public/tickets/{id}/comments` | `suppo_add_comment` |
| `GET` | `/api/public/agents` | `suppo_list_agents` |
| `GET` | `/api/public/kb/articles/search?q=` | `suppo_search_kb` |
| `POST` | `/api/public/kb/articles` | `suppo_create_kb_article` |

The MCP server clients are already wired to these paths — implement them in Suppo to complete end-to-end functionality.

---

```json:metadata
{
  "tasks": [
    {
      "id": "task-1",
      "subject": "Task 1: Project Scaffold",
      "files": ["apps/mcp-support/package.json", "apps/mcp-support/tsconfig.json", "apps/mcp-support/src/config.ts"],
      "verifyCommand": "cd apps/mcp-support && npm install && npm run type-check",
      "acceptanceCriteria": ["npm install succeeds", "type-check exits 0"]
    },
    {
      "id": "task-2",
      "subject": "Task 2: Suppo API Client",
      "files": ["apps/mcp-support/src/clients/suppo.ts"],
      "verifyCommand": "cd apps/mcp-support && npm run type-check",
      "acceptanceCriteria": ["All 10 Suppo operations have a client method", "type-check exits 0"]
    },
    {
      "id": "task-3",
      "subject": "Task 3: Suppo MCP Tools",
      "files": ["apps/mcp-support/src/tools/suppo.ts"],
      "verifyCommand": "cd apps/mcp-support && npm run type-check",
      "acceptanceCriteria": ["10 tools in toolDefinitions", "callTool handles all 10", "type-check exits 0"]
    },
    {
      "id": "task-4",
      "subject": "Task 4: GoGoMail Admin API Client",
      "files": ["apps/mcp-support/src/clients/gogomail.ts"],
      "verifyCommand": "cd apps/mcp-support && npm run type-check",
      "acceptanceCriteria": ["18 tool operations have client methods", "type-check exits 0"]
    },
    {
      "id": "task-5",
      "subject": "Task 5: GoGoMail MCP Tools (18)",
      "files": ["apps/mcp-support/src/tools/gogomail.ts"],
      "verifyCommand": "cd apps/mcp-support && npm run type-check",
      "acceptanceCriteria": ["18 tools in toolDefinitions", "action tools write audit comments", "type-check exits 0"]
    },
    {
      "id": "task-6",
      "subject": "Task 6: GitHub Client + Tools",
      "files": ["apps/mcp-support/src/clients/github.ts", "apps/mcp-support/src/tools/github.ts"],
      "verifyCommand": "cd apps/mcp-support && npm run type-check",
      "acceptanceCriteria": ["6 GitHub tools", "type-check exits 0"]
    },
    {
      "id": "task-7",
      "subject": "Task 7: Server Entrypoint + Transport",
      "files": ["apps/mcp-support/src/index.ts"],
      "verifyCommand": "cd apps/mcp-support && npm run build",
      "acceptanceCriteria": ["build exits 0", "tools/list returns 34 tools", "no crash on startup"]
    },
    {
      "id": "task-8",
      "subject": "Task 8: Claude Desktop Config + README",
      "files": ["apps/mcp-support/README.md"],
      "verifyCommand": "cat apps/mcp-support/README.md",
      "acceptanceCriteria": ["README has Claude Desktop JSON snippet", "README lists all 34 tools"]
    }
  ]
}
```
