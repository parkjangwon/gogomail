import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { SuppoClient } from "../clients/suppo.js";

export type OptionalSuppo = SuppoClient | null;

const NOT_CONFIGURED = "Suppo is not configured. Set SUPPO_API_URL and SUPPO_API_KEY environment variables to enable helpdesk integration.";

export const toolDefinitions: Tool[] = [
  {
    name: "suppo_list_tickets",
    description: "List helpdesk tickets. Filter by status (open/pending/closed/resolved) and/or priority (low/normal/high/urgent).",
    inputSchema: {
      type: "object",
      properties: {
        status: { type: "string", description: "Filter by ticket status", enum: ["open", "pending", "resolved", "closed"] },
        priority: { type: "string", description: "Filter by priority", enum: ["low", "normal", "high", "urgent"] },
        limit: { type: "number", description: "Max results (default 20)", minimum: 1, maximum: 200 },
      },
    },
  },
  {
    name: "suppo_get_ticket",
    description: "Get full ticket detail including all comment history.",
    inputSchema: {
      type: "object",
      properties: {
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
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
        customerEmail: { type: "string", format: "email", maxLength: 254 },
        query: { type: "string", maxLength: 500 },
      },
    },
  },
  {
    name: "suppo_create_ticket",
    description: "Create a new helpdesk ticket (e.g. for internally-discovered issues).",
    inputSchema: {
      type: "object",
      properties: {
        customerName: { type: "string", maxLength: 256 },
        customerEmail: { type: "string", format: "email", maxLength: 254 },
        subject: { type: "string", maxLength: 512 },
        description: { type: "string", maxLength: 10000 },
        priority: { type: "string", description: "low | normal | high | urgent", enum: ["low", "normal", "high", "urgent"] },
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
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        status: { type: "string", enum: ["open", "pending", "resolved", "closed"] },
        priority: { type: "string", enum: ["low", "normal", "high", "urgent"] },
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
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        body: { type: "string", maxLength: 10000 },
        internal: { type: "boolean", description: "true = internal memo, false = customer-visible reply. Defaults to true." },
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
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        assigneeId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
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
        query: { type: "string", maxLength: 500 },
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
        title: { type: "string", maxLength: 512 },
        content: { type: "string", maxLength: 100000 },
      },
      required: ["title", "content"],
    },
  },
];

const safeId = () => z.string().regex(/^[A-Za-z0-9_-]+$/).max(128);
const ticketStatus = z.enum(["open", "pending", "resolved", "closed"]);
const priority = z.enum(["low", "normal", "high", "urgent"]);
const nonEmptyText = (max: number) => z.string().trim().min(1).max(max);

const ListTicketsSchema = z.object({
  status: ticketStatus.optional(),
  priority: priority.optional(),
  limit: z.number().int().min(1).max(200).optional(),
});

const TicketIdSchema = z.object({ ticketId: safeId() });

const SearchTicketsSchema = z.object({
  customerEmail: z.string().email().max(254).optional(),
  query: nonEmptyText(500).optional(),
}).refine((p) => p.customerEmail || p.query, {
  message: "customerEmail or query is required",
});

const CreateTicketSchema = z.object({
  customerName: nonEmptyText(256),
  customerEmail: z.string().email().max(254),
  subject: nonEmptyText(512),
  description: nonEmptyText(10_000),
  priority: priority.optional(),
});

const UpdateTicketSchema = z.object({
  ticketId: safeId(),
  status: ticketStatus.optional(),
  priority: priority.optional(),
}).refine((p) => p.status || p.priority, {
  message: "status or priority is required",
});

const AddCommentSchema = z.object({
  ticketId: safeId(),
  body: nonEmptyText(10_000),
  internal: z.boolean().default(true),
});

const AssignTicketSchema = z.object({
  ticketId: safeId(),
  assigneeId: safeId(),
});

const SearchKbSchema = z.object({ query: nonEmptyText(500) });

const CreateKbArticleSchema = z.object({
  title: nonEmptyText(512),
  content: nonEmptyText(100_000),
});

export async function callTool(
  client: OptionalSuppo,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  if (!client) throw new Error(NOT_CONFIGURED);
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
