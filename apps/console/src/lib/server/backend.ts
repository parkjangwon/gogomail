import { NextResponse } from 'next/server';

export function requiredBackendUrl(): string {
  const raw = (process.env.GOGOMAIL_BACKEND_URL || process.env.ADMIN_BACKEND_URL || '').trim();
  if (!raw) {
    throw new Error('GOGOMAIL_BACKEND_URL is required for server-side API proxy routes');
  }
  const url = new URL(raw);
  return url.toString().replace(/\/$/, '');
}

export function backendConfigErrorResponse(): NextResponse {
  return NextResponse.json(
    { error: 'Backend URL is not configured' },
    { status: 500, headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' } },
  );
}
