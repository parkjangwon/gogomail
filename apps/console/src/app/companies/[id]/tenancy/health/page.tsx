'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  Box,
  Spinner,
  ColumnLayout,
  StatusIndicator,
  ProgressBar,
  Badge,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import { useTenantHealth } from '@/hooks';

const fmt = (bytes: number) => {
  if (bytes >= 1e12) return `${(bytes / 1e12).toFixed(1)} TB`;
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`;
  if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`;
  return `${bytes} B`;
};

const statusType = (s: string): 'success' | 'warning' | 'error' => {
  if (s === 'healthy') return 'success';
  if (s === 'warning') return 'warning';
  return 'error';
};

const quotaColor = (pct: number): 'green' | 'red' | 'grey' =>
  pct > 90 ? 'red' : pct > 75 ? 'grey' : 'green';

export default function TenantHealthPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;
  const healthQuery = useTenantHealth(cid, 30_000);
  const health = healthQuery.data ?? null;
  const loading = healthQuery.isLoading;

  if (loading && !health) return <Box padding="xl"><Spinner /></Box>;

  const pct = health?.quota?.usage_pct ?? 0;
  const statusText = health?.status ?? 'warning';

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={health ? `${t('tenant_health.last_checked')}: ${new Date(health.checked_at ?? Date.now()).toLocaleString()}` : ''}
          actions={<Button iconName="refresh" onClick={() => healthQuery.refetch()} loading={loading}>{t('common.refresh')}</Button>}
        >
          {t('nav.tenant_health')}
        </Header>
      }
    >
      <SpaceBetween size="m">
        {health && (
          <>
            <Container header={<Header variant="h2">{t('tenant_health.overall_status')}</Header>}>
              <ColumnLayout columns={2} variant="text-grid">
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.health')}</Box>
                  <StatusIndicator type={statusType(statusText)}>
                    {t(`tenant_health.status_${statusText}`)}
                  </StatusIndicator>
                </div>
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.company')}</Box>
                  <Box>{health.company_name ?? '—'}</Box>
                </div>
              </ColumnLayout>
            </Container>

            <Container header={<Header variant="h2">{t('nav.domains')}</Header>}>
              <ColumnLayout columns={3} variant="text-grid">
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.total_domains')}</Box>
                  <Box variant="h2">{health.domain_count ?? 0}</Box>
                </div>
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.active_domains')}</Box>
                  <SpaceBetween size="xs" direction="horizontal">
                    <Box variant="h2">{health.active_domains ?? 0}</Box>
                    {(health.active_domains ?? 0) < (health.domain_count ?? 0) && (
                      <Badge color="red">{(health.domain_count ?? 0) - (health.active_domains ?? 0)} {t('tenant_health.inactive')}</Badge>
                    )}
                  </SpaceBetween>
                </div>
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.domain_health')}</Box>
                  <StatusIndicator type={(health.active_domains ?? 0) === (health.domain_count ?? 0) ? 'success' : 'warning'}>
                    {(health.active_domains ?? 0) === (health.domain_count ?? 0) ? t('tenant_health.all_active') : t('tenant_health.some_inactive')}
                  </StatusIndicator>
                </div>
              </ColumnLayout>
            </Container>

            <Container header={<Header variant="h2">{t('tenant_health.storage_quota')}</Header>}>
              <SpaceBetween size="m">
                <ColumnLayout columns={3} variant="text-grid">
                  <div>
                    <Box variant="awsui-key-label">{t('tenant_health.used')}</Box>
                    <Box variant="h2">{fmt(health.quota?.used_bytes ?? 0)}</Box>
                  </div>
                  <div>
                    <Box variant="awsui-key-label">{t('tenant_health.total_allocated')}</Box>
                    <Box variant="h2">{fmt(health.quota?.total_bytes ?? 0)}</Box>
                  </div>
                  <div>
                    <Box variant="awsui-key-label">{t('tenant_health.usage')}</Box>
                    <Badge color={quotaColor(pct)}>{pct.toFixed(1)}%</Badge>
                  </div>
                </ColumnLayout>
                <ProgressBar
                  value={pct}
                  additionalInfo={health.over_allocated ? t('tenant_health.over_allocated_hint') : undefined}
                  status={pct > 90 ? 'error' : pct > 75 ? 'in-progress' : 'success'}
                />
              </SpaceBetween>
            </Container>

            <Container header={<Header variant="h2">{t('tenant_health.integrations')}</Header>}>
              <ColumnLayout columns={2} variant="text-grid">
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.active_webhooks')}</Box>
                  <Box variant="h2">{health.active_webhooks}</Box>
                </div>
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.quota_over_allocated')}</Box>
                  <StatusIndicator type={health.over_allocated ? 'error' : 'success'}>
                    {health.over_allocated ? t('tenant_health.yes_action_required') : t('tenant_health.no')}
                  </StatusIndicator>
                </div>
              </ColumnLayout>
            </Container>
          </>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
