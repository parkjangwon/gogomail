'use client';

import {
  ContentLayout,
  Header,
  Table,
  Box,
  ButtonDropdown,
  TextFilter,
  SpaceBetween,
  Button,
  Select,
  SelectProps,
  Badge,
  StatusIndicator,
  Flashbar,
  FlashbarProps,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface AuditLog {
  id: string;
  actor_id: string;
  category: string;
  action: string;
  target_type: string;
  target_id: string;
  result: string;
  ip_address: string;
  created_at: string;
}

const CATEGORY_OPTIONS: SelectProps.Option[] = [
  { label: 'All Categories', value: '' },
  { label: 'Config', value: 'config' },
  { label: 'Security', value: 'security' },
  { label: 'User', value: 'user' },
  { label: 'Domain', value: 'domain' },
  { label: 'Auth', value: 'auth' },
];

const LIMIT_OPTIONS: SelectProps.Option[] = [
  { label: 'Last 50', value: '50' },
  { label: 'Last 100', value: '100' },
  { label: 'Last 500', value: '500' },
];

const resultType = (r: string): 'success' | 'error' | 'pending' =>
  r === 'success' ? 'success' : r === 'error' ? 'error' : 'pending';

export default function AuditLogsPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;

  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [limit, setLimit] = useState('100');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [exporting, setExporting] = useState(false);

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit });
      if (categoryFilter) params.set('category', categoryFilter);
      const res = await fetch(`/admin/v1/audit-logs?company_id=${cid}&${params}`);
      const data = await res.json();
      setLogs(data.audit_logs ?? []);
    } catch {
      setFlash([{ type: 'error', content: 'Failed to load audit logs', dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setLoading(false);
    }
  }, [cid, categoryFilter, limit]);

  useEffect(() => { load(); }, [load]);

  const filtered = useMemo(() => {
    if (!filter) return logs;
    const q = filter.toLowerCase();
    return logs.filter(l =>
      l.action.toLowerCase().includes(q) ||
      l.actor_id?.toLowerCase().includes(q) ||
      l.target_id?.toLowerCase().includes(q) ||
      l.category?.toLowerCase().includes(q)
    );
  }, [logs, filter]);

  const handleExport = async (format: 'csv' | 'json') => {
    if (!cid) return;
    setExporting(true);
    try {
      if (format === 'csv') {
        const params = new URLSearchParams({ limit: '10000' });
        if (categoryFilter) params.set('category', categoryFilter);
        const res = await fetch(`/admin/v1/companies/${cid}/audit-logs/export?${params}`);
        if (!res.ok) throw new Error(await res.text());
        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `audit-logs-${cid}.csv`;
        a.click();
        URL.revokeObjectURL(url);
      } else {
        const content = JSON.stringify(filtered, null, 2);
        const blob = new Blob([content], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `audit-logs-${cid}.json`;
        a.click();
        URL.revokeObjectURL(url);
      }
      setFlash([{ type: 'success', content: `Exported as ${format.toUpperCase()}`, dismissible: true, onDismiss: () => setFlash([]) }]);
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setExporting(false);
    }
  };

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          counter={`(${filtered.length})`}
          actions={
            <SpaceBetween size="xs" direction="horizontal">
              <ButtonDropdown
                loading={exporting}
                items={[
                  { id: 'csv', text: 'Export as CSV' },
                  { id: 'json', text: 'Export as JSON' },
                ]}
                onItemClick={({ detail }) => handleExport(detail.id as 'csv' | 'json')}
              >
                Export
              </ButtonDropdown>
              <Button iconName="refresh" onClick={load} loading={loading}>Refresh</Button>
            </SpaceBetween>
          }
        >
          {t('nav.audit_logs')}
        </Header>
      }
    >
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}

        <Table
          items={filtered}
          loading={loading}
          loadingText="Loading audit logs…"
          columnDefinitions={[
            { id: 'time', header: 'Time', cell: (l) => new Date(l.created_at).toLocaleString(), width: 160 },
            { id: 'actor', header: 'Actor', cell: (l) => l.actor_id || '—' },
            { id: 'action', header: 'Action', cell: (l) => <Box variant="code">{l.action}</Box> },
            { id: 'category', header: 'Category', cell: (l) => <Badge color="blue">{l.category}</Badge> },
            { id: 'target', header: 'Target', cell: (l) => l.target_type ? `${l.target_type}:${l.target_id}` : '—' },
            { id: 'result', header: 'Result', cell: (l) => <StatusIndicator type={resultType(l.result)}>{l.result}</StatusIndicator> },
            { id: 'ip', header: 'IP', cell: (l) => l.ip_address || '—' },
          ]}
          header={
            <Header variant="h2" counter={`(${filtered.length})`}>
              Audit Trail
            </Header>
          }
          filter={
            <SpaceBetween size="xs" direction="horizontal">
              <TextFilter
                filteringText={filter}
                filteringPlaceholder="Search action, actor, target…"
                onChange={(e) => setFilter(e.detail.filteringText)}
                countText={`${filtered.length} entries`}
              />
              <Select
                selectedOption={CATEGORY_OPTIONS.find(o => o.value === categoryFilter) ?? CATEGORY_OPTIONS[0]}
                options={CATEGORY_OPTIONS}
                onChange={(e) => setCategoryFilter(e.detail.selectedOption.value ?? '')}
              />
              <Select
                selectedOption={LIMIT_OPTIONS.find(o => o.value === limit) ?? LIMIT_OPTIONS[1]}
                options={LIMIT_OPTIONS}
                onChange={(e) => setLimit(e.detail.selectedOption.value ?? '100')}
              />
            </SpaceBetween>
          }
          empty={<Box textAlign="center" padding="l" color="inherit">No audit logs found</Box>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
