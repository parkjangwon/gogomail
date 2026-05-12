import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';

const BACKEND = process.env.NEXT_PUBLIC_GOGOMAIL_BACKEND_URL || 'http://localhost:8080';
const DEV_USER_ID = process.env.GOGOMAIL_DEV_USER_ID || '';
const MAIL_BASE_PREFIXES = new Set(['addressbooks', 'contacts', 'directory']);

function isDrivePublicShareLinkRoute(method: string, segments: string[]): boolean {
  if (method !== 'GET' && method !== 'HEAD') return false;
  if (segments.length < 2 || segments[0] !== 'drive' || segments[1] !== 'share-links') return false;

  if (segments.length === 3) return true;
  return segments.length === 4 && segments[3] === 'download';
}

function backendBaseFor(pathStr: string): '/api/mail' | '/api/v1' {
  const [prefix] = pathStr.split('/');
  return MAIL_BASE_PREFIXES.has(prefix) ? '/api/mail' : '/api/v1';
}

async function handler(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  const pathStr = path.join('/');
  const isPublicShareLinkRoute = isDrivePublicShareLinkRoute(req.method, path);
  const reqUrl = new URL(req.url);
  if (isPublicShareLinkRoute) {
    reqUrl.searchParams.delete('user_id');
  }

  // In dev mode (no JWT configured), inject user_id query param
  if (!isPublicShareLinkRoute && DEV_USER_ID && !reqUrl.searchParams.has('user_id')) {
    reqUrl.searchParams.set('user_id', DEV_USER_ID);
  }

  const search = reqUrl.search;
  const url = `${BACKEND}${backendBaseFor(pathStr)}/${pathStr}${search}`;

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
