'use client';

import {
  ContentLayout,
  Header,
  Box,
  Spinner,
  SpaceBetween,
  Container,
  ColumnLayout,
  ProgressBar,
  KeyValuePairs,
  Button,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useParams, useRouter } from 'next/navigation';
import { useDashboard } from '@/hooks/useDashboard';
import { useCompany } from '@/contexts/CompanyContext';
import { useI18n } from '@/app/i18n-provider';
import { useQueryClient } from '@tanstack/react-query';
import { useState, useEffect } from 'react';
import { getRecentVisits } from '@/components/AdminLayout';

const healthIndicator = (status: string, t: (key: string, defaultValue?: string) => string) => {
  if (status === 'healthy') return <StatusIndicator type="success">{t('status.healthy')}</StatusIndicator>;
  if (status === 'warning') return <StatusIndicator type="warning">{t('status.warning')}</StatusIndicator>;
  if (status === 'degraded') return <StatusIndicator type="error">{t('status.degraded')}</StatusIndicator>;
  return <StatusIndicator type="pending">{t('status.unknown')}</StatusIndicator>;
};

const fmtGb = (bytes: number) => `${(bytes / 1073741824).toFixed(1)} GB`;

export default function DashboardPage() {
  const { t } = useI18n();
  const params = useParams();
  const router = useRouter();
  const companyId = params.id as string;
  const { currentCompany } = useCompany();
  const queryClient = useQueryClient();
  const { data, isLoading, isFetching } = useDashboard(companyId);
  const [countdown, setCountdown] = useState(30);
  const [recentVisitMap, setRecentVisitMap] = useState<Map<string, number>>(new Map());

  // Countdown to next auto-refresh
  useEffect(() => {
    if (!data) return;
    setCountdown(30);
    const iv = setInterval(() => setCountdown(c => (c <= 1 ? 30 : c - 1)), 1000);
    return () => clearInterval(iv);
  }, [data?.fetchedAt]);

  useEffect(() => {
    const visits = getRecentVisits();
    const map = new Map<string, number>();
    for (const v of visits) map.set(v.path, v.ts);
    setRecentVisitMap(map);
  }, []);

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['dashboard', companyId] });
  };

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.dashboard_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const stats = data?.stats ?? {
    total_users: 0,
    active_users: 0,
    suspended_users: 0,
    active_domains: 0,
    domain_count: 0,
    total_storage_used: 0,
    total_storage_limit: 0,
    storage_pct: 0,
    over_allocated: false,
    active_webhooks: 0,
    health_status: 'unknown' as const,
    security_score: 0,
    mfa_rate: 0,
    mfa_enabled: 0,
    mfa_total: 0,
  };

  const storageUsed = stats.total_storage_used;
  const storageLimit = stats.total_storage_limit;
  const storagePct = stats.storage_pct;

  const fetchedAt = data?.fetchedAt;
  const lastUpdated = fetchedAt
    ? fetchedAt.toLocaleTimeString()
    : '—';

  const formatRelativeVisit = (ts: number) => {
    const mins = Math.floor((Date.now() - ts) / 60000);
    if (mins < 1) return t('pages.dashboard_page.just_now');
    if (mins < 60) return t('pages.dashboard_page.minutes_ago').replace('{n}', String(mins));
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return t('pages.dashboard_page.hours_ago').replace('{n}', String(hrs));
    return t('pages.dashboard_page.days_ago').replace('{n}', String(Math.floor(hrs / 24)));
  };

  const ALL_QUICK_LINKS = [
    { labelKey: 'manage_users', path: '/users' },
    { labelKey: 'manage_domains', path: '/tenancy/domains' },
    { labelKey: 'audit_logs', path: '/audit-logs' },
    { labelKey: 'dkim_keys', path: '/security/dkim-keys' },
    { labelKey: 'quota_usage', path: '/storage/quota-usage' },
    { labelKey: 'api_health', path: '/system/health' },
  ];

  const quickLinks = [...ALL_QUICK_LINKS].sort((a, b) => {
    const tsA = recentVisitMap.get(`/companies/${companyId}${a.path}`) ?? 0;
    const tsB = recentVisitMap.get(`/companies/${companyId}${b.path}`) ?? 0;
    return tsB - tsA;
  });

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={currentCompany ? `${currentCompany.name} — ${currentCompany.status}` : t('pages.dashboard_page.system_overview')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Box color="text-status-inactive" padding={{ top: 'xs' }} fontSize="body-s">
                {t('pages.dashboard_page.updated')} {lastUpdated} · {t('pages.dashboard_page.refreshes_in')} {countdown}s
              </Box>
              <Button iconName="refresh" loading={isFetching} onClick={handleRefresh}>{t('common.refresh')}</Button>
              <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/tenancy/health`)}>
                {t('pages.dashboard_page.health')}
              </Button>
            </SpaceBetween>
          }
        >
          {t('pages.dashboard_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* Health + KPI row */}
        <ColumnLayout columns={4} variant="text-grid" minColumnWidth={140}>
          <Container>
            <SpaceBetween size="xs">
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.dashboard_page.tenant_health')}</Box>
              {healthIndicator(stats.health_status, t)}
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold"
                color={stats.security_score >= 80 ? 'text-status-success' : stats.security_score >= 50 ? 'text-status-warning' : 'text-status-error'}>
                {stats.security_score > 0 ? `${stats.security_score}/100` : '—'}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.dashboard_page.security_score')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{stats.total_users > 0 ? stats.total_users : stats.active_domains}</Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {stats.total_users > 0
                  ? t('pages.dashboard_page.users_active').replace('{total}', String(stats.total_users)).replace('{active}', String(stats.active_users))
                  : t('pages.dashboard_page.active_domains')}
              </Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">
                {stats.mfa_total > 0 ? `${stats.mfa_rate.toFixed(0)}%` : '—'}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {t('pages.dashboard_page.mfa_adoption')}{stats.mfa_total > 0 ? ` (${stats.mfa_enabled}/${stats.mfa_total})` : ''}
              </Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        <ColumnLayout columns={2}>
          {/* Storage */}
          <Container header={<Header variant="h3">{t('pages.dashboard_page.storage_quota')}</Header>}>
            <SpaceBetween size="m">
              {storageLimit > 0 ? (
                <ProgressBar
                  value={storagePct}
                  status={stats.over_allocated ? 'error' : storagePct > 80 ? 'in-progress' : 'success'}
                  additionalInfo={`${fmtGb(storageUsed)} / ${fmtGb(storageLimit)}`}
                />
              ) : (
                <StatusIndicator type="success">{t('pages.dashboard_page.unlimited_storage')}</StatusIndicator>
              )}
              {stats.over_allocated && (
                <StatusIndicator type="error">{t('pages.dashboard_page.over_allocated')}</StatusIndicator>
              )}
              <KeyValuePairs
                items={[
                  { label: t('pages.dashboard_page.used_label'), value: fmtGb(storageUsed) },
                  { label: t('pages.dashboard_page.limit_label'), value: storageLimit > 0 ? fmtGb(storageLimit) : t('pages.dashboard_page.unlimited_label') },
                  {
                    label: t('pages.dashboard_page.domains'),
                    value: t('pages.dashboard_page.domains_active')
                      .replace('{active}', String(stats.active_domains))
                      .replace('{total}', String(stats.domain_count)),
                  },
                ]}
              />
            </SpaceBetween>
          </Container>

          {/* Quick Links */}
          <Container header={<Header variant="h3">{t('pages.dashboard_page.quick_actions')}</Header>}>
            <SpaceBetween size="s">
              {quickLinks.map(({ labelKey, path }) => {
                const ts = recentVisitMap.get(`/companies/${companyId}${path}`);
                const ago = ts ? (() => {
                  return formatRelativeVisit(ts);
                })() : null;
                return (
                  <SpaceBetween key={labelKey} direction="horizontal" size="xs">
                    <Button
                      variant="inline-link"
                      onClick={() => router.push(`/companies/${companyId}${path}`)}
                    >
                      {t(`pages.dashboard_page.${labelKey}`)} →
                    </Button>
                    {ago && <Box color="text-body-secondary" fontSize="body-s" padding={{ top: 'xxs' }}>{ago}</Box>}
                  </SpaceBetween>
                );
              })}
            </SpaceBetween>
          </Container>
        </ColumnLayout>
      </SpaceBetween>
    </ContentLayout>
  );
}
