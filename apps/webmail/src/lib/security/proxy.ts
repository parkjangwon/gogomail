const MUTATING_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);
const FORWARDED_HEADER_ALLOWLIST = new Set(['content-type', 'content-range', 'x-content-sha256', 'range', 'x-request-id']);

export function encodeBackendPath(path: string[]): string {
  return path.map((segment) => {
    if (!segment || segment.includes('/') || segment.includes('\\') || segment.includes('\0')) {
      throw new Error('Invalid proxy path');
    }
    return encodeURIComponent(segment);
  }).join('/');
}

function requestPublicOrigin(requestUrl: string, headers: Headers): string {
  const url = new URL(requestUrl);
  const forwardedHost = headers.get('x-forwarded-host')?.split(',')[0]?.trim();
  const host = forwardedHost || headers.get('host') || url.host;
  const forwardedProto = headers.get('x-forwarded-proto')?.split(',')[0]?.trim();
  const proto = forwardedProto || url.protocol.replace(':', '');
  return `${proto}://${host}`;
}

export function assertSameOriginForMutation(method: string, requestUrl: string, headers: Headers): void {
  if (!MUTATING_METHODS.has(method)) return;
  const expectedOrigin = requestPublicOrigin(requestUrl, headers);
  const origin = headers.get('origin');
  if (origin) {
    if (origin !== expectedOrigin) throw new Error('Invalid request origin');
    return;
  }
  const referer = headers.get('referer');
  if (!referer) throw new Error('Missing request origin');
  if (new URL(referer).origin !== expectedOrigin) {
    throw new Error('Invalid request origin');
  }
}

export function headersForBackend(input: Headers, token?: string, publicRoute = false): Headers {
  const headers = new Headers();
  for (const [name, value] of input) {
    if (FORWARDED_HEADER_ALLOWLIST.has(name.toLowerCase())) headers.set(name, value);
  }
  if (token && !publicRoute) headers.set('Authorization', `Bearer ${token}`);
  return headers;
}
