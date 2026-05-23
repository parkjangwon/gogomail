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
      parts.push(...params.labels.map((l) => `label:"${l.replace(/"/g, "")}"`));
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
