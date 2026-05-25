import { NextResponse, type NextRequest } from 'next/server';

const REQUEST_ID_HEADER = 'x-request-id';
const REQUEST_ID_RESPONSE_HEADER = 'X-Request-ID';
const MAX_REQUEST_ID_LENGTH = 128;

export function middleware(req: NextRequest) {
  const requestID = sanitizeRequestID(req.headers.get(REQUEST_ID_HEADER)) || crypto.randomUUID();
  const requestHeaders = new Headers(req.headers);
  requestHeaders.set(REQUEST_ID_HEADER, requestID);
  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set(REQUEST_ID_RESPONSE_HEADER, requestID);
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
