import { NextResponse, type NextRequest } from 'next/server';

const REQUEST_ID_HEADER = 'x-request-id';
const REQUEST_ID_RESPONSE_HEADER = 'X-Request-ID';
const MAX_REQUEST_ID_LENGTH = 128;
const NONCE_HEADER = 'x-nonce';

export function middleware(req: NextRequest) {
  const requestID =
    sanitizeRequestID(req.headers.get(REQUEST_ID_HEADER)) || crypto.randomUUID();
  const nonce = Buffer.from(crypto.randomUUID()).toString('base64');

  const cspHeader = [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}'`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "connect-src 'self'",
    "font-src 'self' data:",
    "frame-src 'none'",
    "frame-ancestors 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "upgrade-insecure-requests",
  ].join('; ');

  const requestHeaders = new Headers(req.headers);
  requestHeaders.set(REQUEST_ID_HEADER, requestID);
  requestHeaders.set(NONCE_HEADER, nonce);

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
