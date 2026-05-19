import { NextRequest, NextResponse } from 'next/server';
import { assertSameOriginRequest } from '@/lib/server/adminProxy';
import { ADMIN_ACCESS_TOKEN_COOKIE, LEGACY_ADMIN_ACCESS_TOKEN_COOKIE } from '@/lib/server/cookies';

const BACKEND_URL = process.env.GOGOMAIL_BACKEND_URL || 'http://localhost:8080';
const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  try {
    assertSameOriginRequest(req);
  } catch {
    return NextResponse.json({ error: 'Invalid request origin' }, { status: 403 });
  }

  let body: unknown;
  try { body = await req.json(); } catch {
    return NextResponse.json({ error: 'Invalid request body' }, { status: 400 });
  }

  const upstream = await fetch(`${BACKEND_URL}/admin/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return NextResponse.json({ error: 'Backend unreachable' }, { status: 503 });

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: 'Invalid credentials' }));
    return NextResponse.json(err, { status: upstream.status });
  }

  const data = await upstream.json() as {
    access_token?: string;
    refresh_token?: string;
    expires_at?: string;
    mfa_required?: boolean;
    pending_token?: string;
    mfa_setup_required?: boolean;
    user?: { id: string; role: string; company_id: string };
  };

  // MFA challenge — don't set cookies yet, pass pending_token to frontend.
  if (data.mfa_required) {
    return NextResponse.json(
      { mfa_required: true, pending_token: data.pending_token },
      { headers: { 'Cache-Control': 'no-store' } }
    );
  }

  // Full token — set cookies.
  const maxAge = data.expires_at
    ? Math.max(60, Math.floor((new Date(data.expires_at).getTime() - Date.now()) / 1000))
    : 86400;

  if (!data.access_token) {
    return NextResponse.json({ error: 'No access token in response' }, { status: 502 });
  }

  const responseBody: Record<string, unknown> = { ok: true };
  if (data.mfa_setup_required) responseBody.mfa_setup_required = true;

  const response = NextResponse.json(responseBody, {
    headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' },
  });
  response.cookies.set(ADMIN_ACCESS_TOKEN_COOKIE, data.access_token, {
    httpOnly: true,
    secure: IS_PROD,
    sameSite: 'strict',
    path: '/',
    maxAge,
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
