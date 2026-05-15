'use client';
import { DataTable } from '@/components/DataTable';
import {
  ContentLayout,
  Header,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Badge,
} from '@cloudscape-design/components';
import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useQuotaAlerts, type QuotaAlert } from '@/hooks';

export default function QuotaAlertsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { data: alerts = [], isLoading } = useQuotaAlerts(companyId);
  const [filter, setFilter] = useState('');

  const filteredAlerts = useMemo(
    () =>
      alerts.filter((alert) =>
        [
          alert.company_id,
          alert.domain_id,
          alert.user_id,
          alert.scope,
          alert.alert_type,
        ]
          .filter((value): value is string => Boolean(value))
          .some((value) => value.toLowerCase().includes(filter.toLowerCase()))
      ),
    [alerts, filter]
  );

  const getAlertColor = (status: QuotaAlert['alert_type']) => {
    switch (status) {
      case 'critical':
        return 'red';
      case 'warning':
        return 'severity-high';
      case 'exhausted':
        return 'red';
      default:
        return 'grey';
    }
  };

  const getScopeColor = (scope: QuotaAlert['scope']) => {
    switch (scope) {
      case 'company':
        return 'blue';
      case 'domain':
        return 'green';
      case 'user':
        return 'grey';
      default:
        return 'grey';
    }
  };

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.quota_alerts.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.quota_alerts_page.description')}
        >
          {t('pages.quota_alerts_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: 'Scope',
              cell: (item: QuotaAlert) => (
                <Badge color={getScopeColor(item.scope)}>{item.scope}</Badge>
              ),
              width: '25%',
            },
            {
              header: 'Alert Type',
              cell: (item: QuotaAlert) => (
                <Badge color={getAlertColor(item.alert_type)}>{item.alert_type}</Badge>
              ),
              width: '15%',
            },
            {
              header: 'Usage',
              cell: (item: QuotaAlert) => `${(item.usage_ratio * 100).toFixed(1)}%`,
              width: '12%',
            },
            {
              header: 'Limit',
              cell: (item: QuotaAlert) => item.quota_limit.toLocaleString(),
              width: '12%',
            },
            {
              header: t('pages.quota_alerts_page.created'),
              cell: (item: QuotaAlert) => new Date(item.created_at).toLocaleString(),
              width: '18%',
            },
            {
              header: t('pages.quota_alerts_page.domain_id'),
              cell: (item: QuotaAlert) => item.domain_id || '—',
              width: '10%',
            },
            {
              header: t('pages.quota_alerts_page.user_id'),
              cell: (item: QuotaAlert) => item.user_id || '—',
              width: '10%',
            },
          ]}
          items={filteredAlerts}
          header={<Header variant="h2" counter={`(${filteredAlerts.length})`}>{t('pages.quota_alerts_page.alerts')}</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
