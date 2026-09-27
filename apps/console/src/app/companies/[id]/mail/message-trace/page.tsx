'use client';
import { DataTable } from '@/components/DataTable';

import {
  ContentLayout,
  Header,
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
  ColumnLayout,
} from '@cloudscape-design/components';
import { useEffect, useRef, useState } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import {
  buildMailFlowLogsQuery,
  exportMailFlowLogsCsv,
  downloadCsv,
  isValidDateTimeInput,
  toRFC3339,
} from '@/lib/mailFlowLogs';

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

const statusColor = (s: string): 'green' | 'red' | 'severity-high' | 'blue' | 'grey' => {
  switch (s) {
    case 'received': return 'blue';
    case 'delivered': return 'green';
    case 'failed': return 'red';
    case 'bounced': return 'red';
    case 'rejected': return 'red';
    case 'filtered': return 'severity-high';
    case 'pending': return 'blue';
    default: return 'grey';
  }
};

export default function MessageTracePage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const statusLabel = (value: string): string => {
    switch (value) {
      case 'received': return t('pages.flow_logs_page.status_received');
      case 'delivered': return t('pages.flow_logs_page.status_delivered');
      case 'failed': return t('pages.flow_logs_page.status_failed');
      case 'bounced': return t('pages.flow_logs_page.status_bounced');
      case 'filtered': return t('pages.flow_logs_page.status_filtered');
      case 'rejected': return t('pages.flow_logs_page.status_rejected');
      case 'pending': return t('pages.flow_logs_page.status_pending');
      default: return value || t('pages.flow_logs_page.status_unknown');
    }
  };

  const directionLabel = (value: string): string => {
    switch (value) {
      case 'inbound': return t('pages.flow_logs.direction_inbound');
      case 'outbound': return t('pages.flow_logs.direction_outbound');
      default: return value || '—';
    }
  };

  const STATUS_OPTIONS: SelectProps.Option[] = [
    { label: t('common.all'), value: '' },
    { label: t('pages.flow_logs_page.status_received'), value: 'received' },
    { label: t('pages.flow_logs_page.status_delivered'), value: 'delivered' },
    { label: t('pages.flow_logs_page.status_failed'), value: 'failed' },
    { label: t('pages.flow_logs_page.status_bounced'), value: 'bounced' },
    { label: t('pages.flow_logs_page.status_filtered'), value: 'filtered' },
    { label: t('pages.flow_logs_page.status_rejected'), value: 'rejected' },
    { label: t('pages.flow_logs_page.status_pending'), value: 'pending' },
  ];

  const DIR_OPTIONS: SelectProps.Option[] = [
    { label: t('common.all'), value: '' },
    { label: t('pages.flow_logs.direction_inbound'), value: 'inbound' },
    { label: t('pages.flow_logs.direction_outbound'), value: 'outbound' },
  ];

  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const [searchError, setSearchError] = useState(false);

  const [fromAddr, setFromAddr] = useState('');
  const [toAddr, setToAddr] = useState('');
  const [subject, setSubject] = useState('');
  const [rfcMsgId, setRfcMsgId] = useState('');
  const [status, setStatus] = useState<SelectProps.Option>(STATUS_OPTIONS[0]);
  const [direction, setDirection] = useState<SelectProps.Option>(DIR_OPTIONS[0]);
  const [since, setSince] = useState('');
  const [until, setUntil] = useState('');
  const [sinceError, setSinceError] = useState('');
  const [untilError, setUntilError] = useState('');

  const [selected, setSelected] = useState<LogEntry[]>([]);

  const abortRef = useRef<AbortController | null>(null);
  useEffect(() => () => abortRef.current?.abort(), []);

  const handleExportCSV = () => {
    if (logs.length === 0) return;
    const csv = exportMailFlowLogsCsv(
      logs.map((log) => ({
        id: log.id,
        from: log.from_addr,
        to: log.to_addr,
        subject: log.subject,
        status: log.flow_status,
        created_at: log.created_at,
        timestamp: log.created_at,
        message_size: 0,
      }))
    );
    downloadCsv(csv, `message-trace-${new Date().toISOString().slice(0, 10)}.csv`);
  };

  const handleSearch = async () => {
    const sinceValid = isValidDateTimeInput(since);
    const untilValid = isValidDateTimeInput(until);
    setSinceError(sinceValid ? '' : t('pages.message_trace.invalid_datetime', 'Enter a valid date/time.'));
    setUntilError(untilValid ? '' : t('pages.message_trace.invalid_datetime', 'Enter a valid date/time.'));
    if (!sinceValid || !untilValid) {
      return;
    }

    // Clear any stale detail panel from a previous selection before searching.
    setSelected([]);

    // Cancel any in-flight request so a slow older response can't overwrite
    // newer results.
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setSearched(true);
    setSearchError(false);
    try {
      const qs = buildMailFlowLogsQuery({
        companyId,
        userId: '',
        status: (status.value as string) || '',
        direction: (direction.value as string) || '',
        fromAddr,
        toAddr,
        subject,
        rfcMessageId: rfcMsgId,
        since: toRFC3339(since),
        until: toRFC3339(until),
        limit: 100,
      });
      const res = await fetch(`/api/admin/mail-flow-logs${qs ? `?${qs}` : ''}`, {
        credentials: 'include',
        signal: controller.signal,
      });
      if (res.ok) {
        const data = await res.json();
        setLogs(data.mail_flow_logs || []);
      } else {
        setSearchError(true);
      }
    } catch (err: unknown) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        return; // superseded by a newer request.
      }
      setSearchError(true);
    } finally {
      if (abortRef.current === controller) {
        setLoading(false);
      }
    }
  };

  const handleClear = () => {
    setFromAddr('');
    setToAddr('');
    setSubject('');
    setRfcMsgId('');
    setSince('');
    setUntil('');
    setSinceError('');
    setUntilError('');
    setStatus(STATUS_OPTIONS[0]);
    setDirection(DIR_OPTIONS[0]);
    setSelected([]);
  };

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.message_trace.description')}>
          {t('pages.message_trace.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h2">{t('pages.message_trace.search_filters')}</Header>}>
          <Form
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={handleClear}>
                  {t('pages.message_trace.clear')}
                </Button>
                <Button variant="primary" onClick={handleSearch} loading={loading}>
                  {t('pages.message_trace.search')}
                </Button>
              </SpaceBetween>
            }
          >
            <SpaceBetween size="m">
              <ColumnLayout columns={2}>
                <FormField label={t('pages.message_trace.from')}>
                  <Input value={fromAddr} onChange={e => setFromAddr(e.detail.value)} placeholder={t('pages.message_trace.from_placeholder')} />
                </FormField>
                <FormField label={t('pages.message_trace.to')}>
                  <Input value={toAddr} onChange={e => setToAddr(e.detail.value)} placeholder={t('pages.message_trace.to_placeholder')} />
                </FormField>
                <FormField label={t('pages.message_trace.subject')}>
                  <Input value={subject} onChange={e => setSubject(e.detail.value)} placeholder={t('pages.message_trace.subject_placeholder')} />
                </FormField>
                <FormField label={t('pages.message_trace.message_id')}>
                  <Input value={rfcMsgId} onChange={e => setRfcMsgId(e.detail.value)} placeholder="<msg-id@domain>" />
                </FormField>
                <FormField label={t('pages.message_trace.status')}>
                  <Select
                    selectedOption={status}
                    options={STATUS_OPTIONS}
                    onChange={e => setStatus(e.detail.selectedOption)}
                  />
                </FormField>
                <FormField label={t('pages.message_trace.direction')}>
                  <Select
                    selectedOption={direction}
                    options={DIR_OPTIONS}
                    onChange={e => setDirection(e.detail.selectedOption)}
                  />
                </FormField>
                <FormField label={t('pages.message_trace.since')} errorText={sinceError || undefined}>
                  <Input value={since} onChange={e => setSince(e.detail.value)} placeholder="2026-01-01T00:00" />
                </FormField>
                <FormField label={t('pages.message_trace.until')} errorText={untilError || undefined}>
                  <Input value={until} onChange={e => setUntil(e.detail.value)} placeholder="2026-12-31T23:59" />
                </FormField>
              </ColumnLayout>
            </SpaceBetween>
          </Form>
        </Container>

        {searched && (
          <DataTable
            columnDefinitions={[
              {
                header: t('pages.message_trace.from_to'),
                cell: (item: LogEntry) => (
                  <SpaceBetween size="xxxs">
                    <Box fontWeight="bold" fontSize="body-s">{item.from_addr}</Box>
                    <Box color="text-body-secondary" fontSize="body-s">→ {item.to_addr}</Box>
                  </SpaceBetween>
                ),
                width: '25%',
              },
              {
                header: t('pages.message_trace.subject'),
                cell: (item: LogEntry) => (
                  <Box fontSize="body-s" color={item.subject ? undefined : 'text-body-secondary'}>
                    {item.subject || t('pages.message_trace.no_subject')}
                  </Box>
                ),
                width: '25%',
              },
              {
                header: t('pages.message_trace.status'),
                cell: (item: LogEntry) => (
                  <Badge color={statusColor(item.flow_status)}>{statusLabel(item.flow_status)}</Badge>
                ),
                width: '12%',
              },
              {
                header: t('pages.message_trace.direction'),
                cell: (item: LogEntry) => (
                  <Badge color={item.direction === 'inbound' ? 'blue' : 'grey'}>{directionLabel(item.direction)}</Badge>
                ),
                width: '12%',
              },
              {
                header: t('pages.message_trace.time'),
                cell: (item: LogEntry) => (
                  <Box fontSize="body-s" color="text-body-secondary">
                    {item.created_at ? new Date(item.created_at).toLocaleString() : '—'}
                  </Box>
                ),
                width: '16%',
              },
              {
                header: t('pages.message_trace.message_id'),
                cell: (item: LogEntry) => (
                  <Box fontSize="body-s" color="text-body-secondary">
                    {item.rfc_message_id || item.message_id || '—'}
                  </Box>
                ),
                width: '10%',
              },
            ]}
            items={logs}
            loading={loading}
            loadingText={t('pages.message_trace.searching')}
            filter={false}
            selectionType="single"
            selectedItems={selected}
            onSelectionChange={e => setSelected(e.detail.selectedItems)}
            header={
              <Header
                variant="h2"
                counter={`(${logs.length})`}
                actions={
                  logs.length > 0 ? (
                    <Button iconName="download" onClick={handleExportCSV}>
                      {t('pages.message_trace.export_csv')}
                    </Button>
                  ) : undefined
                }
              >
                {t('pages.message_trace.results')}
              </Header>
            }
            empty={
              searchError ? (
                <Box textAlign="center" padding="l">
                  <SpaceBetween size="s" alignItems="center">
                    <Box color="text-status-error">{t('pages.message_trace.search_failed')}</Box>
                    <Button iconName="refresh" onClick={handleSearch} loading={loading}>{t('pages.message_trace.retry')}</Button>
                  </SpaceBetween>
                </Box>
              ) : (
                <Box textAlign="center" padding="l" color="text-body-secondary">
                  {searched ? t('pages.message_trace.no_messages') : t('pages.message_trace.enter_filters')}
                </Box>
              )
            }
          />
        )}

        {selected.length > 0 && (
          <ExpandableSection headerText={`${t('pages.message_trace.detail')}: ${selected[0].rfc_message_id || selected[0].id}`} defaultExpanded>
            <SpaceBetween size="s">
              {([
                [t('pages.message_trace.internal_id'), selected[0].id],
                [t('pages.message_trace.rfc_message_id'), selected[0].rfc_message_id],
                [t('pages.message_trace.from'), selected[0].from_addr],
                [t('pages.message_trace.to'), selected[0].to_addr],
                [t('pages.message_trace.subject'), selected[0].subject],
                [t('pages.message_trace.status'), statusLabel(selected[0].flow_status)],
                [t('pages.message_trace.direction'), directionLabel(selected[0].direction)],
                [t('pages.message_trace.received'), selected[0].created_at ? new Date(selected[0].created_at).toISOString() : '—'],
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
