import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';

const PRIVATE_IP_RE =
  /^(localhost$|127\.|10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.|::1$|0\.0\.0\.0)/i;
const ALLOWED_CONTENT_TYPE_RE =
  /^image\/(jpeg|png|gif|webp|svg\+xml|bmp|tiff|x-icon|avif)/;

export async function GET(req: NextRequest) {
  const cookieStore = await cookies();
  const token = cookieStore.get('webmail_token')?.value;
  const devUserId = process.env.GOGOMAIL_DEV_USER_ID || '';
  if (!token && !devUserId) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  const rawUrl = req.nextUrl.searchParams.get('url');
  if (!rawUrl) return NextResponse.json({ error: 'Missing url' }, { status: 400 });

  let parsed: URL;
  try { parsed = new URL(rawUrl); } catch {
    return NextResponse.json({ error: 'Invalid URL' }, { status: 400 });
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return NextResponse.json({ error: 'Only http/https allowed' }, { status: 400 });
  }

  if (PRIVATE_IP_RE.test(parsed.hostname)) {
    return NextResponse.json({ error: 'Private addresses not allowed' }, { status: 403 });
  }

  try {
    const upstream = await fetch(rawUrl, {
      headers: { 'User-Agent': 'GoGoMail-ImageProxy/1.0' },
      redirect: 'follow',
      signal: AbortSignal.timeout(10_000),
    });

    const ct = upstream.headers.get('content-type') ?? '';
    if (!ALLOWED_CONTENT_TYPE_RE.test(ct)) {
      return NextResponse.json({ error: 'Not an allowed image type' }, { status: 415 });
    }

    const data = await upstream.arrayBuffer();
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
