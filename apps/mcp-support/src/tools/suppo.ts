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
