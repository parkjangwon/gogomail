import { NextRequest, NextResponse } from 'next/server';
import { LEGACY_WEBMAIL_TOKEN_COOKIE, WEBMAIL_TOKEN_COOKIE } from '@/lib/security/cookies';
import { assertSameOriginForMutation } from '@/lib/security/proxy';

const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  try {
    assertSameOriginForMutation(req.method, req.url, req.headers);
  } catch {
    return NextResponse.json({ error: 'Invalid request origin' }, { status: 403 });
  }

  const response = NextResponse.json({ ok: true }, { headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } });
  response.cookies.set(WEBMAIL_TOKEN_COOKIE, '', {
    httpOnly: true,
    secure: IS_PROD,
    sameSite: 'strict',
    path: '/',
    maxAge: 0,
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
