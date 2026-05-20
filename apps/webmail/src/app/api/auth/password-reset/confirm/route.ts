import { NextRequest, NextResponse } from 'next/server';
import { assertSameOriginForMutation } from '@/lib/security/proxy';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';

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

  const upstream = await fetch(`${backendUrl}/api/v1/auth/password-reset/confirm`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => null);

  if (!upstream) return NextResponse.json({ error: 'Backend unreachable' }, { status: 503 });

  if (!upstream.ok) {
    const err = await upstream.json().catch(() => ({ error: '유효하지 않거나 만료된 토큰입니다.' }));
    return NextResponse.json(err, { status: upstream.status });
  }

  return NextResponse.json(
    {},
    { status: 200, headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } },
  );
}
