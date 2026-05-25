import { NextRequest, NextResponse } from 'next/server';
import { LEGACY_WEBMAIL_TOKEN_COOKIE, WEBMAIL_TOKEN_COOKIE } from '@/lib/security/cookies';
import { assertSameOriginForMutation } from '@/lib/security/proxy';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';
import { logServerRequest, requestIDFromHeaders, responseHeadersWithRequestID } from '@/lib/server/requestLog';

const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  const started = Date.now();
  const requestID = requestIDFromHeaders(req.headers);
  const finish = (response: NextResponse, upstreamStatus?: number): NextResponse => {
    logServerRequest({
      requestID,
      method: req.method,
      route: '/api/auth/login',
      status: response.status,
      durationMs: Date.now() - started,
      upstreamStatus,
    });
    return response;
  };
  try {
    assertSameOriginForMutation(req.method, req.url, req.headers);
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

  const upstream = await fetch(`${backendUrl}/api/v1/auth/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestID },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return finish(NextResponse.json({ error: 'Backend unreachable' }, {
    status: 503,
    headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
  }));

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: '로그인에 실패했습니다.' }));
    return finish(NextResponse.json(err, {
      status: upstream.status,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }), upstream.status);
  }

  const data = await upstream.json() as {
    token?: string;
    expires_at?: string;
    must_change_password?: boolean;
    client_ip?: string;
    mfa_required?: boolean;
    pending_token?: string;
    mfa_setup_required?: boolean;
  };

  // MFA TOTP required: return pending token to client without setting cookie.
  if (data.mfa_required) {
    return finish(NextResponse.json(
      { mfa_required: true, pending_token: data.pending_token },
      { headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' }, requestID) },
    ), upstream.status);
  }

  const maxAge = data.expires_at
    ? Math.max(60, Math.floor((new Date(data.expires_at).getTime() - Date.now()) / 1000))
    : 86400;

  const response = NextResponse.json({
    expires_at: data.expires_at,
    must_change_password: data.must_change_password,
    client_ip: data.client_ip,
    mfa_setup_required: data.mfa_setup_required ?? false,
  }, { headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' }, requestID) });

  response.cookies.set(WEBMAIL_TOKEN_COOKIE, data.token!, {
    httpOnly: true,
    secure: IS_PROD,
    sameSite: 'strict',
    path: '/',
    maxAge,
  });
  if (WEBMAIL_TOKEN_COOKIE !== LEGACY_WEBMAIL_TOKEN_COOKIE) {
    response.cookies.set(LEGACY_WEBMAIL_TOKEN_COOKIE, '', {
      httpOnly: true,
      secure: IS_PROD,
      sameSite: 'strict',
      path: '/',
      maxAge: 0,
    });
  }

  return finish(response, upstream.status);
}
