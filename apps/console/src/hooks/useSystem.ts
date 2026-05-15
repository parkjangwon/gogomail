'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components, operations } from '@gogomail/api-types';

export type AdminHealthEnvelope = operations['adminGetHealth']['responses'][200]['content']['application/json'];
export type AdminHealthCheck = NonNullable<AdminHealthEnvelope['checks']>[number];
export type QueueStatsEnvelope = operations['getAdminQueueStats']['responses'][200]['content']['application/json'];
export type QueueStat = components['schemas']['QueueStat'];
export type BackpressureEnvelope = operations['getAdminBackpressure']['responses'][200]['content']['application/json'];
export type BackpressureState = components['schemas']['BackpressureState'];
export type BackpressureUpdateRequest = components['requestBodies']['BackpressureUpdate']['content']['application/json'];

const healthKey = ['admin', 'system', 'health'] as const;
const queueKey = ['admin', 'system', 'queue'] as const;
const backpressureKey = ['admin', 'system', 'backpressure'] as const;

export function useAdminHealth(refetchInterval = 15_000) {
  return useQuery({
    queryKey: healthKey,
    queryFn: async () => api.get<AdminHealthEnvelope>('/health'),
    refetchInterval,
    staleTime: 5_000,
  });
}

export function useAdminQueueStats(refetchInterval = 5_000) {
  return useQuery({
    queryKey: queueKey,
    queryFn: async () => api.get<QueueStatsEnvelope>('/queue'),
    refetchInterval,
    staleTime: 2_500,
  });
}

export function useAdminBackpressure(refetchInterval = 5_000) {
  return useQuery({
    queryKey: backpressureKey,
    queryFn: async () => api.get<BackpressureEnvelope>('/backpressure'),
    refetchInterval,
    staleTime: 2_500,
  });
}

export function useUpdateAdminBackpressure() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: BackpressureUpdateRequest) => api.patch<BackpressureEnvelope>('/backpressure', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: backpressureKey });
    },
  });
}
