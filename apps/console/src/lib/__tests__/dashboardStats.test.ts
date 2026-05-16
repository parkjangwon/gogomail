import { describe, expect, it } from 'vitest';
import { buildDashboardWindows, composeDashboardData } from '../dashboardStats';

describe('dashboardStats', () => {
  it('builds bounded dashboard windows', () => {
    const windows = buildDashboardWindows(new Date('2026-05-15T12:00:00Z'));

    expect(windows).toEqual({
      mailStatsSince: '2026-05-14T12:00:00.000Z',
      mailStatsUntil: '2026-05-15T12:00:00.000Z',
      mailDailySince: '2026-05-08T12:00:00.000Z',
      mailDailyUntil: '2026-05-15T12:00:00.000Z',
    });
  });

  it('composes dashboard totals from the available sources', () => {
    const dashboard = composeDashboardData({
      domains: [
        { status: 'active', quota_used: 1024, quota_limit: 4096 },
        { status: 'suspended', quota_used: 2048, quota_limit: 4096 },
      ],
      health: { status: 'warning', over_allocated: true, active_webhooks: 3 },
      posture: { score: 82, mfa: { total: 50, enabled: 35, rate: 70 } },
      seatUsage: { total_users: 100, active_users: 72, suspended_users: 8 },
      mailFlowStats: { total_messages: 240, delivered: 222, failed: 8, bounced: 4, filtered: 3, rejected: 1 },
      mailFlowDailyStats: [
        { date: '2026-05-13', inbound_messages: 25, outbound_messages: 35, delivered: 52, failed: 2, bounced: 1, filtered: 0, rejected: 0 },
        { date: '2026-05-14', inbound_messages: 30, outbound_messages: 40, delivered: 68, failed: 1, bounced: 2, filtered: 1, rejected: 0 },
      ],
    });

    expect(dashboard.stats).toMatchObject({
      total_users: 100,
      active_users: 72,
      suspended_users: 8,
      active_domains: 1,
      domain_count: 2,
      total_storage_used: 3072,
      total_storage_limit: 8192,
      storage_pct: 38,
      over_allocated: true,
      active_webhooks: 3,
      health_status: 'warning',
      security_score: 82,
      mfa_rate: 70,
      mfa_enabled: 35,
      mfa_total: 50,
    });

    expect(dashboard.userActivity).toEqual({
      total_users: 100,
      active_users: 72,
      suspended_users: 8,
      active_rate: 72,
    });

    expect(dashboard.mailVolume).toEqual({
      total_24h: 240,
      delivered_24h: 222,
      failed_24h: 8,
      bounced_24h: 4,
      filtered_24h: 3,
      rejected_24h: 1,
      inbound_7d: 55,
      outbound_7d: 75,
      total_7d: 130,
      average_7d: 65,
      delivered_7d: 120,
      failed_7d: 3,
      bounced_7d: 3,
      filtered_7d: 1,
      rejected_7d: 0,
      daily: [
        { date: '2026-05-13', inbound_messages: 25, outbound_messages: 35, delivered: 52, failed: 2, bounced: 1, filtered: 0, rejected: 0, total: 60 },
        { date: '2026-05-14', inbound_messages: 30, outbound_messages: 40, delivered: 68, failed: 1, bounced: 2, filtered: 1, rejected: 0, total: 70 },
      ],
    });
  });
});
