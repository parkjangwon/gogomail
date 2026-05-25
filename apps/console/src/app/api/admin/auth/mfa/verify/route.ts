import { NextRequest, NextResponse } from 'next/server';
import { assertSameOriginRequest } from '@/lib/server/adminProxy';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';
import { ADMIN_ACCESS_TOKEN_COOKIE, LEGACY_ADMIN_ACCESS_TOKEN_COOKIE } from '@/lib/server/cookies';
import { logServerRequest, requestIDFromHeaders, responseHeadersWithRequestID } from '@/lib/server/requestLog';

const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  const started = Date.now();
  const requestID = requestIDFromHeaders(req.headers);
  const finish = (response: NextResponse, upstreamStatus?: number): NextResponse => {
    logServerRequest({
      requestID,
      method: req.method,
      route: '/api/admin/auth/mfa/verify',
      status: response.status,
      durationMs: Date.now() - started,
      upstreamStatus,
    });
    return response;
  };
  try {
    assertSameOriginRequest(req);
  } catch {
    return finish(NextResponse.json({ error: 'Invalid request origin' }, {
      status: 403,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }));
  }
  let backendUrl: string;
  try {
    backendUrl = requiredBackendUrl();
  } catch {
    const response = backendConfigErrorResponse();
    response.headers.set('X-Request-ID', requestID);
    return finish(response);
  }

  let body: unknown;
  try { body = await req.json(); } catch {
    return finish(NextResponse.json({ error: 'Invalid request body' }, {
      status: 400,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }));
  }

  const upstream = await fetch(`${backendUrl}/admin/v1/auth/mfa/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestID },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return finish(NextResponse.json({ error: 'Backend unreachable' }, {
    status: 503,
    headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
  }));

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: 'Verification failed' }));
    return finish(NextResponse.json(err, {
      status: upstream.status,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }), upstream.status);
  }

  const data = await upstream.json() as { access_token?: string; refresh_token?: string };

  if (!data.access_token) {
    return finish(NextResponse.json({ error: 'No access token in response' }, {
      status: 502,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }), upstream.status);
  }

  const response = NextResponse.json(
    { ok: true },
    { headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' }, requestID) }
  );
  response.cookies.set(ADMIN_ACCESS_TOKEN_COOKIE, data.access_token, {
    httpOnly: true,
    secure: IS_PROD,
    sameSite: 'strict',
    path: '/',
    maxAge: 900,
  });
  if (ADMIN_ACCESS_TOKEN_COOKIE !== LEGACY_ADMIN_ACCESS_TOKEN_COOKIE) {
    response.cookies.set(LEGACY_ADMIN_ACCESS_TOKEN_COOKIE, '', {
      httpOnly: true,
      secure: IS_PROD,
      sameSite: 'strict',
      path: '/',
      maxAge: 0,
    });
  }
  return finish(response, upstream.status);
}
