'use client';

import {
  ContentLayout,
  Header,
  Table,
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

const CATEGORY_OPTIONS: SelectProps.Option[] = [
  { label: 'All categories', value: '' },
  { label: 'Admin', value: 'admin' },
  { label: 'Auth', value: 'auth' },
  { label: 'User', value: 'user' },
  { label: 'Domain', value: 'domain' },
  { label: 'Mail', value: 'mail' },
];

const RESULT_OPTIONS: SelectProps.Option[] = [
  { label: 'All results', value: '' },
  { label: 'Success', value: 'success' },
  { label: 'Failure', value: 'failure' },
];

const PAGE_SIZE = 25;

export default function AdminActivityPage() {
  const params = useParams();
  const companyId = params?.id as string;

  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState(false);
  const [filter, setFilter] = useState('');
  const [category, setCategory] = useState<SelectProps.Option>(CATEGORY_OPTIONS[0]);
  const [result, setResult] = useState<SelectProps.Option>(RESULT_OPTIONS[0]);
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
      <ContentLayout header={<Header variant="h1">Admin Activity Log</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (fetchError) {
    return (
      <ContentLayout header={<Header variant="h1">Admin Activity Log</Header>}>
        <Box textAlign="center" padding="xl">
          <SpaceBetween size="m" alignItems="center">
            <Box color="text-status-error">Failed to load audit logs. Check your connection or permissions.</Box>
            <Button iconName="refresh" onClick={fetchLogs}>Retry</Button>
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
          description="Audit trail of all admin-level actions across the organization."
          actions={<Button iconName="refresh" onClick={fetchLogs}>Refresh</Button>}
        >
          Admin Activity Log
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Time',
              cell: (item: AuditLog) => (
                <Box fontSize="body-s" color="text-body-secondary">
                  {item.created_at ? new Date(item.created_at).toLocaleString() : '—'}
                </Box>
              ),
              width: '16%',
            },
            {
              header: 'Category / Action',
              cell: (item: AuditLog) => (
                <SpaceBetween size="xxxs">
                  <Badge color={item.category === 'admin' ? 'blue' : 'grey'}>{item.category}</Badge>
                  <Box fontSize="body-s" fontWeight="bold">{item.action}</Box>
                </SpaceBetween>
              ),
              width: '22%',
            },
            {
              header: 'Actor',
              cell: (item: AuditLog) => (
                <Box fontSize="body-s" color="text-body-secondary">
                  {item.actor_id || '—'}
                </Box>
              ),
              width: '20%',
            },
            {
              header: 'Target',
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
              header: 'IP',
              cell: (item: AuditLog) => (
                <Box fontSize="body-s" color="text-body-secondary">{item.ip_address || '—'}</Box>
              ),
              width: '12%',
            },
            {
              header: 'Result',
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
              Events
            </Header>
          }
          filter={
            <SpaceBetween direction="horizontal" size="xs">
              <TextFilter
                filteringText={filter}
                filteringPlaceholder="Search action, actor, IP..."
                onChange={e => { setFilter(e.detail.filteringText); setCurrentPage(1); }}
              />
              <Select
                selectedOption={category}
                options={CATEGORY_OPTIONS}
                onChange={e => { setCategory(e.detail.selectedOption); setCurrentPage(1); }}
                expandToViewport
              />
              <Select
                selectedOption={result}
                options={RESULT_OPTIONS}
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
              No audit events found.
            </Box>
          }
        />

        {selected.length > 0 && (
          <ExpandableSection headerText={`Detail: ${selected[0].action} — ${selected[0].id}`} defaultExpanded>
            <SpaceBetween size="s">
              {([
                ['ID', selected[0].id],
                ['Category', selected[0].category],
                ['Action', selected[0].action],
                ['Actor', selected[0].actor_id],
                ['Target Type', selected[0].target_type],
                ['Target ID', selected[0].target_id],
                ['IP Address', selected[0].ip_address],
                ['Result', selected[0].result],
                ['Time', selected[0].created_at ? new Date(selected[0].created_at).toISOString() : '—'],
              ] as [string, string][]).map(([label, value]) => (
                <ColumnLayout key={label} columns={2} variant="text-grid">
                  <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">{label}</Box>
                  <Box fontSize="body-s">{value || '—'}</Box>
                </ColumnLayout>
              ))}
              {selected[0].detail && Object.keys(selected[0].detail).length > 0 && (
                <SpaceBetween size="xxs">
                  <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">Detail</Box>
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

