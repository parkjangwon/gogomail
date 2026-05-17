import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';
import { assertImageContentType, validateOutboundHttpUrl } from '@/lib/security/outboundUrl';
import { LEGACY_WEBMAIL_TOKEN_COOKIE, WEBMAIL_TOKEN_COOKIE } from '@/lib/security/cookies';

const MAX_IMAGE_BYTES = 5 * 1024 * 1024;
const MAX_REDIRECTS = 3;

async function fetchImage(url: URL, redirects = 0): Promise<Response> {
  const upstream = await fetch(url, {
    headers: { 'User-Agent': 'GoGoMail-ImageProxy/1.0' },
    redirect: 'manual',
    signal: AbortSignal.timeout(10_000),
  });
  if (upstream.status >= 300 && upstream.status < 400 && upstream.headers.has('location')) {
    if (redirects >= MAX_REDIRECTS) throw new Error('Too many redirects');
    const next = new URL(upstream.headers.get('location') ?? '', url);
    await validateOutboundHttpUrl(next.toString());
    return fetchImage(next, redirects + 1);
  }
  return upstream;
}

export async function GET(req: NextRequest) {
  const cookieStore = await cookies();
  const token = cookieStore.get(WEBMAIL_TOKEN_COOKIE)?.value
    ?? cookieStore.get(LEGACY_WEBMAIL_TOKEN_COOKIE)?.value;
  const devUserId = process.env.GOGOMAIL_DEV_USER_ID || '';
  if (!token && !devUserId) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  const rawUrl = req.nextUrl.searchParams.get('url');
  if (!rawUrl) return NextResponse.json({ error: 'Missing url' }, { status: 400 });

  let parsed: URL;
  try { parsed = await validateOutboundHttpUrl(rawUrl); } catch {
    return NextResponse.json({ error: 'Invalid URL' }, { status: 400 });
  }

  try {
    const upstream = await fetchImage(parsed);

    const ct = upstream.headers.get('content-type') ?? '';
    try {
      assertImageContentType(ct);
    } catch {
      return NextResponse.json({ error: 'Not an allowed image type' }, { status: 415 });
    }

    const length = Number(upstream.headers.get('content-length') ?? '0');
    if (length > MAX_IMAGE_BYTES) {
      return NextResponse.json({ error: 'Image is too large' }, { status: 413 });
    }
    const data = await upstream.arrayBuffer();
    if (data.byteLength > MAX_IMAGE_BYTES) {
      return NextResponse.json({ error: 'Image is too large' }, { status: 413 });
    }
    return new NextResponse(data, {
      status: 200,
      headers: {
        'Content-Type': ct,
        'Cache-Control': 'public, max-age=86400, immutable',
        'Content-Security-Policy': "default-src 'none'",
        'X-Content-Type-Options': 'nosniff',
      },
    });
  } catch {
    return NextResponse.json({ error: 'Failed to fetch image' }, { status: 502 });
  }
}
