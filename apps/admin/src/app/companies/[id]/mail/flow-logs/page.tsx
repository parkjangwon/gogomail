'use client';

import {
  ContentLayout,
  Header,
  Table,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Badge,
  Button,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface MailLog {
  id: string;
  from: string;
  to: string;
  subject: string;
  status: string;
  timestamp: string;
  message_size: number;
}

export default function MailFlowLogsPage() {
  const { t } = useI18n();
  const [logs, setLogs] = useState<MailLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchMailLogs();
  }, []);

  const fetchMailLogs = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/mail-flow-logs?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setLogs(data.logs || []);
      }
    } catch (error) {
      console.error('Failed to fetch mail logs:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'delivered': return 'green';
      case 'failed': return 'red';
      case 'quarantine': return 'severity-high';
      case 'pending': return 'blue';
      default: return 'grey';
    }
  };

  const filteredLogs = logs.filter(l =>
    l.from.toLowerCase().includes(filter.toLowerCase()) ||
    l.to.toLowerCase().includes(filter.toLowerCase()) ||
    l.subject.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.flow_logs.title')}</Header>}>
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
          description="View mail delivery logs and troubleshoot delivery issues"
          actions={
            <Button variant="primary" disabled>
              Export Logs
            </Button>
          }
        >
          Mail Flow Logs
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'From',
              cell: (item: MailLog) => item.from,
              width: '20%',
            },
            {
              header: 'To',
              cell: (item: MailLog) => item.to,
              width: '20%',
            },
            {
              header: t('pages.flow_logs.subject'),
              cell: (item: MailLog) => item.subject,
              width: '25%',
            },
            {
              header: t('pages.flow_logs.status'),
              cell: (item: MailLog) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: 'Size',
              cell: (item: MailLog) => `${(item.message_size / 1024).toFixed(2)} KB`,
              width: '10%',
            },
            {
              header: 'Timestamp',
              cell: (item: MailLog) => new Date(item.timestamp).toLocaleString(),
              width: '13%',
            },
          ]}
          items={filteredLogs}
          header={<Header variant="h2" counter={`(${filteredLogs.length})`}>Logs</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
