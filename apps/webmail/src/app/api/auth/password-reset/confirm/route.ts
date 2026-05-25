import { NextRequest, NextResponse } from 'next/server';
import { assertSameOriginForMutation } from '@/lib/security/proxy';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';
import { logServerRequest, requestIDFromHeaders, responseHeadersWithRequestID } from '@/lib/server/requestLog';

export async function POST(req: NextRequest) {
  const started = Date.now();
  const requestID = requestIDFromHeaders(req.headers);
  const finish = (response: NextResponse, upstreamStatus?: number): NextResponse => {
    logServerRequest({
      requestID,
      method: req.method,
      route: '/api/auth/password-reset/confirm',
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

  const upstream = await fetch(`${backendUrl}/api/v1/auth/password-reset/confirm`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestID },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return finish(NextResponse.json({ error: 'Backend unreachable' }, {
    status: 503,
    headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
  }));

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: '유효하지 않거나 만료된 토큰입니다.' }));
    return finish(NextResponse.json(err, {
      status: upstream.status,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }), upstream.status);
  }

  return finish(NextResponse.json(
    {},
    { status: 200, headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' }, requestID) },
  ), upstream.status);
}
