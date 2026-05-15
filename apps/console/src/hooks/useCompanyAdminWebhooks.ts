'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components, operations } from '@gogomail/api-types';

export type CompanyWebhook = components['schemas']['Webhook'];
export type CompanyWebhookInput = components['schemas']['WebhookInput'];
export type CompanyNotifTemplate = components['schemas']['NotifTemplate'];

export type CompanyWebhookListEnvelope = operations['adminListCompanyWebhooks']['responses'][200]['content']['application/json'];
export type CompanyWebhookCreateEnvelope = operations['adminCreateCompanyWebhook']['responses'][201]['content']['application/json'];
export type CompanyWebhookTestEnvelope = operations['adminTestCompanyWebhook']['responses'][200]['content']['application/json'];
export type CompanyNotifTemplateListEnvelope = operations['adminListCompanyNotifTemplates']['responses'][200]['content']['application/json'];
export type CompanyNotifTemplateUpdateEnvelope = operations['adminUpdateCompanyNotifTemplate']['responses'][200]['content']['application/json'];

const webhooksKey = (companyId: string) => ['companies', companyId, 'webhooks'] as const;
const notifTemplatesKey = (companyId: string) => ['companies', companyId, 'notification-templates'] as const;
const healthKey = (companyId: string) => ['companies', companyId, 'health'] as const;

export function useCompanyWebhooks(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? webhooksKey(companyId) : ['companies', 'webhooks'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<CompanyWebhookListEnvelope>(`/companies/${companyId}/webhooks`);
      return res.webhooks ?? [];
    },
    enabled: !!companyId,
  });
}

export function useCreateCompanyWebhook() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, data }: { companyId: string; data: CompanyWebhookInput }) =>
      api.post<CompanyWebhookCreateEnvelope>(`/companies/${companyId}/webhooks`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: webhooksKey(companyId) });
      queryClient.invalidateQueries({ queryKey: healthKey(companyId) });
    },
  });
}

export function useDeleteCompanyWebhook() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, webhookId }: { companyId: string; webhookId: string }) =>
      api.delete<{ deleted?: boolean }>(`/companies/${companyId}/webhooks/${webhookId}`),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: webhooksKey(companyId) });
      queryClient.invalidateQueries({ queryKey: healthKey(companyId) });
    },
  });
}

export function useTestCompanyWebhook() {
  return useMutation({
    mutationFn: async ({ companyId, webhookId }: { companyId: string; webhookId: string }) =>
      api.post<CompanyWebhookTestEnvelope>(`/companies/${companyId}/webhooks/${webhookId}/test`),
  });
}

export function useCompanyNotificationTemplates(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? notifTemplatesKey(companyId) : ['companies', 'notification-templates'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<CompanyNotifTemplateListEnvelope>(`/companies/${companyId}/notification-templates`);
      return res.templates ?? [];
    },
    enabled: !!companyId,
  });
}

export function useUpdateCompanyNotificationTemplate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, templateId, data }: { companyId: string; templateId: string; data: CompanyNotifTemplate }) =>
      api.put<CompanyNotifTemplateUpdateEnvelope>(`/companies/${companyId}/notification-templates/${templateId}`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: notifTemplatesKey(companyId) });
    },
  });
}
