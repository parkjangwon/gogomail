import { cookies } from 'next/headers';
import { ADMIN_ACCESS_TOKEN_COOKIE, LEGACY_ADMIN_ACCESS_TOKEN_COOKIE } from './cookies';
import {
  logServerRequest,
  requestIDFromHeaders,
  responseHeadersWithRequestID,
  setRequestIDHeader,
} from './requestLog';

const MUTATING_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);
const REQUEST_HEADER_ALLOWLIST = new Set(['accept', 'content-type', 'x-request-id']);
const DOWNLOAD_CONTENT_TYPES = ['text/csv', 'application/octet-stream'];

export function encodeProxyPath(path: string[]): string {
  return path.map((segment) => {
    if (!segment || segment.includes('/') || segment.includes('\\') || segment.includes('\0')) {
      throw new Error('Invalid proxy path');
    }
    return encodeURIComponent(segment);
  }).join('/');
}

export function assertSameOriginRequest(req: Request): void {
  if (!MUTATING_METHODS.has(req.method)) return;
  const url = new URL(req.url);
  const origin = req.headers.get('origin');
  if (origin) {
    if (origin !== url.origin) throw new Error('Invalid request origin');
    return;
  }
  const referer = req.headers.get('referer');
  if (!referer) throw new Error('Missing request origin');
  if (new URL(referer).origin !== url.origin) {
    throw new Error('Invalid request origin');
  }
}

export function requestHeadersForBackend(req: Request, token?: string): Headers {
  const headers = new Headers();
  for (const [name, value] of req.headers) {
    if (REQUEST_HEADER_ALLOWLIST.has(name.toLowerCase())) headers.set(name, value);
  }
  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (req.method === 'GET' || req.method === 'HEAD') headers.delete('content-type');
  return headers;
}

export function responseHeadersFromBackend(response: Response): Record<string, string> {
  const contentType = response.headers.get('content-type') || 'application/json';
  const headers: Record<string, string> = {
    'content-type': contentType,
    'cache-control': 'no-store',
    'x-content-type-options': 'nosniff',
  };
  const contentDisposition = response.headers.get('content-disposition') ?? '';
  if (contentDisposition && DOWNLOAD_CONTENT_TYPES.some((type) => contentType.includes(type))) {
    headers['content-disposition'] = contentDisposition.replace(/[\r\n]/g, ' ');
  }
  return headers;
}

export async function adminProxyHandler(
  req: Request,
  path: string[],
  backendURL: string,
): Promise<Response> {
  const started = Date.now();
  const requestID = requestIDFromHeaders(req.headers);
  const route = `/api/admin/${path.join('/')}`;
  const finish = (response: Response, upstreamStatus?: number): Response => {
    logServerRequest({
      requestID,
      method: req.method,
      route,
      status: response.status,
      durationMs: Date.now() - started,
      upstreamStatus,
    });
    return response;
  };
  let encodedPath: string;
  try {
    assertSameOriginRequest(req);
    encodedPath = encodeProxyPath(path);
  } catch {
    return finish(Response.json({ error: 'Invalid proxy request' }, {
      status: 400,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }));
  }
  const reqUrl = new URL(req.url);
  const url = `${backendURL}/admin/v1/${encodedPath}${reqUrl.search}`;
  const cookieStore = await cookies();
  const token = cookieStore.get(ADMIN_ACCESS_TOKEN_COOKIE)?.value
    ?? cookieStore.get(LEGACY_ADMIN_ACCESS_TOKEN_COOKIE)?.value;
  const headers = setRequestIDHeader(requestHeadersForBackend(req, token), requestID);
  const hasRequestBody = req.method !== 'GET' && req.method !== 'HEAD';
  const body = hasRequestBody ? await req.arrayBuffer() : undefined;
  if (body && body.byteLength === 0) headers.delete('content-type');

  try {
    const response = await fetch(url, {
      method: req.method,
      headers,
      body,
    });
    const responseHeaders = responseHeadersWithRequestID(responseHeadersFromBackend(response), requestID);
    if (response.status === 204) {
      return finish(new Response(null, { status: 204, headers: responseHeaders }), response.status);
    }
    const contentType = response.headers.get('content-type') ?? '';
    if (DOWNLOAD_CONTENT_TYPES.some((type) => contentType.includes(type))) {
      return finish(new Response(await response.arrayBuffer(), { status: response.status, headers: responseHeaders }), response.status);
    }
    const responseBody = contentType.includes('application/json') ? await response.json() : await response.text();
    return finish(Response.json(responseBody, { status: response.status, headers: responseHeaders }), response.status);
  } catch (error) {
    logServerRequest({
      requestID,
      method: req.method,
      route,
      status: 500,
      durationMs: Date.now() - started,
      error,
    });
    return Response.json({ error: 'Failed to proxy request to backend' }, {
      status: 500,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    });
  }
}
