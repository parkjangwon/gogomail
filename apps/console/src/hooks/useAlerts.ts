'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface AlertRule {
  id: string;
  company_id: string;
  alert_type: 'storage' | 'login_failures' | 'api_errors';
  name: string;
  description?: string;
  threshold: number;
  check_interval_minutes: number;
  is_enabled: boolean;
  created_at: string;
  created_by?: string;
}

export interface AlertChannel {
  id: string;
  company_id: string;
  channel_type: 'email' | 'webhook' | 'dashboard';
  name: string;
  config: {
    recipients?: string[];
    url?: string;
    auth_header?: string;
  };
  is_enabled: boolean;
  created_at: string;
  created_by?: string;
}

export interface AlertEvent {
  id: string;
  company_id: string;
  alert_rule_id: string;
  current_value: number;
  threshold: number;
  message?: string;
  triggered_at: string;
  resolved_at?: string;
}

export function useAlertRules(companyId: string | undefined) {
  return useQuery({
    queryKey: ['alertRules', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/alert-rules`) as any;
      return (res.data?.rules || []) as AlertRule[];
    },
    enabled: !!companyId,
  });
}

export function useCreateAlertRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      data,
    }: {
      companyId: string;
      data: Omit<AlertRule, 'id' | 'created_at' | 'company_id'>;
    }) => {
      const res = await api.post(
        `/companies/${companyId}/alert-rules`,
        data
      ) as any;
      return res.data as AlertRule;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['alertRules', companyId] });
    },
  });
}

export function useGetAlertRule(ruleId: string | undefined) {
  return useQuery({
    queryKey: ['alertRule', ruleId],
    queryFn: async () => {
      if (!ruleId) return null;
      const res = await api.get(`/alert-rules/${ruleId}`) as any;
      return res.data as AlertRule;
    },
    enabled: !!ruleId,
  });
}

export function useUpdateAlertRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      ruleId,
      data,
    }: {
      ruleId: string;
      companyId: string;
      data: Partial<AlertRule>;
    }) => {
      const res = await api.put(`/alert-rules/${ruleId}`, data) as any;
      return res.data;
    },
    onSuccess: (_, { ruleId, companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['alertRule', ruleId] });
      queryClient.invalidateQueries({ queryKey: ['alertRules', companyId] });
    },
  });
}

export function useDeleteAlertRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ ruleId }: { ruleId: string; companyId: string }) => {
      const res = await api.delete(`/alert-rules/${ruleId}`) as any;
      return res.data;
    },
    onSuccess: (_, { ruleId, companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['alertRule', ruleId] });
      queryClient.invalidateQueries({ queryKey: ['alertRules', companyId] });
    },
  });
}

export function useAlertChannels(companyId: string | undefined) {
  return useQuery({
    queryKey: ['alertChannels', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/alert-channels`) as any;
      return (res.data?.channels || []) as AlertChannel[];
    },
    enabled: !!companyId,
  });
}

export function useCreateAlertChannel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      data,
    }: {
      companyId: string;
      data: Omit<AlertChannel, 'id' | 'created_at' | 'company_id'>;
    }) => {
      const res = await api.post(
        `/companies/${companyId}/alert-channels`,
        data
      ) as any;
      return res.data as AlertChannel;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({
        queryKey: ['alertChannels', companyId],
      });
    },
  });
}

export function useAlertEvents(companyId: string | undefined) {
  return useQuery({
    queryKey: ['alertEvents', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/alert-events`) as any;
      return (res.data?.events || []) as AlertEvent[];
    },
    enabled: !!companyId,
  });
}
