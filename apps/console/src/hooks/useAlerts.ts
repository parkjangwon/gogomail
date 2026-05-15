'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components, operations } from '@gogomail/api-types';

export type AlertRule = components['schemas']['AlertRule'];
export type AlertChannel = components['schemas']['AlertChannel'];
export type AlertEvent = components['schemas']['AlertEvent'];

export type AlertRuleListEnvelope =
  operations['listAlertRules']['responses'][200]['content']['application/json'];
export type AlertRuleCreateRequest =
  operations['createAlertRule']['requestBody']['content']['application/json'];
export type AlertRuleCreateResponse =
  operations['createAlertRule']['responses'][201]['content']['application/json'];
export type AlertRuleUpdateRequest =
  operations['updateAlertRule']['requestBody']['content']['application/json'];
export type AlertRuleUpdateResponse =
  operations['updateAlertRule']['responses'][200]['content']['application/json'];
export type AlertChannelListEnvelope =
  operations['listAlertChannels']['responses'][200]['content']['application/json'];
export type AlertChannelCreateRequest =
  operations['createAlertChannel']['requestBody']['content']['application/json'];
export type AlertChannelCreateResponse =
  operations['createAlertChannel']['responses'][201]['content']['application/json'];
export type AlertChannelUpdateRequest = components['schemas']['UpdateAlertChannel'];
export type AlertChannelUpdateResponse = components['schemas']['AlertChannel'];
export type AlertEventListEnvelope =
  operations['listAlertEvents']['responses'][200]['content']['application/json'];

const alertRulesKey = (companyId: string) => ['companies', companyId, 'alert-rules'] as const;
const alertChannelsKey = (companyId: string) => ['companies', companyId, 'alert-channels'] as const;
const alertEventsKey = (companyId: string) => ['companies', companyId, 'alert-events'] as const;

export function useAlertRules(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? alertRulesKey(companyId) : ['companies', 'alerts', 'rules'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<AlertRuleListEnvelope>(`/companies/${companyId}/alert-rules`);
      return res.rules ?? [];
    },
    enabled: !!companyId,
    staleTime: 30_000,
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
      data: AlertRuleCreateRequest;
    }) => api.post<AlertRuleCreateResponse>(`/companies/${companyId}/alert-rules`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: alertRulesKey(companyId) });
      queryClient.invalidateQueries({ queryKey: alertEventsKey(companyId) });
    },
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
      data: AlertRuleUpdateRequest;
    }) => api.put<AlertRuleUpdateResponse>(`/alert-rules/${ruleId}`, data),
    onSuccess: (_, { ruleId, companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['companies', companyId, 'alert-rules'] });
      queryClient.invalidateQueries({ queryKey: ['alert-rules', ruleId] });
      queryClient.invalidateQueries({ queryKey: alertEventsKey(companyId) });
    },
  });
}

export function useDeleteAlertRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ ruleId }: { ruleId: string; companyId: string }) =>
      api.delete<{ status: string; id?: string }>(`/alert-rules/${ruleId}`),
    onSuccess: (_, { ruleId, companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['companies', companyId, 'alert-rules'] });
      queryClient.invalidateQueries({ queryKey: ['alert-rules', ruleId] });
      queryClient.invalidateQueries({ queryKey: alertEventsKey(companyId) });
    },
  });
}

export function useAlertChannels(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? alertChannelsKey(companyId) : ['companies', 'alerts', 'channels'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<AlertChannelListEnvelope>(`/companies/${companyId}/alert-channels`);
      return res.channels ?? [];
    },
    enabled: !!companyId,
    staleTime: 30_000,
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
      data: AlertChannelCreateRequest;
    }) => api.post<AlertChannelCreateResponse>(`/companies/${companyId}/alert-channels`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: alertChannelsKey(companyId) });
    },
  });
}

export function useUpdateAlertChannel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      channelId,
      data,
    }: {
      channelId: string;
      companyId: string;
      data: AlertChannelUpdateRequest;
    }) => api.put<AlertChannelUpdateResponse>(`/alert-channels/${channelId}`, data),
    onSuccess: (_, { companyId, channelId }) => {
      queryClient.invalidateQueries({ queryKey: alertChannelsKey(companyId) });
      queryClient.invalidateQueries({ queryKey: ['companies', companyId, 'alert-channels', channelId] });
    },
  });
}

export function useDeleteAlertChannel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ channelId }: { channelId: string; companyId: string }) =>
      api.delete<{ status: string; id?: string }>(`/alert-channels/${channelId}`),
    onSuccess: (_, { companyId, channelId }) => {
      queryClient.invalidateQueries({ queryKey: alertChannelsKey(companyId) });
      queryClient.invalidateQueries({ queryKey: ['companies', companyId, 'alert-channels', channelId] });
    },
  });
}

export function useAlertEvents(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? alertEventsKey(companyId) : ['companies', 'alerts', 'events'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<AlertEventListEnvelope>(`/companies/${companyId}/alert-events`);
      return res.events ?? [];
    },
    enabled: !!companyId,
    staleTime: 15_000,
    refetchInterval: 60_000,
  });
}
