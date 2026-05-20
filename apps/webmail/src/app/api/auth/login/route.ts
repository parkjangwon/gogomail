import { NextRequest, NextResponse } from 'next/server';
import { LEGACY_WEBMAIL_TOKEN_COOKIE, WEBMAIL_TOKEN_COOKIE } from '@/lib/security/cookies';
import { assertSameOriginForMutation } from '@/lib/security/proxy';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';

const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  try {
    assertSameOriginForMutation(req.method, req.url, req.headers);
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

  const upstream = await fetch(`${backendUrl}/api/v1/auth/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return NextResponse.json({ error: 'Backend unreachable' }, { status: 503 });

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: '로그인에 실패했습니다.' }));
    return NextResponse.json(err, { status: upstream.status });
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
    return NextResponse.json(
      { mfa_required: true, pending_token: data.pending_token },
      { headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } },
    );
  }

  const maxAge = Math.max(
    60,
    Math.floor((new Date(data.expires_at!).getTime() - Date.now()) / 1000),
  );

  const response = NextResponse.json({
    expires_at: data.expires_at,
    must_change_password: data.must_change_password,
    client_ip: data.client_ip,
    mfa_setup_required: data.mfa_setup_required ?? false,
  }, { headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } });

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

  return response;
}
