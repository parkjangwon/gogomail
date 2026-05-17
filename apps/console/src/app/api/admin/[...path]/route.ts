import { adminProxyHandler } from '@/lib/server/adminProxy';

const BACKEND_URL = process.env.GOGOMAIL_BACKEND_URL || 'http://localhost:8080';

async function handler(
  req: Request,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  return adminProxyHandler(req, path, BACKEND_URL);
}

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
