export interface Folder {
  id: string;
  name: string;
  unread_count: number;
  total_count: number;
}

export interface EmailAddress {
  name: string;
  email: string;
}

export interface MessageSummary {
  id: string;
  subject: string;
  from: EmailAddress;
  date: string;
  is_read: boolean;
  is_starred: boolean;
  preview: string;
  folder_id: string;
}

export interface MessageDetail extends MessageSummary {
  body_html?: string;
  body_text?: string;
  to: EmailAddress[];
  cc?: EmailAddress[];
}

export interface AuthTokenResponse {
  token: string;
  expires_at: string;
  must_change_password: boolean;
}

export interface SendMessageRequest {
  to: string;
  subject: string;
  body: string;
}

function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('webmail_token');
}

function clearTokenAndRedirect(): void {
  localStorage.removeItem('webmail_token');
  window.location.href = '/login';
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers = new Headers(options.headers as HeadersInit | undefined);

  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json');
  }

  const res = await fetch(`/api/mail/${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    clearTokenAndRedirect();
    throw new Error('Unauthorized');
  }

  if (!res.ok) {
    let message = `Request failed: ${res.status}`;
    try {
      const errBody = (await res.json()) as { error?: string; message?: string };
      message = errBody.error ?? errBody.message ?? message;
    } catch {
      // ignore parse error
    }
    throw new Error(message);
  }

  if (res.status === 204) {
    return undefined as unknown as T;
  }

  return res.json() as Promise<T>;
}

export async function loginUser(
  email: string,
  password: string
): Promise<AuthTokenResponse> {
  const res = await fetch('/api/mail/auth/token', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });

  if (!res.ok) {
    let message = '로그인에 실패했습니다.';
    try {
      const errBody = (await res.json()) as { error?: string; message?: string };
      message = errBody.error ?? errBody.message ?? message;
    } catch {
      // ignore
    }
    throw new Error(message);
  }

  return res.json() as Promise<AuthTokenResponse>;
}

export function apiGet<T>(path: string, params?: Record<string, string>): Promise<T> {
  const search = params ? '?' + new URLSearchParams(params).toString() : '';
  return request<T>(`${path}${search}`, { method: 'GET' });
}

export function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

export function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'PATCH',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

export function apiDelete<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' });
}

export function getFolders(): Promise<{ folders: Folder[] }> {
  return apiGet<{ folders: Folder[] }>('folders');
}

export function getMessages(
  folderId: string,
  page = 1,
  perPage = 50
): Promise<{ messages: MessageSummary[]; total: number }> {
  return apiGet<{ messages: MessageSummary[]; total: number }>('messages', {
    folder_id: folderId,
    page: String(page),
    per_page: String(perPage),
  });
}

export function getMessage(id: string): Promise<MessageDetail> {
  return apiGet<MessageDetail>(`messages/${id}`);
}

export function markRead(id: string, isRead: boolean): Promise<MessageDetail> {
  return apiPatch<MessageDetail>(`messages/${id}`, { is_read: isRead });
}

export function deleteMessage(id: string): Promise<void> {
  return apiDelete<void>(`messages/${id}`);
}

export function sendMessage(data: SendMessageRequest): Promise<void> {
  return apiPost<void>('messages/send', data);
}
