'use client';

import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components, operations } from '@gogomail/api-types';

export type QuotaUsage = components['schemas']['QuotaUsage'];
export type QuotaUsageListEnvelope =
  operations['listAdminQuotaUsage']['responses'][200]['content']['application/json'];

const quotaUsageKey = (companyId: string) => ['companies', companyId, 'quota-usage'] as const;

export function useQuotaUsage(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? quotaUsageKey(companyId) : ['companies', 'quota-usage'],
    queryFn: async () => {
      const res = await api.get<QuotaUsageListEnvelope>('/quota-usage', {
        params: { limit: 100 },
      });
      return res.quota_usage ?? [];
    },
    enabled: !!companyId,
    staleTime: 30_000,
  });
}
