'use client';

import { useQuery } from '@tanstack/react-query';

export interface DashboardData {
  stats: {
    total_users: number;
    active_domains: number;
    total_storage_used: number;
    total_storage_limit: number;
    over_allocated: boolean;
  };
  apiUsageMetrics: {
    requests_today: number;
    requests_this_month: number;
  };
}

export function useDashboard(companyId: string) {
  return useQuery<DashboardData>({
    queryKey: ['dashboard', companyId],
    queryFn: async () => {
      const id = companyId === 'default' ? '' : companyId;
      const [companiesRes, domainsRes, usersRes] = await Promise.all([
        fetch(id ? `/api/admin/companies/${id}` : '/api/admin/companies?limit=1', { credentials: 'include' }),
        fetch(`/api/admin/domains?${id ? `company_id=${id}&` : ''}limit=200`, { credentials: 'include' }),
        fetch(`/api/admin/users?${id ? `company_id=${id}&` : ''}limit=1`, { credentials: 'include' }),
      ]);

      type CompanyShape = { quota_used?: number; quota_limit?: number; over_allocated?: boolean };
      const [companiesData, domainsData, usersData] = await Promise.all([
        companiesRes.ok ? companiesRes.json() : {},
        domainsRes.ok ? domainsRes.json() : { domains: [] },
        usersRes.ok ? usersRes.json() : { users: [], total: 0 },
      ]) as [Record<string, unknown>, { domains?: Array<{ status: string }> }, { total?: number; users?: unknown[] }];

      const company: CompanyShape = id
        ? (companiesData.company as CompanyShape ?? {})
        : ((companiesData.companies as CompanyShape[])?.[0] ?? {});
      const domains = domainsData.domains ?? [];
      const activeDomains = domains.filter(d => d.status === 'active').length;

      return {
        stats: {
          total_users: usersData.total ?? usersData.users?.length ?? 0,
          active_domains: activeDomains,
          total_storage_used: company.quota_used ?? 0,
          total_storage_limit: company.quota_limit ?? 0,
          over_allocated: company.over_allocated ?? false,
        },
        apiUsageMetrics: {
          requests_today: 0,
          requests_this_month: 0,
        },
      };
    },
    enabled: !!companyId,
    staleTime: 30_000,
  });
}
