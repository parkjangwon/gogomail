import { NextRequest, NextResponse } from 'next/server';
import { LEGACY_WEBMAIL_TOKEN_COOKIE, WEBMAIL_TOKEN_COOKIE } from '@/lib/security/cookies';
import { assertSameOriginForMutation } from '@/lib/security/proxy';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';
import { logServerRequest, requestIDFromHeaders, responseHeadersWithRequestID } from '@/lib/server/requestLog';
import { fetchUpstreamOrNull, readJSONOrDefault } from '@/lib/server/upstream';

const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  const started = Date.now();
  const requestID = requestIDFromHeaders(req.headers);
  const finish = (response: NextResponse, upstreamStatus?: number): NextResponse => {
    logServerRequest({
      requestID,
      method: req.method,
      route: '/api/auth/mfa',
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

  const upstream = await fetchUpstreamOrNull(`${backendUrl}/api/v1/auth/mfa/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestID },
    body: JSON.stringify(body),
  });

  if (!upstream) return finish(NextResponse.json({ error: 'Backend unreachable' }, {
    status: 503,
    headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
  }));

  if (!upstream.ok) {
    const err = await readJSONOrDefault(upstream, { error: 'MFA 인증에 실패했습니다.' });
    return finish(NextResponse.json(err, {
      status: upstream.status,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }), upstream.status);
  }

  const data = await upstream.json() as { token: string; expires_at: string };

  const maxAge = Math.max(
    60,
    Math.floor((new Date(data.expires_at).getTime() - Date.now()) / 1000),
  );

  const response = NextResponse.json(
    { expires_at: data.expires_at },
    { headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' }, requestID) },
  );

  response.cookies.set(WEBMAIL_TOKEN_COOKIE, data.token, {
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
