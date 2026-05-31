import { NextResponse } from 'next/server';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';
import { logServerRequest, requestIDFromHeaders, responseHeadersWithRequestID } from '@/lib/server/requestLog';
import { fetchUpstreamOrNull } from '@/lib/server/upstream';

export async function GET(req: Request) {
  const started = Date.now();
  const requestID = requestIDFromHeaders(req.headers);
  const finish = (response: NextResponse, upstreamStatus?: number): NextResponse => {
    logServerRequest({
      requestID,
      method: req.method,
      route: '/api/v1/config/web-push',
      status: response.status,
      durationMs: Date.now() - started,
      upstreamStatus,
    });
    return response;
  };
  let backendUrl: string;
  try {
    backendUrl = requiredBackendUrl();
  } catch {
    const response = backendConfigErrorResponse();
    response.headers.set('X-Request-ID', requestID);
    return finish(response);
  }

  const upstream = await fetchUpstreamOrNull(`${backendUrl}/api/v1/config/web-push`, {
    method: 'GET',
    headers: { 'Accept': 'application/json', 'X-Request-ID': requestID },
    cache: 'no-store',
  });

  if (!upstream) {
    return finish(NextResponse.json({ error: 'Backend unreachable' }, {
      status: 503,
      headers: responseHeadersWithRequestID({ 'Cache-Control': 'no-store' }, requestID),
    }));
  }

  const body = await upstream.arrayBuffer();
  return finish(new NextResponse(body, {
    status: upstream.status,
    headers: responseHeadersWithRequestID({
      'Content-Type': upstream.headers.get('content-type') || 'application/json',
      'Cache-Control': 'no-store',
      'X-Content-Type-Options': 'nosniff',
    }, requestID),
  }), upstream.status);
}
