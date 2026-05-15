'use client';

import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { operations } from '@gogomail/api-types';

export type SeatUsageEnvelope =
  operations['getSeatUsage']['responses'][200]['content']['application/json'];

const seatUsageKey = (companyId: string) => ['companies', companyId, 'seat-usage'] as const;

export function useSeatUsage(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? seatUsageKey(companyId) : ['companies', 'seat-usage'],
    queryFn: async () => {
      if (!companyId) return null;
      return api.get<SeatUsageEnvelope>(`/companies/${companyId}/seat-usage`);
    },
    enabled: !!companyId,
    staleTime: 30_000,
  });
}
