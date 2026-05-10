'use client';

import {
  ContentLayout,
  Header,
  Box,
  Spinner,
  SpaceBetween,
  Button,
  Badge,
  Table,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface ReconciliationJob {
  id: string;
  status: string;
  started_at: string;
  completed_at?: string;
  discrepancies_found: number;
  corrections_applied: number;
}

export default function ReconciliationPage() {
  const { t } = useI18n();
  const [jobs, setJobs] = useState<ReconciliationJob[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchReconciliationJobs();
  }, []);

  const fetchReconciliationJobs = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/quota-reconciliation?limit=50', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setJobs(data.jobs || []);
      }
    } catch (error) {
      console.error('Failed to fetch reconciliation jobs:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleRunReconciliation = async () => {
    try {
      await fetch('/api/admin/quota-reconciliation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
      });
      fetchReconciliationJobs();
    } catch (error) {
      console.error('Failed to run reconciliation:', error);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed': return 'green';
      case 'failed': return 'red';
      case 'running': return 'blue';
      case 'pending': return 'severity-high';
      default: return 'grey';
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.reconciliation.title')}</Header>}>
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
          description={t('pages.reconciliation_page.description')}
          actions={
            <Button variant="primary" onClick={handleRunReconciliation}>
              {t('pages.reconciliation_page.run_btn')}
            </Button>
          }
        >
          {t('pages.reconciliation_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.reconciliation_page.status'),
              cell: (item: ReconciliationJob) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.reconciliation_page.started'),
              cell: (item: ReconciliationJob) => new Date(item.started_at).toLocaleString(),
              width: '25%',
            },
            {
              header: t('pages.reconciliation_page.completed'),
              cell: (item: ReconciliationJob) => item.completed_at ? new Date(item.completed_at).toLocaleString() : '-',
              width: '25%',
            },
            {
              header: t('pages.reconciliation_page.discrepancies'),
              cell: (item: ReconciliationJob) => item.discrepancies_found,
              width: '15%',
            },
            {
              header: t('pages.reconciliation_page.corrections'),
              cell: (item: ReconciliationJob) => item.corrections_applied,
              width: '20%',
            },
          ]}
          items={jobs}
          header={<Header variant="h2" counter={`(${jobs.length})`}>{t('pages.reconciliation_page.jobs')}</Header>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
