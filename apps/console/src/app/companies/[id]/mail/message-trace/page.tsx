'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Badge,
  Form,
  FormField,
  Input,
  Select,
  SelectProps,
  Container,
  ExpandableSection,
} from '@cloudscape-design/components';
import { useState } from 'react';
import { useParams } from 'next/navigation';

interface LogEntry {
  id: string;
  message_id: string;
  rfc_message_id: string;
  from_addr: string;
  to_addr: string;
  subject: string;
  flow_status: string;
  direction: string;
  created_at: string;
}

const STATUS_OPTIONS: SelectProps.Option[] = [
  { label: 'All', value: '' },
  { label: 'Delivered', value: 'delivered' },
  { label: 'Queued', value: 'queued' },
  { label: 'Bounced', value: 'bounced' },
  { label: 'Rejected', value: 'rejected' },
  { label: 'Spam', value: 'spam' },
];

const DIR_OPTIONS: SelectProps.Option[] = [
  { label: 'All directions', value: '' },
  { label: 'Inbound', value: 'inbound' },
  { label: 'Outbound', value: 'outbound' },
];

const statusColor = (s: string) => {
  switch (s) {
    case 'delivered': return 'green';
    case 'queued': return 'blue';
    case 'bounced': return 'red';
    case 'rejected': return 'red';
    case 'spam': return 'severity-high';
    default: return 'grey';
  }
};

export default function MessageTracePage() {
  const params = useParams();
  const companyId = params?.id as string;

  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);

  const [fromAddr, setFromAddr] = useState('');
  const [toAddr, setToAddr] = useState('');
  const [subject, setSubject] = useState('');
  const [rfcMsgId, setRfcMsgId] = useState('');
  const [status, setStatus] = useState<SelectProps.Option>(STATUS_OPTIONS[0]);
  const [direction, setDirection] = useState<SelectProps.Option>(DIR_OPTIONS[0]);
  const [since, setSince] = useState('');
  const [until, setUntil] = useState('');

  const [selected, setSelected] = useState<LogEntry[]>([]);

  const handleSearch = async () => {
    setLoading(true);
    setSearched(true);
    try {
      const qs = new URLSearchParams({ company_id: companyId, limit: '100' });
      if (fromAddr) qs.set('from_addr', fromAddr);
      if (toAddr) qs.set('to_addr', toAddr);
      if (subject) qs.set('subject', subject);
      if (rfcMsgId) qs.set('rfc_message_id', rfcMsgId);
      if (status.value) qs.set('flow_status', status.value as string);
      if (direction.value) qs.set('direction', direction.value as string);
      if (since) qs.set('since', since);
      if (until) qs.set('until', until);

      const res = await fetch(`/api/admin/mail-flow-logs?${qs}`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setLogs(data.mail_flow_logs || []);
      }
    } catch (e) {
      console.error('Message trace failed:', e);
    } finally {
      setLoading(false);
    }
  };

  return (
    <ContentLayout
      header={
        <Header variant="h1" description="Search and trace individual email messages across the system.">
          Message Trace
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h2">Search Filters</Header>}>
          <Form
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => { setFromAddr(''); setToAddr(''); setSubject(''); setRfcMsgId(''); setSince(''); setUntil(''); setStatus(STATUS_OPTIONS[0]); setDirection(DIR_OPTIONS[0]); }}>
                  Clear
                </Button>
                <Button variant="primary" onClick={handleSearch} loading={loading}>
                  Search
                </Button>
              </SpaceBetween>
            }
          >
            <SpaceBetween size="m">
              <ColumnLayout columns={2}>
                <FormField label="From">
                  <Input value={fromAddr} onChange={e => setFromAddr(e.detail.value)} placeholder="sender@example.com" />
                </FormField>
                <FormField label="To">
                  <Input value={toAddr} onChange={e => setToAddr(e.detail.value)} placeholder="recipient@example.com" />
                </FormField>
                <FormField label="Subject">
                  <Input value={subject} onChange={e => setSubject(e.detail.value)} placeholder="Partial subject match" />
                </FormField>
                <FormField label="Message ID">
                  <Input value={rfcMsgId} onChange={e => setRfcMsgId(e.detail.value)} placeholder="<msg-id@domain>" />
                </FormField>
                <FormField label="Status">
                  <Select
                    selectedOption={status}
                    options={STATUS_OPTIONS}
                    onChange={e => setStatus(e.detail.selectedOption)}
                  />
                </FormField>
                <FormField label="Direction">
                  <Select
                    selectedOption={direction}
                    options={DIR_OPTIONS}
                    onChange={e => setDirection(e.detail.selectedOption)}
                  />
                </FormField>
                <FormField label="Since (ISO 8601)">
                  <Input value={since} onChange={e => setSince(e.detail.value)} placeholder="2024-01-01T00:00:00Z" />
                </FormField>
                <FormField label="Until (ISO 8601)">
                  <Input value={until} onChange={e => setUntil(e.detail.value)} placeholder="2024-12-31T23:59:59Z" />
                </FormField>
              </ColumnLayout>
            </SpaceBetween>
          </Form>
        </Container>

        {searched && (
          <Table
            columnDefinitions={[
              {
                header: 'From / To',
                cell: (item: LogEntry) => (
                  <SpaceBetween size="xxxs">
                    <Box fontWeight="bold" fontSize="body-s">{item.from_addr}</Box>
                    <Box color="text-body-secondary" fontSize="body-s">→ {item.to_addr}</Box>
                  </SpaceBetween>
                ),
                width: '25%',
              },
              {
                header: 'Subject',
                cell: (item: LogEntry) => (
                  <Box fontSize="body-s" color={item.subject ? undefined : 'text-body-secondary'}>
                    {item.subject || '(no subject)'}
                  </Box>
                ),
                width: '25%',
              },
              {
                header: 'Status',
                cell: (item: LogEntry) => (
                  <Badge color={statusColor(item.flow_status)}>{item.flow_status}</Badge>
                ),
                width: '12%',
              },
              {
                header: 'Direction',
                cell: (item: LogEntry) => (
                  <Badge color={item.direction === 'inbound' ? 'blue' : 'grey'}>{item.direction}</Badge>
                ),
                width: '12%',
              },
              {
                header: 'Time',
                cell: (item: LogEntry) => (
                  <Box fontSize="body-s" color="text-body-secondary">
                    {item.created_at ? new Date(item.created_at).toLocaleString() : '—'}
                  </Box>
                ),
                width: '16%',
              },
              {
                header: 'Message ID',
                cell: (item: LogEntry) => (
                  <Box fontSize="body-s" color="text-body-secondary">
                    {item.rfc_message_id || item.message_id || '—'}
                  </Box>
                ),
                width: '10%',
              },
            ]}
            items={loading ? [] : logs}
            loading={loading}
            loadingText="Searching..."
            selectionType="single"
            selectedItems={selected}
            onSelectionChange={e => setSelected(e.detail.selectedItems)}
            header={
              <Header variant="h2" counter={`(${logs.length})`}>
                Results
              </Header>
            }
            empty={
              <Box textAlign="center" padding="l" color="text-body-secondary">
                {searched ? 'No messages found for these filters.' : 'Enter filters and click Search.'}
              </Box>
            }
          />
        )}

        {selected.length > 0 && (
          <ExpandableSection headerText={`Detail: ${selected[0].rfc_message_id || selected[0].id}`} defaultExpanded>
            <SpaceBetween size="s">
              {([
                ['Internal ID', selected[0].id],
                ['RFC Message ID', selected[0].rfc_message_id],
                ['From', selected[0].from_addr],
                ['To', selected[0].to_addr],
                ['Subject', selected[0].subject],
                ['Status', selected[0].flow_status],
                ['Direction', selected[0].direction],
                ['Received', selected[0].created_at ? new Date(selected[0].created_at).toISOString() : '—'],
              ] as [string, string][]).map(([label, value]) => (
                <ColumnLayout key={label} columns={2} variant="text-grid">
                  <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{label}</Box>
                  <Box fontSize="body-s">{value || '—'}</Box>
                </ColumnLayout>
              ))}
            </SpaceBetween>
          </ExpandableSection>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}

function ColumnLayout({ columns, children }: { columns: number; children: React.ReactNode; variant?: string; minColumnWidth?: number }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: `repeat(${columns}, 1fr)`, gap: '16px' }}>
      {children}
    </div>
  );
}
