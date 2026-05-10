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
          description="Run quota reconciliation and fix discrepancies"
          actions={
            <Button variant="primary" onClick={handleRunReconciliation}>
              Run Reconciliation
            </Button>
          }
        >
          Quota Reconciliation
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Status',
              cell: (item: ReconciliationJob) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: 'Started',
              cell: (item: ReconciliationJob) => new Date(item.started_at).toLocaleString(),
              width: '25%',
            },
            {
              header: 'Completed',
              cell: (item: ReconciliationJob) => item.completed_at ? new Date(item.completed_at).toLocaleString() : '-',
              width: '25%',
            },
            {
              header: 'Discrepancies Found',
              cell: (item: ReconciliationJob) => item.discrepancies_found,
              width: '15%',
            },
            {
              header: 'Corrections Applied',
              cell: (item: ReconciliationJob) => item.corrections_applied,
              width: '20%',
            },
          ]}
          items={jobs}
          header={<Header variant="h2" counter={`(${jobs.length})`}>Jobs</Header>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
