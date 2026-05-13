'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  Box,
  Spinner,
  Flashbar,
  FlashbarProps,
  ColumnLayout,
  StatusIndicator,
  ProgressBar,
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface TenantHealth {
  status: 'healthy' | 'warning' | 'degraded';
  company_id: string;
  company_name: string;
  domain_count: number;
  active_domains: number;
  active_webhooks: number;
  over_allocated: boolean;
  quota: {
    total_bytes: number;
    used_bytes: number;
    usage_pct: number;
  };
  checked_at: string;
}

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

  const [health, setHealth] = useState<TenantHealth | null>(null);
  const [loading, setLoading] = useState(true);
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/health`);
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setHealth(data.health);
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setLoading(false);
    }
  }, [cid]);

  useEffect(() => {
    load();
    const iv = setInterval(load, 30_000);
    return () => clearInterval(iv);
  }, [load]);

  if (loading && !health) return <Box padding="xl"><Spinner /></Box>;

  const pct = health?.quota.usage_pct ?? 0;

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={health ? `${t('tenant_health.last_checked')}: ${new Date(health.checked_at).toLocaleString()}` : ''}
          actions={<Button iconName="refresh" onClick={load} loading={loading}>{t('common.refresh')}</Button>}
        >
          {t('nav.tenant_health')}
        </Header>
      }
    >
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}

        {health && (
          <>
            <Container header={<Header variant="h2">{t('tenant_health.overall_status')}</Header>}>
              <ColumnLayout columns={2} variant="text-grid">
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.health')}</Box>
                  <StatusIndicator type={statusType(health.status)}>
                    {t(`tenant_health.status_${health.status}`)}
                  </StatusIndicator>
                </div>
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.company')}</Box>
                  <Box>{health.company_name}</Box>
                </div>
              </ColumnLayout>
            </Container>

            <Container header={<Header variant="h2">{t('nav.domains')}</Header>}>
              <ColumnLayout columns={3} variant="text-grid">
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.total_domains')}</Box>
                  <Box variant="h2">{health.domain_count}</Box>
                </div>
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.active_domains')}</Box>
                  <SpaceBetween size="xs" direction="horizontal">
                    <Box variant="h2">{health.active_domains}</Box>
                    {health.active_domains < health.domain_count && (
                      <Badge color="red">{health.domain_count - health.active_domains} {t('tenant_health.inactive')}</Badge>
                    )}
                  </SpaceBetween>
                </div>
                <div>
                  <Box variant="awsui-key-label">{t('tenant_health.domain_health')}</Box>
                  <StatusIndicator type={health.active_domains === health.domain_count ? 'success' : 'warning'}>
                    {health.active_domains === health.domain_count ? t('tenant_health.all_active') : t('tenant_health.some_inactive')}
                  </StatusIndicator>
                </div>
              </ColumnLayout>
            </Container>

            <Container header={<Header variant="h2">{t('tenant_health.storage_quota')}</Header>}>
              <SpaceBetween size="m">
                <ColumnLayout columns={3} variant="text-grid">
                  <div>
                    <Box variant="awsui-key-label">{t('tenant_health.used')}</Box>
                    <Box variant="h2">{fmt(health.quota.used_bytes)}</Box>
                  </div>
                  <div>
                    <Box variant="awsui-key-label">{t('tenant_health.total_allocated')}</Box>
                    <Box variant="h2">{fmt(health.quota.total_bytes)}</Box>
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
