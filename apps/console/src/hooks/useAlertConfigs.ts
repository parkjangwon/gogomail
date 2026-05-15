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
      const response = await api.get(`/admin/v1/alerts/configs`);
      return response.json();
    },
  });
}

export function useCreateAlertConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (config: Omit<AlertConfig, 'id' | 'created_at' | 'updated_at' | 'company_id'>) => {
      const response = await api.post(`/admin/v1/alerts/configs`, config);
      if (!response.ok) throw new Error('Failed to create alert config');
      return response.json();
    },
    onSuccess: (_, variables) => {
      // Invalidate configs query
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
      const response = await api.put(`/admin/v1/alerts/configs/${id}`, updates);
      if (!response.ok) throw new Error('Failed to update alert config');
      return response.json();
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
      const response = await api.delete(`/admin/v1/alerts/configs/${id}`);
      if (!response.ok) throw new Error('Failed to delete alert config');
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
      const response = await api.get(`/admin/v1/alerts/notifications`);
      return response.json();
    },
  });
}

export function useAcknowledgeNotification() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (notificationId: string) => {
      const response = await api.post(`/admin/v1/alerts/notifications/${notificationId}/acknowledge`);
      if (!response.ok) throw new Error('Failed to acknowledge notification');
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['alert-notifications'],
      });
    },
  });
}
