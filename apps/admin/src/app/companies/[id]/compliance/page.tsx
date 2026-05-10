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

interface ComplianceReport {
  id: string;
  framework: string;
  status: string;
  last_audit: string;
  findings: number;
}

export default function CompliancePage() {
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
      <ContentLayout header={<Header variant="h1">Compliance</Header>}>
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
          description="Monitor compliance status and audit reports"
        >
          Compliance
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h3">Compliance Frameworks</Header>}>
          <Box color="text-body-secondary">
            Track compliance status for industry standards and regulations
          </Box>
        </Container>

        <Table
          columnDefinitions={[
            {
              header: 'Framework',
              cell: (item: ComplianceReport) => item.framework,
              width: '25%',
            },
            {
              header: 'Status',
              cell: (item: ComplianceReport) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: 'Findings',
              cell: (item: ComplianceReport) => item.findings,
              width: '15%',
            },
            {
              header: 'Last Audit',
              cell: (item: ComplianceReport) => new Date(item.last_audit).toLocaleString(),
              width: '40%',
            },
          ]}
          items={reports}
          header={<Header variant="h2" counter={`(${reports.length})`}>Reports</Header>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
