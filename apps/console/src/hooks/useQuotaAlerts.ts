'use client';

import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components, operations } from '@gogomail/api-types';

export type QuotaAlert = components['schemas']['QuotaAlert'];
export type QuotaAlertListEnvelope =
  operations['listAdminQuotaAlerts']['responses'][200]['content']['application/json'];

const quotaAlertsKey = (companyId: string) => ['companies', companyId, 'quota-alerts'] as const;

export function useQuotaAlerts(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? quotaAlertsKey(companyId) : ['companies', 'quota-alerts'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<QuotaAlertListEnvelope>('/quota-alerts', {
        params: {
          company_id: companyId,
          limit: 100,
        },
      });
      return res.quota_alerts ?? [];
    },
    enabled: !!companyId,
    staleTime: 30_000,
  });
}
