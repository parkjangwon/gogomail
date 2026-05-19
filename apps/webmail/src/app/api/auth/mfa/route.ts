import { NextRequest, NextResponse } from 'next/server';
import { LEGACY_WEBMAIL_TOKEN_COOKIE, WEBMAIL_TOKEN_COOKIE } from '@/lib/security/cookies';
import { assertSameOriginForMutation } from '@/lib/security/proxy';

const BACKEND = process.env.GOGOMAIL_BACKEND_URL || 'http://localhost:8080';
const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  try {
    assertSameOriginForMutation(req.method, req.url, req.headers);
  } catch {
    return NextResponse.json({ error: 'Invalid request origin' }, { status: 403 });
  }

  let body: unknown;
  try { body = await req.json(); } catch {
    return NextResponse.json({ error: 'Invalid request body' }, { status: 400 });
  }

  const upstream = await fetch(`${BACKEND}/api/v1/auth/mfa/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return NextResponse.json({ error: 'Backend unreachable' }, { status: 503 });

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: 'MFA 인증에 실패했습니다.' }));
    return NextResponse.json(err, { status: upstream.status });
  }

  const data = await upstream.json() as { token: string; expires_at: string };

  const maxAge = Math.max(
    60,
    Math.floor((new Date(data.expires_at).getTime() - Date.now()) / 1000),
  );

  const response = NextResponse.json(
    { expires_at: data.expires_at },
    { headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } },
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

  return response;
}
