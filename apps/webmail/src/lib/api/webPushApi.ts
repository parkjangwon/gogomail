// Web Push subscription API — wired to the documented backend contract:
//   GET    /api/v1/config/web-push              -> { vapidPublicKey }
//   GET    /api/v1/me/push-subscriptions        -> { subscriptions: [...] }
//   POST   /api/v1/me/push-subscriptions        { endpoint, p256dh, auth, userAgent }
//   DELETE /api/v1/me/push-subscriptions/{id}
//
// All calls route through the same-origin /api/mail/* proxy (the request()
// helper), which maps the `me` and `config` prefixes to the backend /api/v1/*
// namespace and injects the httpOnly session token.
import { request } from './http';

export interface WebPushSubscription {
  id: string;
  endpoint: string;
  user_agent?: string;
  created_at?: string;
  last_used_at?: string;
}

export async function getWebPushConfig(): Promise<{ vapidPublicKey: string | null }> {
  return request<{ vapidPublicKey: string | null }>('config/web-push', { method: 'GET' });
}

export async function listWebPushSubscriptions(): Promise<WebPushSubscription[]> {
  const data = await request<{ subscriptions: WebPushSubscription[] }>('me/push-subscriptions', { method: 'GET' });
  return data.subscriptions ?? [];
}

export interface UpsertWebPushSubscriptionRequest {
  endpoint: string;
  p256dh: string;
  auth: string;
  userAgent?: string;
}

export async function upsertWebPushSubscription(
  req: UpsertWebPushSubscriptionRequest,
): Promise<WebPushSubscription> {
  const data = await request<{ subscription: WebPushSubscription }>('me/push-subscriptions', {
    method: 'POST',
    body: JSON.stringify(req),
  });
  return data.subscription;
}

export async function deleteWebPushSubscription(id: string): Promise<void> {
  await request<unknown>(`me/push-subscriptions/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
