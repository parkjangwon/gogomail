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
  Badge,
} from '@cloudscape-design/components';
import { useParams, useRouter } from 'next/navigation';
import { useDashboard } from '@/hooks/useDashboard';
import { useCompany } from '@/contexts/CompanyContext';
import { useI18n } from '@/app/i18n-provider';
import { useQueryClient } from '@tanstack/react-query';
import { useState, useEffect } from 'react';

const healthIndicator = (status: string) => {
  if (status === 'healthy') return <StatusIndicator type="success">Healthy</StatusIndicator>;
  if (status === 'warning') return <StatusIndicator type="warning">Warning</StatusIndicator>;
  if (status === 'degraded') return <StatusIndicator type="error">Degraded</StatusIndicator>;
  return <StatusIndicator type="pending">Unknown</StatusIndicator>;
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

  // Countdown to next auto-refresh
  useEffect(() => {
    if (!data) return;
    setCountdown(30);
    const iv = setInterval(() => setCountdown(c => (c <= 1 ? 30 : c - 1)), 1000);
    return () => clearInterval(iv);
  }, [data?.fetchedAt]);

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
    active_domains: 0,
    domain_count: 0,
    total_storage_used: 0,
    total_storage_limit: 0,
    storage_pct: 0,
    over_allocated: false,
    active_webhooks: 0,
    health_status: 'unknown' as const,
  };

  const storageUsed = stats.total_storage_used;
  const storageLimit = stats.total_storage_limit;
  const storagePct = stats.storage_pct;

  const fetchedAt = data?.fetchedAt;
  const lastUpdated = fetchedAt
    ? fetchedAt.toLocaleTimeString()
    : '—';

  const quickLinks = [
    { labelKey: 'manage_users', path: '/users' },
    { labelKey: 'manage_domains', path: '/tenancy/domains' },
    { labelKey: 'audit_logs', path: '/audit-logs' },
    { labelKey: 'dkim_keys', path: '/security/dkim-keys' },
    { labelKey: 'quota_usage', path: '/storage/quota-usage' },
    { labelKey: 'api_health', path: '/system/health' },
  ];

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={currentCompany ? `${currentCompany.name} — ${currentCompany.status}` : t('pages.dashboard_page.system_overview')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Box color="text-status-inactive" padding={{ top: 'xs' }} fontSize="body-s">
                Updated {lastUpdated} · refreshes in {countdown}s
              </Box>
              <Button iconName="refresh" loading={isFetching} onClick={handleRefresh}>Refresh</Button>
              <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/tenancy/health`)}>
                Health
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
              <Box color="text-body-secondary" fontSize="body-s">Tenant Health</Box>
              {healthIndicator(stats.health_status)}
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{stats.active_domains}</Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {t('pages.dashboard_page.active_domains')}
                {stats.domain_count > stats.active_domains && (
                  <Badge color="red"> {stats.domain_count - stats.active_domains} inactive</Badge>
                )}
              </Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{fmtGb(storageUsed)}</Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {storageLimit > 0 ? `/ ${fmtGb(storageLimit)} (${storagePct}%)` : 'Storage Used'}
              </Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{stats.active_webhooks}</Box>
              <Box color="text-body-secondary" fontSize="body-s">Active Webhooks</Box>
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
                  { label: 'Domains', value: `${stats.active_domains} / ${stats.domain_count} active` },
                ]}
              />
            </SpaceBetween>
          </Container>

          {/* Quick Links */}
          <Container header={<Header variant="h3">{t('pages.dashboard_page.quick_actions')}</Header>}>
            <SpaceBetween size="s">
              {quickLinks.map(({ labelKey, path }) => (
                <Button
                  key={labelKey}
                  variant="inline-link"
                  onClick={() => router.push(`/companies/${companyId}${path}`)}
                >
                  {t(`pages.dashboard_page.${labelKey}`)} →
                </Button>
              ))}
            </SpaceBetween>
          </Container>
        </ColumnLayout>
      </SpaceBetween>
    </ContentLayout>
  );
}
