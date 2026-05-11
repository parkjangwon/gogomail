'use client';

import { useQuery } from '@tanstack/react-query';

export interface DashboardData {
  stats: {
    total_users: number;
    active_domains: number;
    domain_count: number;
    total_storage_used: number;
    total_storage_limit: number;
    storage_pct: number;
    over_allocated: boolean;
    active_webhooks: number;
    health_status: 'healthy' | 'warning' | 'degraded' | 'unknown';
  };
  apiUsageMetrics: {
    requests_today: number;
    requests_this_month: number;
  };
  fetchedAt: Date;
}

export function useDashboard(companyId: string) {
  return useQuery<DashboardData>({
    queryKey: ['dashboard', companyId],
    queryFn: async () => {
      const id = companyId === 'default' ? '' : companyId;

      const [domainsRes, healthRes] = await Promise.all([
        fetch(`/admin/v1/domains?${id ? `company_id=${id}&` : ''}limit=200`),
        id ? fetch(`/admin/v1/companies/${id}/health`) : Promise.resolve(null),
      ]);

      type DomainShape = { status: string; quota_used: number; quota_limit: number };
      const domainsData = domainsRes.ok
        ? (await domainsRes.json() as { domains?: DomainShape[] })
        : { domains: [] };

      const healthData = healthRes?.ok
        ? (await healthRes.json() as { health?: Record<string, unknown> })
        : null;

      const domains = domainsData.domains ?? [];
      const activeDomains = domains.filter(d => d.status === 'active').length;
      const totalUsed = domains.reduce((s, d) => s + (d.quota_used ?? 0), 0);
      const totalLimit = domains.reduce((s, d) => s + (d.quota_limit ?? 0), 0);

      const health = healthData?.health ?? {};
      const healthStatus = (health.status as DashboardData['stats']['health_status']) ?? 'unknown';

      return {
        stats: {
          total_users: 0,
          active_domains: activeDomains,
          domain_count: domains.length,
          total_storage_used: totalUsed,
          total_storage_limit: totalLimit,
          storage_pct: totalLimit > 0 ? Math.round((totalUsed / totalLimit) * 100) : 0,
          over_allocated: (health.over_allocated as boolean) ?? false,
          active_webhooks: (health.active_webhooks as number) ?? 0,
          health_status: healthStatus,
        },
        apiUsageMetrics: {
          requests_today: 0,
          requests_this_month: 0,
        },
        fetchedAt: new Date(),
      };
    },
    enabled: !!companyId,
    staleTime: 30_000,
    refetchInterval: 30_000,
  });
}
