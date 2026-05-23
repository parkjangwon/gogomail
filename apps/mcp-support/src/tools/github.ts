import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GithubClient } from "../clients/github.js";

export type OptionalGithub = GithubClient | null;

const NOT_CONFIGURED = "GitHub is not configured. Set GITHUB_TOKEN environment variable to enable GitHub Issues integration.";

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
  client: OptionalGithub,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  if (!client) throw new Error(NOT_CONFIGURED);
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
