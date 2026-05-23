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
