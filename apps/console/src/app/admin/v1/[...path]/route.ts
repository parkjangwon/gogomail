import { adminProxyHandler } from '@/lib/server/adminProxy';
import { backendConfigErrorResponse, requiredBackendUrl } from '@/lib/server/backend';

async function handler(
  req: Request,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  let backendUrl: string;
  try {
    backendUrl = requiredBackendUrl();
  } catch {
    return backendConfigErrorResponse();
  }
  return adminProxyHandler(req, path, backendUrl);
}

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
