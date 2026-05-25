'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import {
  Badge,
  Box,
  Button,
  ColumnLayout,
  Container,
  ContentLayout,
  FormField,
  Header,
  Input,
  Select,
  type SelectProps,
  SpaceBetween,
  Spinner,
} from '@cloudscape-design/components';

import { DataTable } from '@/components/DataTable';
import { useI18n } from '@/app/i18n-provider';
import { buildMailFlowLogsQuery, exportMailFlowLogsCsv, type MailFlowLogRow } from '@/lib/mailFlowLogs';

interface MailLog extends MailFlowLogRow {
  from_addr?: string;
  to_addr?: string;
  domain_id?: string;
  flow_status?: string;
  received_at?: string;
  processed_at?: string;
}

const statusColor = (status: string): 'green' | 'red' | 'severity-high' | 'blue' | 'grey' => {
  switch (status) {
    case 'delivered':
      return 'green';
    case 'failed':
    case 'bounced':
    case 'rejected':
      return 'red';
    case 'filtered':
      return 'severity-high';
    case 'pending':
      return 'blue';
    default:
      return 'grey';
  }
};

export default function MailFlowLogsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const statusOptions: SelectProps.Option[] = useMemo(
    () => [
      { label: t('common.all'), value: '' },
      { label: t('pages.flow_logs_page.status_received'), value: 'received' },
      { label: t('pages.flow_logs_page.status_delivered'), value: 'delivered' },
      { label: t('pages.flow_logs_page.status_failed'), value: 'failed' },
      { label: t('pages.flow_logs_page.status_bounced'), value: 'bounced' },
      { label: t('pages.flow_logs_page.status_filtered'), value: 'filtered' },
      { label: t('pages.flow_logs_page.status_rejected'), value: 'rejected' },
      { label: t('pages.flow_logs_page.status_pending'), value: 'pending' },
    ],
    [t]
  );
  const directionOptions: SelectProps.Option[] = useMemo(
    () => [
      { label: t('common.all'), value: '' },
      { label: t('pages.flow_logs_page.direction_inbound'), value: 'inbound' },
      { label: t('pages.flow_logs_page.direction_outbound'), value: 'outbound' },
    ],
    [t]
  );
  const statusLabel = useCallback(
    (value: string) => {
      switch (value) {
        case 'received':
          return t('pages.flow_logs_page.status_received');
        case 'delivered':
          return t('pages.flow_logs_page.status_delivered');
        case 'failed':
          return t('pages.flow_logs_page.status_failed');
        case 'bounced':
          return t('pages.flow_logs_page.status_bounced');
        case 'filtered':
          return t('pages.flow_logs_page.status_filtered');
        case 'rejected':
          return t('pages.flow_logs_page.status_rejected');
        case 'pending':
          return t('pages.flow_logs_page.status_pending');
        default:
          return t('pages.flow_logs_page.status_unknown');
      }
    },
    [t]
  );

  const [logs, setLogs] = useState<MailLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingError, setLoadingError] = useState(false);

  const [searchText, setSearchText] = useState('');
  const [domainId, setDomainId] = useState('');
  const [userId, setUserId] = useState('');
  const [status, setStatus] = useState<SelectProps.Option>({ label: t('common.all'), value: '' });
  const [direction, setDirection] = useState<SelectProps.Option>({ label: t('common.all'), value: '' });
  const [since, setSince] = useState('');
  const [until, setUntil] = useState('');
  const [activeFilters, setActiveFilters] = useState(0);

  const requestFilters = useMemo(
    () => ({
      companyId,
      domainId,
      userId,
      status: (status.value as string) || '',
      direction: (direction.value as string) || '',
      search: searchText,
      since,
      until,
      limit: 100,
    }),
    [companyId, direction.value, domainId, searchText, since, status.value, until, userId]
  );

  const loadMailLogs = useCallback(
    async (override?: Partial<typeof requestFilters>) => {
      setLoading(true);
      setLoadingError(false);
      try {
        const query = buildMailFlowLogsQuery({ ...requestFilters, ...override });
        const suffix = query ? `?${query}` : '';
        const res = await fetch(`/api/admin/mail-flow-logs${suffix}`, {
          credentials: 'include',
        });
        if (!res.ok) {
          throw new Error(`Unexpected response ${res.status}`);
        }
        const data = await res.json();
        setLogs(data.logs || data.mail_flow_logs || []);
      } catch {
        setLogs([]);
        setLoadingError(true);
      } finally {
        setLoading(false);
      }
    },
    [requestFilters]
  );

  useEffect(() => {
    void loadMailLogs();
  }, [loadMailLogs]);

  const currentFilterCount = useMemo(() => {
    let count = 0;
    if (searchText.trim()) count += 1;
    if (domainId.trim()) count += 1;
    if (userId.trim()) count += 1;
    if (status.value) count += 1;
    if (direction.value) count += 1;
    if (since.trim()) count += 1;
    if (until.trim()) count += 1;
    return count;
  }, [direction.value, domainId, searchText, since, status.value, until, userId]);

  useEffect(() => {
    setActiveFilters(currentFilterCount);
  }, [currentFilterCount]);

  const exportLogs = () => {
    const csv = exportMailFlowLogsCsv(logs);
    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'mail-flow-logs.csv';
    anchor.click();
    URL.revokeObjectURL(url);
  };

  const clearFilters = () => {
    setSearchText('');
    setDomainId('');
    setUserId('');
    setStatus(statusOptions[0]);
    setDirection(directionOptions[0]);
    setSince('');
    setUntil('');
    void loadMailLogs({
      domainId: '',
      userId: '',
      status: '',
      direction: '',
      search: '',
      since: '',
      until: '',
      limit: 100,
    });
  };

  const hasRows = logs.length > 0;

  if (loading && !hasRows) {
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
          description={t('pages.flow_logs_page.description')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={clearFilters} disabled={loading && !hasRows}>
                {t('common.clear')}
              </Button>
              <Button onClick={() => void loadMailLogs()} disabled={loading}>
                {t('common.refresh')}
              </Button>
              <Button
                variant="primary"
                onClick={exportLogs}
                disabled={loading || !hasRows}
              >
                {t('pages.flow_logs_page.export_logs')}
              </Button>
            </SpaceBetween>
          }
        >
          {t('pages.flow_logs.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container
          header={
            <Header
              variant="h2"
              counter={`(${activeFilters})`}
              description={t('pages.flow_logs_page.description')}
            >
              {t('pages.flow_logs_page.filters')}
            </Header>
          }
        >
          <SpaceBetween size="m">
            <ColumnLayout columns={2}>
              <FormField label={t('common.search')}>
                <Input
                  value={searchText}
                  onChange={(event) => setSearchText(event.detail.value)}
                  placeholder={t('pages.flow_logs_page.search_placeholder')}
                />
              </FormField>
              <FormField label={t('pages.flow_logs_page.domain_id')}>
                <Input
                  value={domainId}
                  onChange={(event) => setDomainId(event.detail.value)}
                  placeholder={t('pages.flow_logs_page.domain_placeholder')}
                />
              </FormField>
              <FormField label={t('pages.flow_logs_page.user_id')}>
                <Input
                  value={userId}
                  onChange={(event) => setUserId(event.detail.value)}
                  placeholder={t('pages.flow_logs_page.user_placeholder')}
                />
              </FormField>
              <FormField label={t('pages.flow_logs.status')}>
                <Select
                  selectedOption={status}
                  options={statusOptions}
                  onChange={(event) => setStatus(event.detail.selectedOption)}
                />
              </FormField>
              <FormField label={t('pages.flow_logs_page.direction')}>
                <Select
                  selectedOption={direction}
                  options={directionOptions}
                  onChange={(event) => setDirection(event.detail.selectedOption)}
                />
              </FormField>
              <FormField label={t('pages.flow_logs_page.since')}>
                <Input
                  value={since}
                  onChange={(event) => setSince(event.detail.value)}
                  placeholder="2026-05-01T00:00:00Z"
                />
              </FormField>
              <FormField label={t('pages.flow_logs_page.until')}>
                <Input
                  value={until}
                  onChange={(event) => setUntil(event.detail.value)}
                  placeholder="2026-05-31T23:59:59Z"
                />
              </FormField>
            </ColumnLayout>
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="primary" onClick={() => void loadMailLogs()}>
                {t('pages.flow_logs_page.apply_filters')}
              </Button>
              <Button onClick={clearFilters}>
                {t('pages.flow_logs_page.clear_filters')}
              </Button>
            </SpaceBetween>
          </SpaceBetween>
        </Container>

        {loadingError ? (
          <Box textAlign="center" padding="l" color="text-status-error">
            {t('pages.flow_logs_page.load_error')}
          </Box>
        ) : (
          <DataTable
            columnDefinitions={[
              {
                header: t('pages.flow_logs_page.from'),
                cell: (item: MailLog) => item.from || item.from_addr || '—',
                width: '20%',
              },
              {
                header: t('pages.flow_logs_page.to'),
                cell: (item: MailLog) => item.to || item.to_addr || '—',
                width: '20%',
              },
              {
                header: t('pages.flow_logs.subject'),
                cell: (item: MailLog) => item.subject || '—',
                width: '26%',
              },
              {
                header: t('pages.flow_logs.status'),
                cell: (item: MailLog) => (
                  <Badge color={statusColor(item.status || item.flow_status || '')}>
                    {statusLabel(item.status || item.flow_status || 'unknown')}
                  </Badge>
                ),
                width: '10%',
              },
              {
                header: t('pages.flow_logs_page.size'),
                cell: (item: MailLog) => `${((item.message_size || 0) / 1024).toFixed(2)} KB`,
                width: '10%',
              },
              {
                header: t('pages.flow_logs_page.timestamp'),
                cell: (item: MailLog) =>
                  new Date(item.created_at || item.received_at || item.processed_at || item.timestamp || '').toLocaleString(),
                width: '14%',
              },
            ]}
            items={loading ? [] : logs}
            loading={loading}
            loadingText={t('pages.flow_logs_page.loading')}
            header={
              <Header variant="h2" counter={`(${logs.length})`}>
                {t('pages.flow_logs_page.logs')}
              </Header>
            }
            empty={
              <Box textAlign="center" padding="l" color="text-body-secondary">
                {loadingError
                  ? t('pages.flow_logs_page.load_error')
                  : t('pages.flow_logs_page.no_logs')}
              </Box>
            }
          />
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
