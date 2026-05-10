'use client';

import {
  ContentLayout,
  Header,
  Table,
  Box,
  Spinner,
  ButtonDropdown,
  TextFilter,
  SpaceBetween,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

interface AuditLog {
  id: string;
  admin_user_id: string;
  action: string;
  resource_type: string;
  timestamp: string;
  ip_address: string;
}

export default function AuditLogsPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filterText, setFilterText] = useState('');

  useEffect(() => {
    fetchLogs();
  }, []);

  const fetchLogs = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/audit-logs?limit=50', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setLogs(data.audit_logs || []);
      }
    } catch (error) {
      console.error('Failed to fetch logs:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleExport = async (format: 'csv' | 'json') => {
    const data = format === 'csv'
      ? logs.map(l => `${l.timestamp},${l.action},${l.resource_type},${l.ip_address}`).join('\n')
      : JSON.stringify(logs, null, 2);

    const blob = new Blob([data], { type: format === 'csv' ? 'text/csv' : 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `audit-logs.${format}`;
    a.click();
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Audit Logs</Header>}>
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
          actions={
            <ButtonDropdown
              items={[
                { text: 'Export as CSV', id: 'csv' },
                { text: 'Export as JSON', id: 'json' },
              ]}
              onItemClick={(e) => handleExport(e.detail.id as 'csv' | 'json')}
            >
              Export
            </ButtonDropdown>
          }
        >
          Audit Logs
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Time',
              cell: (log: AuditLog) => new Date(log.timestamp).toLocaleString(),
              width: '25%',
            },
            {
              header: 'Action',
              cell: (log: AuditLog) => log.action,
              width: '20%',
            },
            {
              header: 'Resource',
              cell: (log: AuditLog) => log.resource_type,
              width: '20%',
            },
            {
              header: 'Admin',
              cell: (log: AuditLog) => log.admin_user_id,
              width: '15%',
            },
            {
              header: 'IP Address',
              cell: (log: AuditLog) => log.ip_address || '-',
              width: '20%',
            },
          ]}
          items={logs}
          header={<Header variant="h2" counter={`(${logs.length})`}>Activity Log</Header>}
          filter={
            <TextFilter
              filteringText={filterText}
              onChange={(e) => setFilterText(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
