'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Table,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface ComplianceReport {
  id: string;
  framework: string;
  status: string;
  last_audit: string;
  findings: number;
}

export default function CompliancePage() {
  const { t } = useI18n();
  const [reports, setReports] = useState<ComplianceReport[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchComplianceReports();
  }, []);

  const fetchComplianceReports = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/compliance', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setReports(data.reports || []);
      }
    } catch (error) {
      console.error('Failed to fetch compliance reports:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'compliant': return 'green';
      case 'non-compliant': return 'red';
      case 'pending': return 'blue';
      case 'partial': return 'severity-high';
      default: return 'grey';
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.compliance_page.title')}</Header>}>
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
          description={t('pages.compliance_page.description')}
        >
          {t('pages.compliance_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h3">{t('pages.compliance_page.frameworks_header')}</Header>}>
          <Box color="text-body-secondary">
            {t('pages.compliance_page.frameworks_desc')}
          </Box>
        </Container>

        <Table
          columnDefinitions={[
            {
              header: t('pages.compliance_page.framework'),
              cell: (item: ComplianceReport) => item.framework,
              width: '25%',
            },
            {
              header: t('pages.compliance_page.status'),
              cell: (item: ComplianceReport) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: t('pages.compliance_page.findings'),
              cell: (item: ComplianceReport) => item.findings,
              width: '15%',
            },
            {
              header: t('pages.compliance_page.last_audit'),
              cell: (item: ComplianceReport) => new Date(item.last_audit).toLocaleString(),
              width: '40%',
            },
          ]}
          items={reports}
          header={<Header variant="h2" counter={`(${reports.length})`}>{t('pages.compliance_page.reports')}</Header>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
