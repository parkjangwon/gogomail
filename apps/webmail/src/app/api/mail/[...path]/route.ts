import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';

const BACKEND = process.env.NEXT_PUBLIC_GOGOMAIL_BACKEND_URL || 'http://localhost:8080';
const DEV_USER_ID = process.env.GOGOMAIL_DEV_USER_ID || '';

async function handler(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  const pathStr = path.join('/');
  const reqUrl = new URL(req.url);

  // In dev mode (no JWT configured), inject user_id query param
  if (DEV_USER_ID && !reqUrl.searchParams.has('user_id')) {
    reqUrl.searchParams.set('user_id', DEV_USER_ID);
  }

  const search = reqUrl.search;
  const url = `${BACKEND}/api/v1/${pathStr}${search}`;

  const headers = new Headers();
  // Read token from httpOnly cookie — never from client-supplied Authorization header
  const cookieStore = await cookies();
  const token = cookieStore.get('webmail_token')?.value;
  if (token) headers.set('Authorization', `Bearer ${token}`);
  const ct = req.headers.get('content-type');
  if (ct) headers.set('Content-Type', ct);

  let body: ArrayBuffer | undefined;
  if (req.method !== 'GET' && req.method !== 'HEAD') {
    body = await req.arrayBuffer();
  }

  try {
    const res = await fetch(url, {
      method: req.method,
      headers,
      body,
    });
    const data = await res.arrayBuffer();
    const responseHeaders: Record<string, string> = {
      'Content-Type': res.headers.get('content-type') || 'application/json',
    };
    const cd = res.headers.get('content-disposition');
    if (cd) responseHeaders['Content-Disposition'] = cd;
    return new NextResponse(data, { status: res.status, headers: responseHeaders });
  } catch (_e) {
    return NextResponse.json({ error: 'Backend unreachable' }, { status: 503 });
  }
}

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
