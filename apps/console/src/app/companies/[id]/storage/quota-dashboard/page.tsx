'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  Box,
  Spinner,
  Alert,
  Flashbar,
  FlashbarProps,
  ColumnLayout,
  Table,
  ProgressBar,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import Link from 'next/link';

interface QuotaSummary {
  total_entries: number;
  total_used_bytes: number;
  total_limit_bytes: number;
  over_limit_count: number;
  usage_ratio: number;
}

interface QuotaConsumer {
  id: string;
  name: string;
  scope: string;
  quota_used: number;
  quota_limit: number;
  usage_ratio: number;
  over_limit: boolean;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${units[i]}`;
}

export default function QuotaDashboardPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const [summary, setSummary] = useState<QuotaSummary | null>(null);
  const [topConsumers, setTopConsumers] = useState<QuotaConsumer[]>([]);
  const [loading, setLoading] = useState(true);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${cid}/quota-summary`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setSummary(data.summary ?? null);
        setTopConsumers(data.top_consumers ?? []);
      } else {
        setNotifications([{ type: 'error', content: t('pages.quota_dashboard_page.no_data'), dismissible: true, onDismiss: () => setNotifications([]), id: 'err' }]);
      }
    } catch {
      setNotifications([{ type: 'error', content: t('pages.quota_dashboard_page.no_data'), dismissible: true, onDismiss: () => setNotifications([]), id: 'err' }]);
    } finally {
      setLoading(false);
    }
  }, [cid, t]);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, [fetchData]);

  if (loading && !summary) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.quota_dashboard_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const overLimitCount = summary?.over_limit_count ?? 0;
  const usageRatioPct = Math.round((summary?.usage_ratio ?? 0) * 100);

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.quota_dashboard_page.description')}
          actions={
            <Button iconName="refresh" onClick={fetchData} loading={loading}>
              {t('pages.quota_dashboard_page.refresh')}
            </Button>
          }
        >
          {t('pages.quota_dashboard_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {notifications.length > 0 && <Flashbar items={notifications} />}

        {overLimitCount > 0 && (
          <Alert type="warning">
            {t('pages.quota_dashboard_page.over_limit_warning')} ({overLimitCount})
          </Alert>
        )}

        {/* Summary stats */}
        <Container>
          <ColumnLayout columns={4} variant="text-grid">
            <SpaceBetween size="xxs">
              <Box variant="awsui-key-label">{t('pages.quota_dashboard_page.total_entries')}</Box>
              <Box variant="awsui-value-large">{summary?.total_entries ?? 0}</Box>
            </SpaceBetween>
            <SpaceBetween size="xxs">
              <Box variant="awsui-key-label">{t('pages.quota_dashboard_page.total_used')}</Box>
              <Box variant="awsui-value-large">{formatBytes(summary?.total_used_bytes ?? 0)}</Box>
            </SpaceBetween>
            <SpaceBetween size="xxs">
              <Box variant="awsui-key-label">{t('pages.quota_dashboard_page.over_limit')}</Box>
              <Box variant="awsui-value-large">
                {overLimitCount > 0 ? (
                  <StatusIndicator type="error">{overLimitCount}</StatusIndicator>
                ) : (
                  <StatusIndicator type="success">0</StatusIndicator>
                )}
              </Box>
            </SpaceBetween>
            <SpaceBetween size="xxs">
              <Box variant="awsui-key-label">{t('pages.quota_dashboard_page.usage_ratio')}</Box>
              <ProgressBar
                value={usageRatioPct}
                status={usageRatioPct >= 90 ? 'error' : usageRatioPct >= 70 ? 'in-progress' : 'success'}
                additionalInfo={`${formatBytes(summary?.total_used_bytes ?? 0)} / ${formatBytes(summary?.total_limit_bytes ?? 0)}`}
              />
            </SpaceBetween>
          </ColumnLayout>
        </Container>

        {/* Top consumers table */}
        <Container
          header={
            <Header
              variant="h2"
              actions={
                <Link href={`/companies/${cid}/storage/quota-usage`}>
                  <Button variant="link">{t('pages.quota_dashboard_page.view_all')}</Button>
                </Link>
              }
            >
              {t('pages.quota_dashboard_page.top_consumers')}
            </Header>
          }
        >
          <Table
            items={topConsumers}
            empty={
              <Box textAlign="center" color="inherit" padding="m">
                {t('pages.quota_dashboard_page.no_data')}
              </Box>
            }
            columnDefinitions={[
              {
                header: t('pages.quota_dashboard_page.name'),
                cell: (item: QuotaConsumer) => item.name || item.id,
              },
              {
                header: t('pages.quota_dashboard_page.scope'),
                cell: (item: QuotaConsumer) => item.scope,
              },
              {
                header: t('pages.quota_dashboard_page.used'),
                cell: (item: QuotaConsumer) => formatBytes(item.quota_used),
              },
              {
                header: t('pages.quota_dashboard_page.limit'),
                cell: (item: QuotaConsumer) => item.quota_limit > 0 ? formatBytes(item.quota_limit) : '∞',
              },
              {
                header: t('pages.quota_dashboard_page.ratio'),
                cell: (item: QuotaConsumer) => (
                  <ProgressBar
                    value={Math.round((item.usage_ratio ?? 0) * 100)}
                    status={item.over_limit ? 'error' : (item.usage_ratio ?? 0) >= 0.9 ? 'in-progress' : 'success'}
                  />
                ),
              },
            ]}
            variant="embedded"
          />
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
