'use client';

import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { APIUsageDailyRow } from '@/lib/apiUsage';
import type { components, operations } from '@gogomail/api-types';

export type APIUsageDaily = components['schemas']['APIUsageDaily'];
export type APIUsageDailyListEnvelope = operations['listAdminAPIUsageDaily']['responses'][200]['content']['application/json'];
export type PushNotificationAttempt = components['schemas']['PushNotificationAttempt'];
export type PushNotificationAttemptsEnvelope = operations['listAdminPushNotificationAttempts']['responses'][200]['content']['application/json'];

export interface APIUsageDailyFilters {
  companyId: string;
  domainId?: string;
  userId?: string;
  principalId?: string;
  route?: string;
  status?: number;
  from?: string;
  to?: string;
  method?: string;
  authSource?: string;
  limit?: number;
}

export interface PushNotificationAttemptFilters {
  limit?: number;
}

function buildAPIUsageParams(filters: APIUsageDailyFilters) {
  const params: Record<string, string | number | boolean> = {
    company_id: filters.companyId,
    limit: filters.limit ?? 100,
  };
  if (filters.domainId) params.domain_id = filters.domainId;
  if (filters.userId) params.user_id = filters.userId;
  if (filters.principalId) params.principal_id = filters.principalId;
  if (filters.route) params.route = filters.route;
  if (filters.status !== undefined) params.status = filters.status;
  if (filters.from) params.from = filters.from;
  if (filters.to) params.to = filters.to;
  if (filters.method) params.method = filters.method;
  if (filters.authSource) params.auth_source = filters.authSource;
  return params;
}

export function useAdminAPIUsageDaily(filters: APIUsageDailyFilters, enabled = true) {
  return useQuery({
    queryKey: ['admin', 'analytics', 'api-usage', filters],
    queryFn: async () => {
      const res = await api.get<APIUsageDailyListEnvelope>('/api-usage/daily', {
        params: buildAPIUsageParams(filters),
      });
      return (res.api_usage_daily ?? []).map((row): APIUsageDailyRow => ({
        day: row.day ?? '',
        method: row.method ?? '',
        route: row.route ?? '',
        status: row.status ?? 0,
        tenant_id: row.tenant_id ?? '',
        company_id: row.company_id ?? '',
        domain_id: row.domain_id ?? '',
        user_id: row.user_id ?? '',
        api_key_id: row.api_key_id ?? '',
        principal_id: row.principal_id ?? '',
        auth_source: row.auth_source ?? '',
        request_count: row.request_count ?? 0,
        request_bytes: row.request_bytes ?? 0,
        response_bytes: row.response_bytes ?? 0,
        latency_ms_total: row.latency_ms_total ?? 0,
        latency_ms_max: row.latency_ms_max ?? 0,
        latency_ms_average: row.latency_ms_average ?? 0,
        first_seen_at: row.first_seen_at ?? '',
        last_seen_at: row.last_seen_at ?? '',
      }));
    },
    enabled: enabled && !!filters.companyId,
  });
}

export function useAdminPushNotificationAttempts(filters: PushNotificationAttemptFilters = {}) {
  return useQuery({
    queryKey: ['admin', 'analytics', 'push-attempts', filters],
    queryFn: async () => {
      const res = await api.get<PushNotificationAttemptsEnvelope>('/push-notification-attempts', {
        params: {
          limit: filters.limit ?? 100,
        },
      });
      return res.push_notification_attempts ?? [];
    },
  });
}
