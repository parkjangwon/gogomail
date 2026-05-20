import { NextRequest, NextResponse } from 'next/server';
import { assertSameOriginRequest } from '@/lib/server/adminProxy';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';
import { ADMIN_ACCESS_TOKEN_COOKIE, LEGACY_ADMIN_ACCESS_TOKEN_COOKIE } from '@/lib/server/cookies';

const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  try {
    assertSameOriginRequest(req);
  } catch {
    return NextResponse.json({ error: 'Invalid request origin' }, { status: 403 });
  }
  let backendUrl: string;
  try {
    backendUrl = requiredBackendUrl();
  } catch {
    return backendConfigErrorResponse();
  }

  let body: unknown;
  try { body = await req.json(); } catch {
    return NextResponse.json({ error: 'Invalid request body' }, { status: 400 });
  }

  const upstream = await fetch(`${backendUrl}/admin/v1/auth/mfa/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return NextResponse.json({ error: 'Backend unreachable' }, { status: 503 });

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: 'Verification failed' }));
    return NextResponse.json(err, { status: upstream.status });
  }

  const data = await upstream.json() as { access_token?: string; refresh_token?: string };

  if (!data.access_token) {
    return NextResponse.json({ error: 'No access token in response' }, { status: 502 });
  }

  const response = NextResponse.json(
    { ok: true },
    { headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } }
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
  return response;
}
