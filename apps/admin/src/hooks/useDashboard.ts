'use client';

import { useQuery } from '@tanstack/react-query';

export interface DashboardData {
  stats: {
    total_users: number;
    active_domains: number;
    total_storage_gb: number;
  };
  activityMetrics: Array<{
    date: string;
    user_logins: number;
    api_calls: number;
    mail_sent: number;
  }>;
  apiUsageMetrics: {
    requests_today: number;
    requests_this_month: number;
  };
  securityEvents: Array<{
    id: string;
    event_type: string;
    severity: string;
    description: string;
    timestamp: string;
  }>;
}

export function useDashboard(companyId: string) {
  return useQuery<DashboardData>({
    queryKey: ['dashboard', companyId],
    queryFn: async () => {
      const now = new Date();
      const daysAgo = Array.from({ length: 7 }, (_, i) => {
        const d = new Date(now);
        d.setDate(d.getDate() - i);
        return d;
      }).reverse();

      return {
        stats: {
          total_users: 150,
          active_domains: 25,
          total_storage_gb: 1250,
        },
        activityMetrics: daysAgo.map((d) => ({
          date: d.toISOString().split('T')[0],
          user_logins: Math.floor(Math.random() * 500),
          api_calls: Math.floor(Math.random() * 2000),
          mail_sent: Math.floor(Math.random() * 5000),
        })),
        apiUsageMetrics: {
          requests_today: 1523,
          requests_this_month: 45230,
        },
        securityEvents: [
          {
            id: '1',
            event_type: 'Failed Login Attempt',
            severity: 'low',
            description: 'Failed authentication attempt from 192.168.1.100',
            timestamp: new Date(Date.now() - 3600000).toISOString(),
          },
        ],
      };
    },
    enabled: !!companyId,
  });
}
