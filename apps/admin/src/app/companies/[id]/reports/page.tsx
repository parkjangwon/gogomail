'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Container,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface Report {
  id: string;
  name: string;
  type: string;
  generated_at: string;
  file_size: number;
}

export default function ReportsPage() {
  const { t } = useI18n();
  const [reports, setReports] = useState<Report[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchReports();
  }, []);

  const fetchReports = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/reports?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setReports(data.reports || []);
      }
    } catch (error) {
      console.error('Failed to fetch reports:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.reports.title')}</Header>}>
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
          description={t('pages.reports_page.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.reports_page.generate_report')}
            </Button>
          }
        >
          {t('pages.reports.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h3">{t('pages.reports_page.available_reports')}</Header>}>
          <Box color="text-body-secondary">
            {t('pages.reports_page.reports_desc')}
          </Box>
        </Container>

        <Table
          columnDefinitions={[
            {
              header: t('pages.reports_page.name'),
              cell: (item: Report) => item.name,
              width: '30%',
            },
            {
              header: t('pages.reports_page.type'),
              cell: (item: Report) => item.type,
              width: '25%',
            },
            {
              header: t('pages.reports_page.size'),
              cell: (item: Report) => `${(item.file_size / 1024 / 1024).toFixed(2)} MB`,
              width: '15%',
            },
            {
              header: t('pages.reports_page.generated'),
              cell: (item: Report) => new Date(item.generated_at).toLocaleString(),
              width: '30%',
            },
          ]}
          items={reports}
          header={<Header variant="h2" counter={`(${reports.length})`}>{t('pages.reports.title')}</Header>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
