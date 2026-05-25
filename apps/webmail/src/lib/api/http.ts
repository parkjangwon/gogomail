// Private HTTP helpers shared across all domain modules.

type APIErrorBody = {
  error?: string | { message?: string; code?: string; status_text?: string };
  error_message?: string;
  message?: string;
};

function messageFromAPIErrorBody(body: APIErrorBody, fallback: string): string {
  if (typeof body.error_message === 'string' && body.error_message.trim()) return body.error_message;
  if (typeof body.error === 'string' && body.error.trim()) return body.error;
  if (typeof body.error === 'object' && typeof body.error.message === 'string' && body.error.message.trim()) {
    return body.error.message;
  }
  if (typeof body.message === 'string' && body.message.trim()) return body.message;
  return fallback;
}

export async function responseErrorMessage(res: Response, fallback: string): Promise<string> {
  try {
    return messageFromAPIErrorBody((await res.json()) as APIErrorBody, fallback);
  } catch {
    return fallback;
  }
}

export function clearTokenAndRedirect(): void {
  fetch('/api/auth/logout', { method: 'POST' }).catch(() => {});
  localStorage.removeItem('webmail_authenticated');
  localStorage.removeItem('webmail_email');
  localStorage.removeItem('webmail_token_expires_at');
  localStorage.removeItem('webmail_must_change_password');
  window.location.href = '/login';
}

export async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const headers = new Headers(options.headers as HeadersInit | undefined);
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json');
  }

  const res = await fetch(`/api/mail/${path}`, {
    ...options,
    headers,
    signal: options.signal ?? AbortSignal.timeout(30_000),
  });

  if (res.status === 401) {
    clearTokenAndRedirect();
    throw new Error('Unauthorized');
  }

  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, `Request failed: ${res.status}`));
  }

  if (res.status === 204) {
    return undefined as unknown as T;
  }

  return res.json() as Promise<T>;
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
