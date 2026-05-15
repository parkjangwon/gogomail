export interface DashboardDomainShape {
  status: string;
  quota_used: number;
  quota_limit: number;
}

export interface DashboardHealthShape {
  status?: string;
  over_allocated?: boolean;
  active_webhooks?: number;
}

export interface DashboardPostureShape {
  score?: number;
  mfa?: {
    total: number;
    enabled: number;
    rate: number;
  };
}

export interface DashboardSeatUsageShape {
  total_users?: number;
  active_users?: number;
  suspended_users?: number;
}

export interface MailFlowStatsShape {
  total_messages?: number;
  unique_senders?: number;
  unique_domains?: number;
  total_size_bytes?: number;
  average_size_bytes?: number;
  max_size_bytes?: number;
  delivered?: number;
  failed?: number;
  bounced?: number;
  filtered?: number;
  rejected?: number;
  delivery_rate?: number;
}

export interface MailFlowDailyStatShape {
  date: string;
  inbound_messages: number;
  outbound_messages: number;
  failed: number;
}

export interface DashboardMailVolume {
  total_24h: number;
  delivered_24h: number;
  failed_24h: number;
  bounced_24h: number;
  filtered_24h: number;
  rejected_24h: number;
  inbound_7d: number;
  outbound_7d: number;
  total_7d: number;
  average_7d: number;
  daily: Array<{
    date: string;
    inbound_messages: number;
    outbound_messages: number;
    failed: number;
    total: number;
  }>;
}

export interface DashboardUserActivity {
  total_users: number;
  active_users: number;
  suspended_users: number;
  active_rate: number;
}

export interface DashboardData {
  stats: {
    total_users: number;
    active_users: number;
    suspended_users: number;
    active_domains: number;
    domain_count: number;
    total_storage_used: number;
    total_storage_limit: number;
    storage_pct: number;
    over_allocated: boolean;
    active_webhooks: number;
    health_status: 'healthy' | 'warning' | 'degraded' | 'unknown';
    security_score: number;
    mfa_rate: number;
    mfa_enabled: number;
    mfa_total: number;
  };
  userActivity: DashboardUserActivity;
  mailVolume: DashboardMailVolume;
  apiUsageMetrics: {
    requests_today: number;
    requests_this_month: number;
  };
  fetchedAt: Date;
}

export interface DashboardStatsInput {
  domains?: DashboardDomainShape[];
  health?: DashboardHealthShape | null;
  posture?: DashboardPostureShape | null;
  seatUsage?: DashboardSeatUsageShape | null;
  mailFlowStats?: MailFlowStatsShape | null;
  mailFlowDailyStats?: MailFlowDailyStatShape[] | null;
}

export interface DashboardWindows {
  mailStatsSince: string;
  mailStatsUntil: string;
  mailDailySince: string;
  mailDailyUntil: string;
}

export function buildDashboardWindows(now: Date = new Date()): DashboardWindows {
  const mailStatsUntil = new Date(now);
  const mailStatsSince = new Date(mailStatsUntil.getTime() - 24 * 60 * 60 * 1000);
  const mailDailyUntil = new Date(now);
  const mailDailySince = new Date(mailDailyUntil.getTime() - 7 * 24 * 60 * 60 * 1000);

  return {
    mailStatsSince: mailStatsSince.toISOString(),
    mailStatsUntil: mailStatsUntil.toISOString(),
    mailDailySince: mailDailySince.toISOString(),
    mailDailyUntil: mailDailyUntil.toISOString(),
  };
}

export function composeDashboardData(input: DashboardStatsInput): DashboardData {
  const domains = input.domains ?? [];
  const activeDomains = domains.filter((domain) => domain.status === 'active').length;
  const totalStorageUsed = domains.reduce((sum, domain) => sum + (domain.quota_used ?? 0), 0);
  const totalStorageLimit = domains.reduce((sum, domain) => sum + (domain.quota_limit ?? 0), 0);
  const healthStatus = normalizeHealthStatus(input.health?.status);
  const totalUsers = input.seatUsage?.total_users ?? 0;
  const activeUsers = input.seatUsage?.active_users ?? 0;
  const suspendedUsers = input.seatUsage?.suspended_users ?? 0;
  const daily = input.mailFlowDailyStats ?? [];
  const dailyTotals = daily.reduce(
    (acc, row) => {
      acc.inbound += row.inbound_messages ?? 0;
      acc.outbound += row.outbound_messages ?? 0;
      acc.failed += row.failed ?? 0;
      return acc;
    },
    { inbound: 0, outbound: 0, failed: 0 },
  );
  const total24h = input.mailFlowStats?.total_messages ?? 0;

  return {
    stats: {
      total_users: totalUsers,
      active_users: activeUsers,
      suspended_users: suspendedUsers,
      active_domains: activeDomains,
      domain_count: domains.length,
      total_storage_used: totalStorageUsed,
      total_storage_limit: totalStorageLimit,
      storage_pct: totalStorageLimit > 0 ? Math.round((totalStorageUsed / totalStorageLimit) * 100) : 0,
      over_allocated: input.health?.over_allocated ?? false,
      active_webhooks: input.health?.active_webhooks ?? 0,
      health_status: healthStatus,
      security_score: input.posture?.score ?? 0,
      mfa_rate: input.posture?.mfa?.rate ?? 0,
      mfa_enabled: input.posture?.mfa?.enabled ?? 0,
      mfa_total: input.posture?.mfa?.total ?? 0,
    },
    userActivity: {
      total_users: totalUsers,
      active_users: activeUsers,
      suspended_users: suspendedUsers,
      active_rate: totalUsers > 0 ? Math.round((activeUsers / totalUsers) * 100) : 0,
    },
    mailVolume: {
      total_24h: total24h,
      delivered_24h: input.mailFlowStats?.delivered ?? 0,
      failed_24h: input.mailFlowStats?.failed ?? 0,
      bounced_24h: input.mailFlowStats?.bounced ?? 0,
      filtered_24h: input.mailFlowStats?.filtered ?? 0,
      rejected_24h: input.mailFlowStats?.rejected ?? 0,
      inbound_7d: dailyTotals.inbound,
      outbound_7d: dailyTotals.outbound,
      total_7d: dailyTotals.inbound + dailyTotals.outbound,
      average_7d: daily.length > 0 ? Math.round((dailyTotals.inbound + dailyTotals.outbound) / daily.length) : 0,
      daily: daily.map((row) => ({
        date: row.date,
        inbound_messages: row.inbound_messages ?? 0,
        outbound_messages: row.outbound_messages ?? 0,
        failed: row.failed ?? 0,
        total: (row.inbound_messages ?? 0) + (row.outbound_messages ?? 0),
      })),
    },
    apiUsageMetrics: {
      requests_today: 0,
      requests_this_month: 0,
    },
    fetchedAt: new Date(),
  };
}

function normalizeHealthStatus(status: string | undefined): DashboardData['stats']['health_status'] {
  if (status === 'healthy' || status === 'warning' || status === 'degraded') return status;
  return 'unknown';
}
