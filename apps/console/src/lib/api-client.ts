export class AdminApiError extends Error {
  constructor(
    public status: number,
    public message: string,
    public details?: Record<string, unknown>
  ) {
    super(message);
    this.name = "AdminApiError";
  }
}

interface FetchOptions extends RequestInit {
  params?: Record<string, string | number | boolean>;
}

export async function apiClient<T>(
  path: string,
  options: FetchOptions = {}
): Promise<T> {
  const { params, ...fetchOpts } = options;

  let url = `/api/admin${path}`;
  if (params) {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      qs.append(key, String(value));
    });
    url += `?${qs.toString()}`;
  }

  const response = await fetch(url, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...fetchOpts.headers,
    },
    ...fetchOpts,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new AdminApiError(
      response.status,
      error.error || "Request failed",
      error
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

export const api = {
  get: <T,>(path: string, options?: FetchOptions) =>
    apiClient<T>(path, { ...options, method: "GET" }),

  post: <T,>(path: string, data?: unknown, options?: FetchOptions) =>
    apiClient<T>(path, {
      ...options,
      method: "POST",
      body: data ? JSON.stringify(data) : undefined,
    }),

  put: <T,>(path: string, data?: unknown, options?: FetchOptions) =>
    apiClient<T>(path, {
      ...options,
      method: "PUT",
      body: data ? JSON.stringify(data) : undefined,
    }),

  patch: <T,>(path: string, data?: unknown, options?: FetchOptions) =>
    apiClient<T>(path, {
      ...options,
      method: "PATCH",
      body: data ? JSON.stringify(data) : undefined,
    }),

  delete: <T,>(path: string, options?: FetchOptions) =>
    apiClient<T>(path, { ...options, method: "DELETE" }),
};
