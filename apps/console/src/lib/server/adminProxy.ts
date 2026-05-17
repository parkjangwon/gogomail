import { cookies } from 'next/headers';

const MUTATING_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);
const REQUEST_HEADER_ALLOWLIST = new Set(['accept', 'content-type']);
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
  if (referer && new URL(referer).origin !== url.origin) {
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
  let encodedPath: string;
  try {
    assertSameOriginRequest(req);
    encodedPath = encodeProxyPath(path);
  } catch {
    return Response.json({ error: 'Invalid proxy request' }, { status: 400 });
  }
  const reqUrl = new URL(req.url);
  const url = `${backendURL}/admin/v1/${encodedPath}${reqUrl.search}`;
  const cookieStore = await cookies();
  const token = cookieStore.get('admin_access_token')?.value;
  const headers = requestHeadersForBackend(req, token);
  const hasRequestBody = req.method !== 'GET' && req.method !== 'HEAD';
  const body = hasRequestBody ? await req.arrayBuffer() : undefined;
  if (body && body.byteLength === 0) headers.delete('content-type');

  try {
    const response = await fetch(url, {
      method: req.method,
      headers,
      body,
    });
    const responseHeaders = responseHeadersFromBackend(response);
    if (response.status === 204) {
      return new Response(null, { status: 204, headers: responseHeaders });
    }
    const contentType = response.headers.get('content-type') ?? '';
    if (DOWNLOAD_CONTENT_TYPES.some((type) => contentType.includes(type))) {
      return new Response(await response.arrayBuffer(), { status: response.status, headers: responseHeaders });
    }
    const responseBody = contentType.includes('application/json') ? await response.json() : await response.text();
    return Response.json(responseBody, { status: response.status, headers: responseHeaders });
  } catch (error) {
    console.error('Admin proxy error:', error);
    return Response.json({ error: 'Failed to proxy request to backend' }, { status: 500 });
  }
}
