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

interface SeatUsageEnvelope {
  total_users: number;
  active_users: number;
}

interface MailFlowDailyStats {
  date: string;
  inbound_messages: number;
  outbound_messages: number;
  failed: number;
}

interface MailFlowDailyStatsEnvelope {
  mail_flow_daily_stats: MailFlowDailyStats[];
}

interface DomainListEnvelope {
  domains: Array<{ name: string }>;
}

export function useStatistics(companyId: string) {
  return useQuery({
    queryKey: ["statistics", companyId],
    queryFn: async () => {
      const seatUsage = await api.get<SeatUsageEnvelope>(`/companies/${companyId}/seat-usage`);
      return {
        total_users: seatUsage.total_users,
        active_sessions: seatUsage.active_users,
        mail_operations: 0,
        audit_logs_24h: 0,
        timestamp: new Date().toISOString(),
      } satisfies StatisticsData;
    },
    enabled: !!companyId,
    refetchInterval: 30000, // Refresh every 30 seconds
    staleTime: 5000, // Consider stale after 5 seconds
  });
}

export function useMailVolumeMetrics(companyId: string, hours: number = 24) {
  return useQuery({
    queryKey: ["mail-volume-metrics", companyId, hours],
    queryFn: async () => {
      const res = await api.get<MailFlowDailyStatsEnvelope>("/mail-flow-logs/daily-stats", {
        params: { days: Math.max(1, Math.ceil(hours / 24)) },
      });
      return res.mail_flow_daily_stats.map((row) => ({
        hour: row.date,
        sent: row.outbound_messages,
        received: row.inbound_messages,
        failed: row.failed,
      }));
    },
    enabled: !!companyId,
    refetchInterval: 60000, // Refresh every minute
    staleTime: 10000,
  });
}

export function useTopDomainsMetrics(companyId: string, limit: number = 10) {
  return useQuery({
    queryKey: ["top-domains-metrics", companyId, limit],
    queryFn: async () => {
      const res = await api.get<DomainListEnvelope>("/domains", {
        params: { company_id: companyId, limit },
      });
      return res.domains.map((domain) => ({
        domain: domain.name,
        mail_count: 0,
        error_rate: 0,
      }));
    },
    enabled: !!companyId,
    refetchInterval: 120000, // Refresh every 2 minutes
    staleTime: 30000,
  });
}
