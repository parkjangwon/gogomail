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

interface Report {
  id: string;
  name: string;
  type: string;
  generated_at: string;
  file_size: number;
}

export default function ReportsPage() {
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
      <ContentLayout header={<Header variant="h1">Reports</Header>}>
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
          description="Generate and view system reports"
          actions={
            <Button variant="primary" disabled>
              Generate Report
            </Button>
          }
        >
          Reports
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h3">Available Reports</Header>}>
          <Box color="text-body-secondary">
            Generate custom reports for system analytics, usage statistics, and compliance
          </Box>
        </Container>

        <Table
          columnDefinitions={[
            {
              header: 'Name',
              cell: (item: Report) => item.name,
              width: '30%',
            },
            {
              header: 'Type',
              cell: (item: Report) => item.type,
              width: '25%',
            },
            {
              header: 'Size',
              cell: (item: Report) => `${(item.file_size / 1024 / 1024).toFixed(2)} MB`,
              width: '15%',
            },
            {
              header: 'Generated',
              cell: (item: Report) => new Date(item.generated_at).toLocaleString(),
              width: '30%',
            },
          ]}
          items={reports}
          header={<Header variant="h2" counter={`(${reports.length})`}>Reports</Header>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
