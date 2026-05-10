'use client';

import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface DashboardStats {
  total_users: number;
  total_domains: number;
  active_sessions: number;
  pending_invitations: number;
}

export interface ActivityMetric {
  date: string;
  user_logins: number;
  api_calls: number;
  mail_sent: number;
  mail_received: number;
}

export interface ApiUsageMetric {
  endpoint: string;
  requests_count: number;
  avg_latency_ms: number;
  error_rate: number;
}

export interface SecurityEvent {
  id: string;
  event_type: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  description: string;
  timestamp: string;
  user_id?: string;
  ip_address?: string;
}

export function useDashboardStats(companyId: string | undefined) {
  return useQuery({
    queryKey: ['dashboardStats', companyId],
    queryFn: async () => {
      if (!companyId) return null;
      const res = await api.get(`/companies/${companyId}/dashboard/stats`) as any;
      return res.data as DashboardStats;
    },
    enabled: !!companyId,
  });
}

export function useActivityMetrics(companyId: string | undefined, days: number = 7) {
  return useQuery({
    queryKey: ['activityMetrics', companyId, days],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/dashboard/activity?days=${days}`) as any;
      return (res.data?.metrics || []) as ActivityMetric[];
    },
    enabled: !!companyId,
  });
}

export function useApiUsageMetrics(companyId: string | undefined) {
  return useQuery({
    queryKey: ['apiUsageMetrics', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/dashboard/api-usage`) as any;
      return (res.data?.metrics || []) as ApiUsageMetric[];
    },
    enabled: !!companyId,
  });
}

export function useSecurityEvents(companyId: string | undefined, limit: number = 10) {
  return useQuery({
    queryKey: ['securityEvents', companyId, limit],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/dashboard/security-events?limit=${limit}`) as any;
      return (res.data?.events || []) as SecurityEvent[];
    },
    enabled: !!companyId,
  });
}
