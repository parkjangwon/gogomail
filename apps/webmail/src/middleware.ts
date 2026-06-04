import { NextResponse, type NextRequest } from 'next/server';

const REQUEST_ID_HEADER = 'x-request-id';
const REQUEST_ID_RESPONSE_HEADER = 'X-Request-ID';
const MAX_REQUEST_ID_LENGTH = 128;
export function middleware(req: NextRequest) {
  const requestID =
    sanitizeRequestID(req.headers.get(REQUEST_ID_HEADER)) || crypto.randomUUID();
  const isDev = process.env.NODE_ENV === 'development';
  // These apps currently serve statically prerendered Next.js HTML in
  // production (x-nextjs-prerender: 1). Next's bootstrap/data scripts in that
  // HTML are inline and cannot receive a per-request nonce after prerendering,
  // so a nonce-only script-src leaves the client unhydrated on first load.
  // Keep the policy strict for everything else, but allow Next's required
  // inline bootstrap scripts until these routes are made fully dynamic or the
  // scripts are covered by stable hashes.
  const cspHeader = [
    "default-src 'self'",
    `script-src 'self' 'unsafe-inline'${isDev ? " 'unsafe-eval'" : ''}`,
    "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
    "img-src 'self' data: blob:",
    "connect-src 'self'",
    "font-src 'self' data: https://fonts.gstatic.com",
    "frame-src 'none'",
    "frame-ancestors 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "upgrade-insecure-requests",
  ].join('; ');

  const requestHeaders = new Headers(req.headers);
  requestHeaders.set(REQUEST_ID_HEADER, requestID);

  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set(REQUEST_ID_RESPONSE_HEADER, requestID);
  response.headers.set('Content-Security-Policy', cspHeader);
  return response;
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
};

function sanitizeRequestID(value: string | null): string {
  const trimmed = (value ?? '').trim();
  if (!trimmed || trimmed.length > MAX_REQUEST_ID_LENGTH) return '';
  if (!/^[A-Za-z0-9._:-]+$/.test(trimmed)) return '';
  return trimmed;
}
