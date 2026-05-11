export interface Folder {
  id: string;
  name: string;
  full_path: string;
  type: string;
  system_type?: string;
  total: number;
  unread: number;
  starred: number;
}

export interface MessageSummary {
  id: string;
  subject: string;
  from_addr: string;
  from_name: string;
  received_at: string;
  read: boolean;
  starred: boolean;
  has_attachment: boolean;
  preview: string;
}

export interface MessageDetail {
  id: string;
  subject: string;
  from_addr: string;
  from_name: string;
  to_addrs: { address: string; name?: string }[];
  cc_addrs?: { address: string; name?: string }[];
  received_at: string;
  text_body: string;
  has_attachment: boolean;
}

export interface AuthTokenResponse {
  token: string;
  expires_at: string;
  must_change_password: boolean;
}

export interface SendMessageRequest {
  to: { address: string; name?: string }[];
  subject: string;
  text_body: string;
  from?: string;
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

function apiGet<T>(path: string, params?: Record<string, string>): Promise<T> {
  const search = params ? '?' + new URLSearchParams(params).toString() : '';
  return request<T>(`${path}${search}`, { method: 'GET' });
}

function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'PATCH',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

function apiDelete<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' });
}

export function getFolders(): Promise<{ folders: Folder[] }> {
  return apiGet<{ folders: Folder[] }>('folders');
}

export function getMessages(
  folderId: string,
  cursor = '',
  limit = 50
): Promise<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }> {
  const params: Record<string, string> = {
    folder_id: folderId,
    limit: String(limit),
  };
  if (cursor) params.cursor = cursor;
  return apiGet<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }>(
    'messages',
    params
  );
}

export async function getMessage(id: string): Promise<MessageDetail> {
  const res = await apiGet<{ message: MessageDetail }>(`messages/${id}`);
  return res.message;
}

export function markRead(id: string, value: boolean): Promise<{ status: string }> {
  return apiPatch<{ status: string }>(`messages/${id}/flags`, { flag: 'read', value });
}

export function deleteMessage(id: string): Promise<void> {
  return apiDelete<void>(`messages/${id}`);
}

export function sendMessage(data: SendMessageRequest): Promise<void> {
  return apiPost<void>('messages/send', data);
}
