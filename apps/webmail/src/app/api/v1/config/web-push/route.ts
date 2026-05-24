import { NextResponse } from 'next/server';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';

export async function GET() {
  let backendUrl: string;
  try {
    backendUrl = requiredBackendUrl();
  } catch {
    return backendConfigErrorResponse();
  }

  const upstream = await fetch(`${backendUrl}/api/v1/config/web-push`, {
    method: 'GET',
    headers: { 'Accept': 'application/json' },
    cache: 'no-store',
  }).catch(() => null);

  if (!upstream) {
    return NextResponse.json({ error: 'Backend unreachable' }, { status: 503 });
  }

  const body = await upstream.arrayBuffer();
  return new NextResponse(body, {
    status: upstream.status,
    headers: {
      'Content-Type': upstream.headers.get('content-type') || 'application/json',
      'Cache-Control': 'no-store',
      'X-Content-Type-Options': 'nosniff',
    },
  });
}
