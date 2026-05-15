'use client';
import { DataTable } from '@/components/DataTable';

import {
  ContentLayout,
  Header,
  SpaceBetween,
  Box,
  Spinner,
  Container,
  ColumnLayout,
  FormField,
  Input,
  Select,
  type SelectProps,
  Button,
} from '@cloudscape-design/components';
import { useMemo, useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import {
  buildAPIUsageQuery,
  exportAPIUsageCsv,
  summarizeAPIUsage,
  type APIUsageDailyRow,
} from '@/lib/apiUsage';

interface APIUsageRecord extends APIUsageDailyRow {}

const METHOD_OPTIONS: SelectProps.Option[] = [
  { label: 'All', value: '' },
  { label: 'GET', value: 'GET' },
  { label: 'POST', value: 'POST' },
  { label: 'PUT', value: 'PUT' },
  { label: 'PATCH', value: 'PATCH' },
  { label: 'DELETE', value: 'DELETE' },
];

const AUTH_SOURCE_OPTIONS: SelectProps.Option[] = [
  { label: 'All', value: '' },
  { label: 'anonymous', value: 'anonymous' },
  { label: 'unknown', value: 'unknown' },
  { label: 'bearer', value: 'bearer' },
  { label: 'admin_token', value: 'admin_token' },
  { label: 'query_user_id', value: 'query_user_id' },
];

function optionValue(option: SelectProps.Option): string {
  return String(option.value ?? '');
}

export default function APIUsagePage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const [records, setRecords] = useState<APIUsageRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [domainId, setDomainId] = useState('');
  const [userId, setUserId] = useState('');
  const [principalId, setPrincipalId] = useState('');
  const [route, setRoute] = useState('');
  const [status, setStatus] = useState('');
  const [fromDate, setFromDate] = useState('');
  const [toDate, setToDate] = useState('');
  const [method, setMethod] = useState<SelectProps.Option>(METHOD_OPTIONS[0]);
  const [authSource, setAuthSource] = useState<SelectProps.Option>(AUTH_SOURCE_OPTIONS[0]);

  const fetchAPIUsage = useCallback(async () => {
    if (!companyId) return;
    setLoading(true);
    try {
      const query = buildAPIUsageQuery({
        companyId,
        domainId,
        userId,
        principalId,
        route,
        status: status ? Number(status) : undefined,
        from: fromDate,
        to: toDate,
        method: optionValue(method),
        authSource: optionValue(authSource),
        limit: 100,
      });
      const res = await fetch(`/api/admin/api-usage/daily${query ? `?${query}` : ''}`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setRecords(data.api_usage_daily || []);
      }
    } catch (error) {
      console.error('Failed to fetch API usage:', error);
    } finally {
      setLoading(false);
    }
  }, [authSource, companyId, domainId, fromDate, method, principalId, route, status, toDate, userId]);

  useEffect(() => {
    fetchAPIUsage();
  }, [fetchAPIUsage]);

  const summary = useMemo(() => summarizeAPIUsage(records), [records]);

  const handleExport = () => {
    if (records.length === 0) return;
    const csv = exportAPIUsageCsv(records);
    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `api-usage-${companyId}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.api_usage_analytics.title')}</Header>}>
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
          description={t('pages.api_usage_analytics.description')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="normal" onClick={() => fetchAPIUsage()}>{t('common.refresh')}</Button>
              <Button variant="primary" onClick={handleExport} disabled={records.length === 0}>{t('pages.api_usage_analytics.export_csv')}</Button>
            </SpaceBetween>
          }
        >
          {t('pages.api_usage_analytics.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <ColumnLayout columns={3} variant="text-grid" minColumnWidth={200}>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{summary.request_count.toLocaleString()}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.api_usage_analytics.api_request_count')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold" color={summary.error_rate > 0 ? 'text-status-warning' : 'text-status-success'}>
                {summary.error_rate}%
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.api_usage_analytics.api_error_rate')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">
                {summary.average_latency_ms > 0 ? `${summary.average_latency_ms} ms` : '—'}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.api_usage_analytics.average_latency')}</Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        <Container header={<Header variant="h2">{t('pages.api_usage_analytics.filters')}</Header>}>
          <SpaceBetween size="m">
            <ColumnLayout columns={3} variant="text-grid" minColumnWidth={220}>
              <FormField label={t('pages.api_usage_analytics.domain_id')}>
                <Input value={domainId} onChange={(e) => setDomainId(e.detail.value)} placeholder={t('pages.api_usage_analytics.domain_placeholder')} />
              </FormField>
              <FormField label={t('pages.api_usage_analytics.principal')}>
                <Input value={principalId} onChange={(e) => setPrincipalId(e.detail.value)} placeholder={t('pages.api_usage_analytics.principal_placeholder')} />
              </FormField>
              <FormField label={t('pages.api_usage_analytics.user_id')}>
                <Input value={userId} onChange={(e) => setUserId(e.detail.value)} placeholder={t('pages.api_usage_analytics.user_placeholder')} />
              </FormField>
              <FormField label={t('pages.api_usage_analytics.route')}>
                <Input value={route} onChange={(e) => setRoute(e.detail.value)} placeholder={t('pages.api_usage_analytics.route_placeholder')} />
              </FormField>
              <FormField label={t('pages.api_usage_analytics.status')}>
                <Input value={status} onChange={(e) => setStatus(e.detail.value)} placeholder="200" inputMode="numeric" />
              </FormField>
              <FormField label={t('pages.api_usage_analytics.method')}>
                <Select selectedOption={method} onChange={(e) => setMethod(e.detail.selectedOption)} options={METHOD_OPTIONS} />
              </FormField>
              <FormField label={t('pages.api_usage_analytics.auth_source')}>
                <Select selectedOption={authSource} onChange={(e) => setAuthSource(e.detail.selectedOption)} options={AUTH_SOURCE_OPTIONS} />
              </FormField>
              <FormField label={t('pages.api_usage_analytics.from_date')}>
                <Input value={fromDate} onChange={(e) => setFromDate(e.detail.value)} placeholder="2026-05-01T00:00:00Z" />
              </FormField>
              <FormField label={t('pages.api_usage_analytics.to_date')}>
                <Input value={toDate} onChange={(e) => setToDate(e.detail.value)} placeholder="2026-05-31T23:59:59Z" />
              </FormField>
            </ColumnLayout>
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="primary" onClick={fetchAPIUsage}>{t('pages.api_usage_analytics.apply_filters')}</Button>
              <Button
                variant="normal"
                onClick={() => {
                  setDomainId('');
                  setUserId('');
                  setPrincipalId('');
                  setRoute('');
                  setStatus('');
                  setFromDate('');
                  setToDate('');
                  setMethod(METHOD_OPTIONS[0]);
                  setAuthSource(AUTH_SOURCE_OPTIONS[0]);
                }}
              >
                {t('pages.api_usage_analytics.clear_filters')}
              </Button>
            </SpaceBetween>
          </SpaceBetween>
        </Container>

        <DataTable
          columnDefinitions={[
            {
              header: t('pages.api_usage_analytics.principal'),
              cell: (item: APIUsageRecord) => item.principal_id || '—',
              width: '16%',
            },
            {
              header: t('pages.api_usage_analytics.domain_id'),
              cell: (item: APIUsageRecord) => item.domain_id || '—',
              width: '16%',
            },
            {
              header: t('pages.api_usage_analytics.route'),
              cell: (item: APIUsageRecord) => item.route,
              width: '26%',
            },
            {
              header: t('pages.api_usage_analytics.method'),
              cell: (item: APIUsageRecord) => item.method,
              width: '10%',
            },
            {
              header: t('pages.api_usage_analytics.status'),
              cell: (item: APIUsageRecord) => String(item.status),
              width: '8%',
            },
            {
              header: t('pages.api_usage_analytics.requests'),
              cell: (item: APIUsageRecord) => item.request_count.toLocaleString(),
              width: '10%',
            },
            {
              header: t('pages.api_usage_analytics.average_latency'),
              cell: (item: APIUsageRecord) => `${item.latency_ms_average} ms`,
              width: '10%',
            },
          ]}
          items={records}
          header={<Header variant="h2" counter={`(${records.length})`}>{t('pages.api_usage_analytics.records')}</Header>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
