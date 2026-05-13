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
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface QuotaAlert {
  id: string;
  tenant: string;
  threshold_percent: number;
  current_percent: number;
  alert_status: string;
  created_at: string;
}

export default function QuotaAlertsPage() {
  const { t } = useI18n();
  const [alerts, setAlerts] = useState<QuotaAlert[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchQuotaAlerts();
  }, []);

  const fetchQuotaAlerts = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/quota-alerts?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setAlerts(data.alerts || []);
      }
    } catch (error) {
      console.error('Failed to fetch quota alerts:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredAlerts = alerts.filter(a =>
    a.tenant.toLowerCase().includes(filter.toLowerCase())
  );

  const getAlertColor = (status: string) => {
    switch (status) {
      case 'critical': return 'red';
      case 'warning': return 'severity-high';
      case 'normal': return 'green';
      default: return 'grey';
    }
  };

  if (loading) {
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
              header: t('pages.quota_alerts_page.tenant'),
              cell: (item: QuotaAlert) => item.tenant,
              width: '25%',
            },
            {
              header: t('pages.quota_alerts_page.threshold'),
              cell: (item: QuotaAlert) => `${item.threshold_percent}%`,
              width: '15%',
            },
            {
              header: t('pages.quota_alerts_page.current_usage'),
              cell: (item: QuotaAlert) => `${item.current_percent.toFixed(1)}%`,
              width: '15%',
            },
            {
              header: t('pages.quota_alerts_page.status'),
              cell: (item: QuotaAlert) => (
                <Badge color={getAlertColor(item.alert_status)}>
                  {item.alert_status}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: t('pages.quota_alerts_page.created'),
              cell: (item: QuotaAlert) => new Date(item.created_at).toLocaleString(),
              width: '25%',
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
