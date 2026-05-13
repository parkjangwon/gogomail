import { cookies } from 'next/headers';

const BACKEND_URL = process.env.NEXT_PUBLIC_GOGOMAIL_BACKEND_URL || 'http://localhost:8080';

type Methods = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
type BodyInit = Exclude<RequestInit['body'], null>;

async function handler(
  req: Request,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  const pathStr = path.join('/');
  const url = `${BACKEND_URL}/admin/v1/${pathStr}${req.url.includes('?') ? '?' + new URL(req.url).search.slice(1) : ''}`;

  const cookieStore = await cookies();
  const token = cookieStore.get('admin_access_token')?.value;

  const headers = new Headers(req.headers);
  headers.delete('host');
  headers.delete('connection');
  headers.delete('content-length');

  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  let body: BodyInit | undefined;
  if (req.method !== 'GET' && req.method !== 'HEAD') {
    body = await req.arrayBuffer();
  }

  try {
    const response = await fetch(url, {
      method: req.method as Methods,
      headers,
      body,
    });

    const contentType = response.headers.get('content-type') ?? '';
    const contentDisposition = response.headers.get('content-disposition') ?? '';
    const responseHeaders: Record<string, string> = {
      'content-type': contentType || 'application/json',
    };
    if (contentDisposition) {
      responseHeaders['content-disposition'] = contentDisposition;
    }

    if (response.status === 204) {
      return new Response(null, { status: 204, headers: responseHeaders });
    }

    if (contentType.includes('text/csv') || contentType.includes('application/octet-stream')) {
      const data = await response.arrayBuffer();
      return new Response(data, {
        status: response.status,
        headers: responseHeaders,
      });
    }

    const responseBody = contentType.includes('application/json')
      ? await response.json()
      : await response.text();

    return Response.json(responseBody, {
      status: response.status,
      headers: responseHeaders,
    });
  } catch (error) {
    console.error('API proxy error:', error);
    return Response.json(
      { error: 'Failed to proxy request to backend' },
      { status: 500 }
    );
  }
}

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
