'use client';
import { DataTable } from '@/components/DataTable';

import {
  ContentLayout,
  Header,
  SpaceBetween,
  Box,
  TextFilter,
  Badge,
  Flashbar,
  FlashbarProps,
  Button,
  Select,
  SelectProps,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { isValidDateTimeInput, toRFC3339 } from '@/lib/mailFlowLogs';

interface DeliveryAttempt {
  id: string;
  message_id: string;
  rfc_message_id: string;
  farm: string;
  sender?: string;
  recipient: string;
  recipient_domain: string;
  status: string;
  enhanced_status?: string;
  error_message: string;
  attempted_at: string;
}

const PAGE_SIZE = 100;

export default function DeliveryAttemptsPage() {
  const { t } = useI18n();
  const [attempts, setAttempts] = useState<DeliveryAttempt[]>([]);
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(false);
  const [limit, setLimit] = useState(PAGE_SIZE);
  const [filter, setFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [sinceDate, setSinceDate] = useState('');
  const [sinceError, setSinceError] = useState('');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);

  const statusOptions: SelectProps.Option[] = [
    { label: t('common.all', 'All'), value: '' },
    { label: t('pages.delivery_attempts.status_delivered', 'Delivered'), value: 'delivered' },
    { label: t('pages.delivery_attempts.status_failed', 'Failed'), value: 'failed' },
    { label: t('pages.delivery_attempts.status_bounced', 'Bounced'), value: 'bounced' },
    { label: t('pages.delivery_attempts.status_exhausted', 'Exhausted'), value: 'exhausted' },
  ];

  const fetchDeliveryAttempts = useCallback(async () => {
    if (sinceDate.trim() && !isValidDateTimeInput(sinceDate)) {
      setSinceError(t('pages.delivery_attempts.invalid_datetime', 'Enter a valid date/time (e.g. 2026-05-01T00:00).'));
      return;
    }
    setSinceError('');
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit: String(limit) });
      if (statusFilter) params.set('status', statusFilter);
      if (sinceDate.trim()) params.set('since', toRFC3339(sinceDate));

      const res = await fetch(`/api/admin/delivery-attempts?${params.toString()}`, {
        credentials: 'include',
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        const msg = (body as { error?: string }).error ?? `HTTP ${res.status}`;
        setFlash([{
          type: 'error',
          header: t('pages.delivery_attempts.fetch_error_header', 'Failed to load delivery attempts'),
          content: msg,
          dismissible: true,
          onDismiss: () => setFlash([]),
        }]);
        return;
      }
      const data = await res.json();
      setAttempts(data.delivery_attempts || []);
      setHasMore(Boolean(data.has_more));
      setFlash([]);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'An unexpected error occurred.';
      setFlash([{
        type: 'error',
        header: t('pages.delivery_attempts.fetch_error_header', 'Failed to load delivery attempts'),
        content: msg,
        dismissible: true,
        onDismiss: () => setFlash([]),
      }]);
    } finally {
      setLoading(false);
    }
  }, [statusFilter, sinceDate, limit, t]);

  useEffect(() => {
    fetchDeliveryAttempts();
  }, [fetchDeliveryAttempts]);

  const getStatusColor = (status: string): 'green' | 'red' | 'severity-high' | 'grey' => {
    switch (status) {
      case 'delivered': return 'green';
      case 'failed': return 'red';
      case 'bounced': return 'red';
      case 'exhausted': return 'severity-high';
      default: return 'grey';
    }
  };

  const statusLabel = (status: string): string => {
    switch (status) {
      case 'delivered': return t('pages.delivery_attempts.status_delivered', 'Delivered');
      case 'failed': return t('pages.delivery_attempts.status_failed', 'Failed');
      case 'bounced': return t('pages.delivery_attempts.status_bounced', 'Bounced');
      case 'exhausted': return t('pages.delivery_attempts.status_exhausted', 'Exhausted');
      default: return status || '—';
    }
  };

  const needle = filter.trim().toLowerCase();
  const filteredAttempts = needle
    ? attempts.filter(a =>
        a.recipient.toLowerCase().includes(needle) ||
        a.message_id.toLowerCase().includes(needle) ||
        (a.rfc_message_id || '').toLowerCase().includes(needle) ||
        (a.sender || '').toLowerCase().includes(needle)
      )
    : attempts;

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.delivery_attempts_page.description')}
          actions={
            <Button iconName="refresh" onClick={fetchDeliveryAttempts} loading={loading}>
              {t('common.refresh', 'Refresh')}
            </Button>
          }
        >
          {t('pages.delivery_attempts.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {flash.length > 0 && <Flashbar items={flash} />}

        <DataTable
          columnDefinitions={[
            {
              id: 'recipient',
              header: t('pages.delivery_attempts.recipient'),
              cell: (item: DeliveryAttempt) => item.recipient,
              width: '22%',
            },
            {
              id: 'sender',
              header: t('pages.delivery_attempts.sender'),
              cell: (item: DeliveryAttempt) => item.sender || '—',
              width: '20%',
            },
            {
              id: 'status',
              header: t('pages.delivery_attempts.status'),
              cell: (item: DeliveryAttempt) => (
                <Badge color={getStatusColor(item.status)}>
                  {statusLabel(item.status)}
                </Badge>
              ),
              width: '12%',
            },
            {
              id: 'error',
              header: t('pages.delivery_attempts_page.error'),
              cell: (item: DeliveryAttempt) => item.error_message || item.enhanced_status || '-',
              width: '24%',
            },
            {
              id: 'message_id',
              header: t('pages.delivery_attempts_page.message_id'),
              cell: (item: DeliveryAttempt) => item.rfc_message_id || item.message_id || '—',
              width: '12%',
            },
            {
              id: 'attempted_at',
              header: t('pages.delivery_attempts_page.timestamp'),
              cell: (item: DeliveryAttempt) =>
                item.attempted_at ? new Date(item.attempted_at).toLocaleString() : '—',
              width: '10%',
            },
          ]}
          items={filteredAttempts}
          loading={loading}
          header={
            <Header variant="h2" counter={`(${filteredAttempts.length})`}>
              {t('pages.delivery_attempts_page.attempts')}
            </Header>
          }
          filter={
            <SpaceBetween size="xs" direction="horizontal">
              <TextFilter
                filteringText={filter}
                filteringPlaceholder={t('common.search')}
                onChange={(e) => setFilter(e.detail.filteringText)}
              />
              <Select
                selectedOption={statusOptions.find(o => o.value === statusFilter) ?? statusOptions[0]}
                options={statusOptions}
                onChange={(e) => setStatusFilter(e.detail.selectedOption.value ?? '')}
              />
              <FormField
                label={t('pages.delivery_attempts_page.since_date', 'Since')}
                errorText={sinceError || undefined}
              >
                <Input
                  value={sinceDate}
                  onChange={(e) => setSinceDate(e.detail.value)}
                  placeholder="2026-05-01T00:00"
                />
              </FormField>
            </SpaceBetween>
          }
          footer={
            hasMore ? (
              <Box textAlign="center">
                <Button
                  onClick={() => setLimit((prev) => prev + PAGE_SIZE)}
                  loading={loading}
                >
                  {t('common.load_more', 'Load more')}
                </Button>
              </Box>
            ) : undefined
          }
          empty={
            <Box textAlign="center" padding="l" color="inherit">
              {t('pages.delivery_attempts.no_attempts', 'No delivery attempts found.')}
            </Box>
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
