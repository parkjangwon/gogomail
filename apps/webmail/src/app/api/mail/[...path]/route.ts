import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';
import { assertSameOriginForMutation, encodeBackendPath, headersForBackend } from '@/lib/security/proxy';
import { LEGACY_WEBMAIL_TOKEN_COOKIE, WEBMAIL_TOKEN_COOKIE } from '@/lib/security/cookies';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';

const DEV_USER_ID = process.env.NODE_ENV !== 'production' ? (process.env.GOGOMAIL_DEV_USER_ID || '') : '';
const MAIL_BASE_PREFIXES = new Set(['addressbooks', 'contacts', 'directory']);

function isDrivePublicShareLinkRoute(method: string, segments: string[]): boolean {
  if (segments.length < 2 || segments[0] !== 'drive' || segments[1] !== 'share-links') return false;
  if ((method === 'GET' || method === 'HEAD') && segments.length === 3) return true;
  if ((method === 'GET' || method === 'HEAD' || method === 'POST') && segments.length === 4 && segments[3] === 'download') return true;
  return false;
}

function isDrivePublicShareDownload(method: string, segments: string[]): boolean {
  return segments.length === 4 && segments[0] === 'drive' && segments[1] === 'share-links' && segments[3] === 'download' && (method === 'GET' || method === 'POST');
}

function backendBaseFor(pathStr: string): '/api/mail' | '/api/v1' {
  const [prefix] = pathStr.split('/');
  return MAIL_BASE_PREFIXES.has(prefix) ? '/api/mail' : '/api/v1';
}

function htmlEscape(value: string): string {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;').replaceAll("'", '&#39;');
}

function passwordForm(req: NextRequest, message = '이 공유 파일은 비밀번호가 필요합니다.'): NextResponse {
  const safeMessage = htmlEscape(message);
  const action = htmlEscape(new URL(req.url).pathname);
  return new NextResponse(`<!doctype html>
<html lang="ko"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>공유 파일 비밀번호</title>
<style>body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#f6f7fb;margin:0;min-height:100vh;display:grid;place-items:center;color:#111827}.card{background:#fff;border:1px solid #e5e7eb;border-radius:16px;padding:28px;width:min(420px,calc(100vw - 32px));box-shadow:0 20px 50px rgba(15,23,42,.12)}h1{font-size:20px;margin:0 0 8px}p{margin:0 0 18px;color:#6b7280;font-size:14px}input{width:100%;box-sizing:border-box;border:1px solid #d1d5db;border-radius:10px;padding:12px;font-size:15px;margin-bottom:14px}button{width:100%;border:0;border-radius:10px;padding:12px;background:#2563eb;color:white;font-weight:700;font-size:15px;cursor:pointer}.msg{color:#dc2626}</style>
</head><body><main class="card"><h1>공유 파일 다운로드</h1><p class="msg">${safeMessage}</p><form method="post" action="${action}"><input type="password" name="password" autocomplete="current-password" autofocus placeholder="비밀번호"><button type="submit">비밀번호 확인 후 다운로드</button></form></main></body></html>`, {
    status: 401,
    headers: { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store' },
  });
}

async function handler(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  let backendUrl: string;
  try {
    backendUrl = requiredBackendUrl();
  } catch {
    return backendConfigErrorResponse();
  }
  let pathStr: string;
  try {
    assertSameOriginForMutation(req.method, req.url, req.headers);
    pathStr = encodeBackendPath(path);
  } catch {
    return NextResponse.json({ error: 'Invalid proxy request' }, { status: 400 });
  }
  const isPublicShareLinkRoute = isDrivePublicShareLinkRoute(req.method, path);
  const reqUrl = new URL(req.url);
  if (isPublicShareLinkRoute) {
    reqUrl.searchParams.delete('user_id');
  }

  // In local development mode (no JWT configured), inject user_id query param.
  if (!isPublicShareLinkRoute && DEV_USER_ID && !reqUrl.searchParams.has('user_id')) {
    reqUrl.searchParams.set('user_id', DEV_USER_ID);
  }

  const search = reqUrl.search;
  const url = `${backendUrl}${backendBaseFor(pathStr)}/${pathStr}${search}`;

  // Read token from httpOnly cookie — never from client-supplied Authorization header
  const cookieStore = await cookies();
  const token = cookieStore.get(WEBMAIL_TOKEN_COOKIE)?.value
    ?? cookieStore.get(LEGACY_WEBMAIL_TOKEN_COOKIE)?.value;
  const headers = headersForBackend(req.headers, token, isPublicShareLinkRoute);

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
    const contentType = res.headers.get('content-type') || 'application/json';
    if (isDrivePublicShareDownload(req.method, path) && res.status === 401 && req.headers.get('accept')?.includes('text/html')) {
      let message = req.method === 'POST' ? '비밀번호가 올바르지 않습니다.' : '이 공유 파일은 비밀번호가 필요합니다.';
      try {
        const parsed = JSON.parse(new TextDecoder().decode(data)) as { error_message?: string; error?: { message?: string } };
        const backendMessage = parsed.error_message ?? parsed.error?.message ?? '';
        if (backendMessage.includes('invalid')) message = '비밀번호가 올바르지 않습니다.';
      } catch {}
      return passwordForm(req, message);
    }
    const responseHeaders: Record<string, string> = {
      'Content-Type': contentType,
      'Cache-Control': 'no-store',
      'X-Content-Type-Options': 'nosniff',
    };
    const cd = res.headers.get('content-disposition');
    if (cd) responseHeaders['Content-Disposition'] = cd.replace(/[\r\n]/g, ' ');
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
export const HEAD = handler;
