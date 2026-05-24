import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GithubClient } from "../clients/github.js";

export type OptionalGithub = GithubClient | null;

const NOT_CONFIGURED = "GitHub is not configured. Set GITHUB_TOKEN environment variable to enable GitHub Issues integration.";
const labelSchema = z.string().trim().min(1).max(128).regex(/^[^\r\n]+$/, "label must be a single line");

export const toolDefinitions: Tool[] = [
  {
    name: "github_search_issues",
    description: "Search GitHub issues by keyword, label, and state.",
    inputSchema: {
      type: "object",
      properties: {
        query: { type: "string", maxLength: 1000 },
        labels: { type: "array", items: { type: "string", maxLength: 128 }, maxItems: 20 },
        state: { type: "string", description: "open | closed | all", enum: ["open", "closed", "all"] },
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
        issueNumber: { type: "number", minimum: 1 },
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
        labels: { type: "array", items: { type: "string", maxLength: 128 }, maxItems: 20 },
        milestone: { type: "string", maxLength: 256 },
        state: { type: "string", description: "open | closed | all", enum: ["open", "closed", "all"] },
      },
    },
  },
  {
    name: "github_create_issue",
    description: "Create a new bug report or feature request on GitHub.",
    inputSchema: {
      type: "object",
      properties: {
        title: { type: "string", maxLength: 512 },
        body: { type: "string", maxLength: 65535 },
        labels: { type: "array", items: { type: "string", maxLength: 128 }, maxItems: 20 },
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
        issueNumber: { type: "number", minimum: 1 },
        body: { type: "string", maxLength: 65535 },
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
        issueNumber: { type: "number", minimum: 1 },
        labels: { type: "array", items: { type: "string", maxLength: 128 }, maxItems: 20 },
        state: { type: "string", description: "open | closed", enum: ["open", "closed"] },
      },
      required: ["issueNumber"],
    },
  },
];

const issueNum = () => z.number().int().min(1);
const SearchSchema = z.object({
  query: z.string().trim().min(1).max(1000),
  labels: z.array(labelSchema).max(20).optional(),
  state: z.enum(["open", "closed", "all"]).optional(),
});
const IssueNumberSchema = z.object({ issueNumber: issueNum() });
const ListSchema = z.object({
  labels: z.array(labelSchema).max(20).optional(),
  milestone: z.string().trim().min(1).max(256).optional(),
  state: z.enum(["open", "closed", "all"]).optional(),
});
const CreateSchema = z.object({
  title: z.string().trim().min(1).max(512),
  body: z.string().trim().min(1).max(65_535),
  labels: z.array(labelSchema).max(20).optional(),
});
const AddCommentSchema = z.object({
  issueNumber: issueNum(),
  body: z.string().trim().min(1).max(65_535),
});
const UpdateSchema = z.object({
  issueNumber: issueNum(),
  labels: z.array(labelSchema).max(20).optional(),
  state: z.enum(["open", "closed"]).optional(),
}).refine((p) => p.labels !== undefined || p.state !== undefined, {
  message: "labels or state is required",
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
