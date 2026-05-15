import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import {
  buildDashboardWindows,
  composeDashboardData,
  type DashboardData,
  type DashboardDomainShape,
  type DashboardHealthShape,
  type DashboardPostureShape,
  type DashboardSeatUsageShape,
  type MailFlowDailyStatShape,
  type MailFlowStatsShape,
} from '@/lib/dashboardStats';

interface DomainsEnvelope {
  domains?: DashboardDomainShape[];
}

interface HealthEnvelope {
  health?: DashboardHealthShape;
}

interface PostureEnvelope {
  score?: number;
  mfa?: DashboardPostureShape['mfa'];
}

interface SeatUsageEnvelope extends DashboardSeatUsageShape {}

interface MailFlowStatsEnvelope {
  mail_flow_stats?: MailFlowStatsShape;
}

interface MailFlowDailyStatsEnvelope {
  mail_flow_daily_stats?: MailFlowDailyStatShape[];
}

function unwrap<T>(result: PromiseSettledResult<T>): T | null {
  return result.status === 'fulfilled' ? result.value : null;
}

export function useStatistics(companyId: string) {
  return useQuery<DashboardData>({
    queryKey: ['statistics', companyId],
    queryFn: async () => {
      const id = companyId === 'default' ? '' : companyId;
      const windows = buildDashboardWindows();
      const domainParams: Record<string, string | number | boolean> = { limit: 200 };
      if (id) domainParams.company_id = id;

      const [domainsRes, healthRes, postureRes, seatRes, mailStatsRes, mailDailyRes] = await Promise.allSettled([
        api.get<DomainsEnvelope>('/domains', { params: domainParams }),
        id ? api.get<HealthEnvelope>(`/companies/${id}/health`) : Promise.resolve(null),
        id ? api.get<PostureEnvelope>(`/companies/${id}/security/posture`) : Promise.resolve(null),
        id ? api.get<SeatUsageEnvelope>(`/companies/${id}/seat-usage`) : Promise.resolve(null),
        id ? api.get<MailFlowStatsEnvelope>('/mail-flow-logs/stats', {
          params: {
            company_id: id,
            since: windows.mailStatsSince,
            until: windows.mailStatsUntil,
          },
        }) : Promise.resolve(null),
        id ? api.get<MailFlowDailyStatsEnvelope>('/mail-flow-logs/daily-stats', {
          params: {
            company_id: id,
            since: windows.mailDailySince,
            until: windows.mailDailyUntil,
          },
        }) : Promise.resolve(null),
      ]);

      const domains = unwrap(domainsRes)?.domains ?? [];
      const health = unwrap(healthRes)?.health ?? null;
      const postureEnvelope = unwrap(postureRes);
      const seatUsage = unwrap(seatRes);
      const mailStats = unwrap(mailStatsRes)?.mail_flow_stats ?? null;
      const mailDailyStats = unwrap(mailDailyRes)?.mail_flow_daily_stats ?? null;

      return composeDashboardData({
        domains,
        health,
        posture: postureEnvelope ? { score: postureEnvelope.score, mfa: postureEnvelope.mfa } : null,
        seatUsage,
        mailFlowStats: mailStats,
        mailFlowDailyStats: mailDailyStats,
      });
    },
    enabled: !!companyId,
    refetchInterval: 30_000,
    staleTime: 5_000,
  });
}

export { buildDashboardWindows, composeDashboardData };
export type {
  DashboardData,
  DashboardDomainShape,
  DashboardHealthShape,
  DashboardMailVolume,
  DashboardPostureShape,
  DashboardSeatUsageShape,
  DashboardUserActivity,
  MailFlowDailyStatShape,
  MailFlowStatsShape,
} from '@/lib/dashboardStats';
