import { NextRequest, NextResponse } from 'next/server';

const BACKEND = process.env.NEXT_PUBLIC_GOGOMAIL_BACKEND_URL || 'http://localhost:8080';
const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST(req: NextRequest) {
  let body: unknown;
  try { body = await req.json(); } catch {
    return NextResponse.json({ error: 'Invalid request body' }, { status: 400 });
  }

  const upstream = await fetch(`${BACKEND}/api/v1/auth/token`, {
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
    token: string;
    expires_at: string;
    must_change_password: boolean;
    client_ip?: string;
  };

  const maxAge = Math.max(
    60,
    Math.floor((new Date(data.expires_at).getTime() - Date.now()) / 1000),
  );

  const response = NextResponse.json({
    expires_at: data.expires_at,
    must_change_password: data.must_change_password,
    client_ip: data.client_ip,
  }, { headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } });

  response.cookies.set('webmail_token', data.token, {
    httpOnly: true,
    secure: IS_PROD,
    sameSite: 'strict',
    path: '/',
    maxAge,
  });

  return response;
}
