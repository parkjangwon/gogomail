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

interface AlertRulesEnvelope {
  rules: AlertRule[];
}

interface AlertChannelsEnvelope {
  channels: AlertChannel[];
}

interface AlertEventsEnvelope {
  events: AlertEvent[];
}

interface IDStatusEnvelope {
  status: 'ok';
  id: string;
}

export function useAlertRules(companyId: string | undefined) {
  return useQuery({
    queryKey: ['alertRules', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<AlertRulesEnvelope>(`/companies/${companyId}/alert-rules`);
      return res.rules;
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
      return api.post<AlertRule>(
        `/companies/${companyId}/alert-rules`,
        data
      );
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
      return api.get<AlertRule>(`/alert-rules/${ruleId}`);
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
      return api.put<IDStatusEnvelope>(`/alert-rules/${ruleId}`, data);
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
      return api.delete<IDStatusEnvelope>(`/alert-rules/${ruleId}`);
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
      const res = await api.get<AlertChannelsEnvelope>(`/companies/${companyId}/alert-channels`);
      return res.channels;
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
      return api.post<AlertChannel>(
        `/companies/${companyId}/alert-channels`,
        data
      );
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
      const res = await api.get<AlertEventsEnvelope>(`/companies/${companyId}/alert-events`);
      return res.events;
    },
    enabled: !!companyId,
  });
}
