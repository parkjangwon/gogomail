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

interface DeliveryAttempt {
  id: string;
  message_id: string;
  recipient: string;
  attempt_number: number;
  status: string;
  error_message?: string;
  timestamp: string;
}

const STATUS_OPTIONS: SelectProps.Option[] = [
  { label: 'All', value: '' },
  { label: 'Success', value: 'success' },
  { label: 'Failed', value: 'failed' },
  { label: 'Permanent failure', value: 'permanent_failure' },
  { label: 'Temporary failure', value: 'temporary_failure' },
];

export default function DeliveryAttemptsPage() {
  const { t } = useI18n();
  const [attempts, setAttempts] = useState<DeliveryAttempt[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [sinceDate, setSinceDate] = useState('');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);

  const fetchDeliveryAttempts = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit: '100' });
      if (statusFilter) params.set('status', statusFilter);
      if (sinceDate.trim()) params.set('since', sinceDate.trim());

      const res = await fetch(`/api/admin/delivery-attempts?${params.toString()}`, {
        credentials: 'include'
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
        setAttempts([]);
        return;
      }
      const data = await res.json();
      setAttempts(data.attempts || []);
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
      setAttempts([]);
    } finally {
      setLoading(false);
    }
  }, [statusFilter, sinceDate, t]);

  useEffect(() => {
    fetchDeliveryAttempts();
  }, [fetchDeliveryAttempts]);

  const getStatusColor = (status: string): 'green' | 'red' | 'severity-critical' | 'severity-high' | 'grey' => {
    switch (status) {
      case 'success': return 'green';
      case 'failed': return 'red';
      case 'permanent_failure': return 'severity-critical';
      case 'temporary_failure': return 'severity-high';
      default: return 'grey';
    }
  };

  const filteredAttempts = attempts.filter(a =>
    a.recipient.toLowerCase().includes(filter.toLowerCase()) ||
    a.message_id.toLowerCase().includes(filter.toLowerCase())
  );

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
              id: 'message_id',
              header: t('pages.delivery_attempts_page.message_id'),
              cell: (item: DeliveryAttempt) => item.message_id,
              width: '20%',
            },
            {
              id: 'recipient',
              header: t('pages.delivery_attempts.recipient'),
              cell: (item: DeliveryAttempt) => item.recipient,
              width: '25%',
            },
            {
              id: 'attempt',
              header: t('pages.delivery_attempts_page.attempt'),
              cell: (item: DeliveryAttempt) => `#${item.attempt_number}`,
              width: '10%',
            },
            {
              id: 'status',
              header: t('pages.delivery_attempts.status'),
              cell: (item: DeliveryAttempt) => (
                <Badge color={getStatusColor(item.status)}>
                  {item.status}
                </Badge>
              ),
              width: '20%',
            },
            {
              id: 'error',
              header: t('pages.delivery_attempts_page.error'),
              cell: (item: DeliveryAttempt) => item.error_message || '-',
              width: '15%',
            },
            {
              id: 'timestamp',
              header: t('pages.delivery_attempts_page.timestamp'),
              cell: (item: DeliveryAttempt) => new Date(item.timestamp).toLocaleString(),
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
                selectedOption={STATUS_OPTIONS.find(o => o.value === statusFilter) ?? STATUS_OPTIONS[0]}
                options={STATUS_OPTIONS}
                onChange={(e) => setStatusFilter(e.detail.selectedOption.value ?? '')}
              />
              <FormField label={t('pages.delivery_attempts_page.since_date', 'Since')}>
                <Input
                  value={sinceDate}
                  onChange={(e) => setSinceDate(e.detail.value)}
                  placeholder="2026-05-01T00:00:00Z"
                />
              </FormField>
            </SpaceBetween>
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
