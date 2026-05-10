import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

export interface StatisticsData {
  total_users: number;
  active_sessions: number;
  mail_operations: number;
  audit_logs_24h: number;
  timestamp: string;
}

export interface MailVolumeMetrics {
  hour: string;
  sent: number;
  received: number;
  failed: number;
}

export interface TopDomainsMetrics {
  domain: string;
  mail_count: number;
  error_rate: number;
}

export function useStatistics(companyId: string) {
  return useQuery({
    queryKey: ["statistics", companyId],
    queryFn: () => api.get<StatisticsData>(`/statistics/${companyId}`),
    enabled: !!companyId,
    refetchInterval: 30000, // Refresh every 30 seconds
    staleTime: 5000, // Consider stale after 5 seconds
  });
}

export function useMailVolumeMetrics(companyId: string, hours: number = 24) {
  return useQuery({
    queryKey: ["mail-volume-metrics", companyId, hours],
    queryFn: () =>
      api.get<MailVolumeMetrics[]>(`/statistics/${companyId}/mail-volume`, {
        params: { hours },
      }),
    enabled: !!companyId,
    refetchInterval: 60000, // Refresh every minute
    staleTime: 10000,
  });
}

export function useTopDomainsMetrics(companyId: string, limit: number = 10) {
  return useQuery({
    queryKey: ["top-domains-metrics", companyId, limit],
    queryFn: () =>
      api.get<TopDomainsMetrics[]>(`/statistics/${companyId}/top-domains`, {
        params: { limit },
      }),
    enabled: !!companyId,
    refetchInterval: 120000, // Refresh every 2 minutes
    staleTime: 30000,
  });
}
