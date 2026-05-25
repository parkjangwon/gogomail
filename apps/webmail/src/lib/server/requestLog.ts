const REQUEST_ID_HEADER = 'x-request-id';
const REQUEST_ID_RESPONSE_HEADER = 'X-Request-ID';
const MAX_REQUEST_ID_LENGTH = 128;

export function requestIDFromHeaders(headers: Headers): string {
  const forwarded = sanitizeRequestID(headers.get(REQUEST_ID_HEADER));
  if (forwarded) return forwarded;
  return crypto.randomUUID();
}

export function setRequestIDHeader(headers: Headers, requestID: string): Headers {
  headers.set(REQUEST_ID_HEADER, requestID);
  return headers;
}

export function responseHeadersWithRequestID(headers: Record<string, string>, requestID: string): Record<string, string> {
  return { ...headers, [REQUEST_ID_RESPONSE_HEADER]: requestID };
}

export function logServerRequest(input: {
  requestID: string;
  method: string;
  route: string;
  status: number;
  durationMs: number;
  upstreamStatus?: number;
  error?: unknown;
}): void {
  const entry: Record<string, unknown> = {
    app: 'webmail',
    component: 'next-api',
    request_id: input.requestID,
    method: input.method,
    route: input.route,
    status: input.status,
    duration_ms: Math.max(0, Math.round(input.durationMs)),
  };
  if (input.upstreamStatus !== undefined) entry.upstream_status = input.upstreamStatus;
  if (input.error !== undefined) entry.error = errorMessage(input.error);
  const line = JSON.stringify(entry);
  if (input.status >= 500) {
    console.error(line);
    return;
  }
  console.log(line);
}

function sanitizeRequestID(value: string | null): string {
  const trimmed = (value ?? '').trim();
  if (!trimmed || trimmed.length > MAX_REQUEST_ID_LENGTH) return '';
  if (!/^[A-Za-z0-9._:-]+$/.test(trimmed)) return '';
  return trimmed;
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === 'string') return error;
  return 'unknown error';
}
