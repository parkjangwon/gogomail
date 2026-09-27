import { ERROR_MESSAGES } from "./error-handler";

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

type QueryValue = string | number | boolean;

interface FetchOptions extends RequestInit {
  params?: Record<string, QueryValue | QueryValue[] | null | undefined>;
}

/**
 * Serialize query params for a request URL.
 * - Empty param objects produce no query string (no dangling `?`).
 * - `undefined` / `null` values are skipped (never serialized as the literal
 *   string `"undefined"`).
 * - Array values are appended as repeated keys (`k=a&k=b`) so multi-select
 *   filters round-trip correctly.
 */
function buildQueryString(
  params: Record<string, QueryValue | QueryValue[] | null | undefined>
): string {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null) continue;
    if (Array.isArray(value)) {
      for (const item of value) {
        if (item === undefined || item === null) continue;
        qs.append(key, String(item));
      }
    } else {
      qs.append(key, String(value));
    }
  }
  return qs.toString();
}

export async function apiClient<T>(
  path: string,
  options: FetchOptions = {}
): Promise<T> {
  const { params, ...fetchOpts } = options;

  let url = `/api/admin${path}`;
  if (params) {
    const qs = buildQueryString(params);
    if (qs) url += `?${qs}`;
  }

  let response: Response;
  try {
    response = await fetch(url, {
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...fetchOpts.headers,
      },
      ...fetchOpts,
    });
  } catch {
    // fetch rejects (e.g. offline, DNS failure, CORS) with a raw TypeError.
    // Wrap it so callers get the designed network-error UX instead of a raw
    // browser error leaking through.
    throw new AdminApiError(0, ERROR_MESSAGES.NETWORK);
  }

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

  // Only parse JSON when the server actually returned JSON. A 2xx with an
  // empty or text body (e.g. 201/202 with no content-type) would otherwise
  // throw on response.json().
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    return response.json();
  }

  const text = await response.text();
  return (text ? (text as unknown) : (undefined as unknown)) as T;
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
