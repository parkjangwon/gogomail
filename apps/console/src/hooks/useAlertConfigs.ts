'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface AlertChannel {
  id?: string;
  channel_type: 'email' | 'webhook' | 'dashboard';
  config: Record<string, unknown>;
  is_enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface AlertConfig {
  id: string;
  company_id: string;
  alert_type: 'storage' | 'login_failures' | 'api_errors';
  threshold: number;
  name: string;
  description?: string;
  check_interval_minutes: number;
  is_enabled: boolean;
  channels: AlertChannel[];
  created_at: string;
  updated_at: string;
}

export interface AlertNotification {
  id: string;
  alert_config_id: string;
  alert_type: 'storage' | 'login_failures' | 'api_errors';
  threshold: number;
  current_value: number;
  email_sent: boolean;
  webhook_sent: boolean;
  dashboard_shown: boolean;
  notification_data?: Record<string, unknown>;
  created_at: string;
  acknowledged_at?: string;
}

export function useAlertConfigs(companyId: string) {
  return useQuery({
    queryKey: ['alert-configs', companyId],
    queryFn: async () => {
      return api.get<AlertConfig[]>(`/alerts/configs`);
    },
  });
}

export function useCreateAlertConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (config: Omit<AlertConfig, 'id' | 'created_at' | 'updated_at' | 'company_id'>) => {
      return api.post<AlertConfig>(`/alerts/configs`, config);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['alert-configs'],
      });
    },
  });
}

export function useUpdateAlertConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, ...updates }: Partial<AlertConfig> & { id: string }) => {
      return api.put<AlertConfig>(`/alerts/configs/${id}`, updates);
    },
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({
        queryKey: ['alert-configs'],
      });
      queryClient.invalidateQueries({
        queryKey: ['alert-config', id],
      });
    },
  });
}

export function useDeleteAlertConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      return api.delete<void>(`/alerts/configs/${id}`);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['alert-configs'],
      });
    },
  });
}

export function useAlertNotifications(companyId: string) {
  return useQuery({
    queryKey: ['alert-notifications', companyId],
    queryFn: async () => {
      return api.get<AlertNotification[]>(`/alerts/notifications`);
    },
  });
}

export function useAcknowledgeNotification() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (notificationId: string) => {
      return api.post<void>(`/alerts/notifications/${notificationId}/acknowledge`);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['alert-notifications'],
      });
    },
  });
}
