'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Badge,
  Select,
  SelectProps,
  TextFilter,
  ExpandableSection,
  ColumnLayout,
  Pagination,
} from '@cloudscape-design/components';
import { useState, useEffect, useMemo } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface AuditLog {
  id: string;
  company_id: string;
  domain_id: string;
  user_id: string;
  actor_id: string;
  category: string;
  action: string;
  target_type: string;
  target_id: string;
  ip_address: string;
  result: string;
  detail: Record<string, unknown>;
  created_at: string;
}

const PAGE_SIZE = 25;

export default function AdminActivityPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const categoryOptions: SelectProps.Option[] = [
    { label: t('admin_activity.all_categories'), value: '' },
    { label: t('admin_activity.cat_admin'), value: 'admin' },
    { label: t('admin_activity.cat_auth'), value: 'auth' },
    { label: t('admin_activity.cat_user'), value: 'user' },
    { label: t('admin_activity.cat_domain'), value: 'domain' },
    { label: t('admin_activity.cat_mail'), value: 'mail' },
  ];
  const resultOptions: SelectProps.Option[] = [
    { label: t('admin_activity.all_results'), value: '' },
    { label: t('admin_activity.result_success'), value: 'success' },
    { label: t('admin_activity.result_failure'), value: 'failure' },
  ];

  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState(false);
  const [filter, setFilter] = useState('');
  const [category, setCategory] = useState<SelectProps.Option>({ label: t('admin_activity.all_categories'), value: '' });
  const [result, setResult] = useState<SelectProps.Option>({ label: t('admin_activity.all_results'), value: '' });
  const [currentPage, setCurrentPage] = useState(1);
  const [selected, setSelected] = useState<AuditLog[]>([]);

  const fetchLogs = async () => {
    setLoading(true);
    setFetchError(false);
    try {
      const qs = new URLSearchParams({ limit: '200' });
      if (companyId) qs.set('company_id', companyId);
      if (category.value) qs.set('category', category.value as string);
      if (result.value) qs.set('result', result.value as string);
      const res = await fetch(`/api/admin/audit-logs?${qs}`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setLogs(data.audit_logs || []);
      } else {
        setFetchError(true);
      }
    } catch {
      setFetchError(true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (companyId) fetchLogs();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [companyId, category.value, result.value]);

  const filtered = useMemo(() => {
    if (!filter) return logs;
    const q = filter.toLowerCase();
    return logs.filter(l =>
      l.action.toLowerCase().includes(q) ||
      l.actor_id.toLowerCase().includes(q) ||
      l.target_type.toLowerCase().includes(q) ||
      l.ip_address.toLowerCase().includes(q),
    );
  }, [logs, filter]);

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const paged = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);

  const resultColor = (r: string) => r === 'success' ? 'green' : r === 'failure' ? 'red' : 'grey';

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('admin_activity.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (fetchError) {
    return (
      <ContentLayout header={<Header variant="h1">{t('admin_activity.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <SpaceBetween size="m" alignItems="center">
            <Box color="text-status-error">{t('admin_activity.failed_load')}</Box>
            <Button iconName="refresh" onClick={fetchLogs}>{t('admin_activity.retry')}</Button>
          </SpaceBetween>
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('admin_activity.description')}
          actions={<Button iconName="refresh" onClick={fetchLogs}>{t('admin_activity.refresh')}</Button>}
        >
          {t('admin_activity.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: t('admin_activity.col_time'),
              cell: (item: AuditLog) => (
                <Box fontSize="body-s" color="text-body-secondary">
                  {item.created_at ? new Date(item.created_at).toLocaleString() : '—'}
                </Box>
              ),
              width: '16%',
            },
            {
              header: t('admin_activity.col_category_action'),
              cell: (item: AuditLog) => (
                <SpaceBetween size="xxxs">
                  <Badge color={item.category === 'admin' ? 'blue' : 'grey'}>{item.category}</Badge>
                  <Box fontSize="body-s" fontWeight="bold">{item.action}</Box>
                </SpaceBetween>
              ),
              width: '22%',
            },
            {
              header: t('admin_activity.col_actor'),
              cell: (item: AuditLog) => (
                <Box fontSize="body-s" color="text-body-secondary">
                  {item.actor_id || '—'}
                </Box>
              ),
              width: '20%',
            },
            {
              header: t('admin_activity.col_target'),
              cell: (item: AuditLog) => (
                <SpaceBetween size="xxxs">
                  <Box fontSize="body-s" color="text-body-secondary">{item.target_type || '—'}</Box>
                  {item.target_id && (
                    <Box fontSize="body-s" color="text-body-secondary">{item.target_id.slice(0, 16)}…</Box>
                  )}
                </SpaceBetween>
              ),
              width: '20%',
            },
            {
              header: t('admin_activity.col_ip'),
              cell: (item: AuditLog) => (
                <Box fontSize="body-s" color="text-body-secondary">{item.ip_address || '—'}</Box>
              ),
              width: '12%',
            },
            {
              header: t('admin_activity.col_result'),
              cell: (item: AuditLog) => (
                <Badge color={resultColor(item.result)}>{item.result || '—'}</Badge>
              ),
              width: '10%',
            },
          ]}
          items={paged}
          selectionType="single"
          selectedItems={selected}
          onSelectionChange={e => setSelected(e.detail.selectedItems)}
          header={
            <Header variant="h2" counter={`(${filtered.length})`}>
              {t('admin_activity.events_header')}
            </Header>
          }
          filter={
            <SpaceBetween direction="horizontal" size="xs">
              <TextFilter
                filteringText={filter}
                filteringPlaceholder={t('admin_activity.search_placeholder')}
                onChange={e => { setFilter(e.detail.filteringText); setCurrentPage(1); }}
              />
              <Select
                selectedOption={category}
                options={categoryOptions}
                onChange={e => { setCategory(e.detail.selectedOption); setCurrentPage(1); }}
                expandToViewport
              />
              <Select
                selectedOption={result}
                options={resultOptions}
                onChange={e => { setResult(e.detail.selectedOption); setCurrentPage(1); }}
                expandToViewport
              />
            </SpaceBetween>
          }
          pagination={
            pageCount > 1 ? (
              <Pagination
                currentPageIndex={currentPage}
                pagesCount={pageCount}
                onChange={e => setCurrentPage(e.detail.currentPageIndex)}
              />
            ) : undefined
          }
          empty={
            <Box textAlign="center" padding="l" color="text-body-secondary">
              {t('admin_activity.no_events')}
            </Box>
          }
        />

        {selected.length > 0 && (
          <ExpandableSection headerText={`${t('admin_activity.detail')}: ${selected[0].action} — ${selected[0].id}`} defaultExpanded>
            <SpaceBetween size="s">
              {([
                ['ID', selected[0].id],
                [t('admin_activity.detail_category'), selected[0].category],
                [t('admin_activity.detail_action'), selected[0].action],
                [t('admin_activity.detail_actor'), selected[0].actor_id],
                [t('admin_activity.detail_target_type'), selected[0].target_type],
                [t('admin_activity.detail_target_id'), selected[0].target_id],
                [t('admin_activity.detail_ip_address'), selected[0].ip_address],
                [t('admin_activity.detail_result'), selected[0].result],
                [t('admin_activity.detail_time'), selected[0].created_at ? new Date(selected[0].created_at).toISOString() : '—'],
              ] as [string, string][]).map(([label, value]) => (
                <ColumnLayout key={label} columns={2} variant="text-grid">
                  <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{label}</Box>
                  <Box fontSize="body-s">{value || '—'}</Box>
                </ColumnLayout>
              ))}
              {selected[0].detail && Object.keys(selected[0].detail).length > 0 && (
                <SpaceBetween size="xxs">
                  <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{t('admin_activity.detail')}</Box>
                  <Box fontSize="body-s">
                    <pre style={{ margin: 0, whiteSpace: 'pre-wrap', fontSize: '12px' }}>
                      {JSON.stringify(selected[0].detail, null, 2)}
                    </pre>
                  </Box>
                </SpaceBetween>
              )}
            </SpaceBetween>
          </ExpandableSection>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
