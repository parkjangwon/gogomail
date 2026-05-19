'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  Box,
  ButtonDropdown,
  TextFilter,
  SpaceBetween,
  Button,
  Input,
  FormField,
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
import { buildAuditLogsQuery, exportAuditLogsCsv, type AuditLogRow } from '@/lib/auditLogs';
import { formatDateTime } from '@/lib/format';

interface AuditLog extends AuditLogRow {
  company_id?: string;
  domain_id?: string;
  user_id?: string;
  id: string;
  actor_id: string;
}

const resultType = (r: string): 'success' | 'error' | 'pending' =>
  r === 'success' ? 'success' : r === 'error' ? 'error' : 'pending';

export default function AuditLogsPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;

  const CATEGORY_OPTIONS: SelectProps.Option[] = [
    { label: t('pages.audit_logs_page.cat_all'), value: '' },
    { label: t('pages.audit_logs_page.cat_config'), value: 'config' },
    { label: t('pages.audit_logs_page.cat_security'), value: 'security' },
    { label: t('pages.audit_logs_page.cat_user'), value: 'user' },
    { label: t('pages.audit_logs_page.cat_domain'), value: 'domain' },
    { label: t('pages.audit_logs_page.cat_auth'), value: 'auth' },
  ];

  const TARGET_TYPE_OPTIONS: SelectProps.Option[] = [
    { label: t('common.all'), value: '' },
    { label: t('pages.audit_logs_page.target_type_user'), value: 'user' },
    { label: t('pages.audit_logs_page.target_type_domain'), value: 'domain' },
    { label: t('pages.audit_logs_page.target_type_session'), value: 'session' },
    { label: t('pages.audit_logs_page.target_type_role'), value: 'role' },
    { label: t('pages.audit_logs_page.target_type_config'), value: 'config' },
  ];

  const LIMIT_OPTIONS: SelectProps.Option[] = [
    { label: t('pages.audit_logs_page.limit_50'), value: '50' },
    { label: t('pages.audit_logs_page.limit_100'), value: '100' },
    { label: t('pages.audit_logs_page.limit_500'), value: '500' },
  ];

  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [actionFilter, setActionFilter] = useState('');
  const [targetTypeFilter, setTargetTypeFilter] = useState('');
  const [fromDate, setFromDate] = useState('');
  const [toDate, setToDate] = useState('');
  const [limit, setLimit] = useState('100');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [exporting, setExporting] = useState(false);

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const query = buildAuditLogsQuery({
        companyId: cid,
        category: categoryFilter,
        action: actionFilter,
        targetType: targetTypeFilter,
        fromDate,
        toDate,
        limit: Number(limit),
        offset: 0,
      });
      const res = await fetch(`/admin/v1/audit-logs${query ? `?${query}` : ''}`);
      const data = await res.json();
      setLogs(data.audit_logs ?? []);
    } catch {
      setFlash([{ type: 'error', content: t('pages.audit_logs_page.load_error'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setLoading(false);
    }
  }, [actionFilter, categoryFilter, cid, fromDate, limit, targetTypeFilter, t, toDate]);

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
        const csv = exportAuditLogsCsv(
          filtered.map((log) => ({
            id: log.id,
            actor_id: log.actor_id,
            category: log.category,
            action: log.action,
            target_type: log.target_type,
            target_id: log.target_id,
            result: log.result,
            ip_address: log.ip_address,
            created_at: log.created_at,
          }))
        );
        const blob = new Blob([csv], { type: 'text/csv' });
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
      setFlash([{ type: 'success', content: t('pages.audit_logs_page.export'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: e instanceof Error ? e.message : 'An unexpected error occurred.', dismissible: true, onDismiss: () => setFlash([]) }]);
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
                  { id: 'csv', text: t('pages.audit_logs_page.export_csv') },
                  { id: 'json', text: t('pages.audit_logs_page.export_json') },
                ]}
                onItemClick={({ detail }) => handleExport(detail.id as 'csv' | 'json')}
              >
                {t('pages.audit_logs_page.export')}
              </ButtonDropdown>
              <Button iconName="refresh" onClick={load} loading={loading}>
                {t('pages.audit_logs_page.refresh')}
              </Button>
            </SpaceBetween>
          }
        >
          {t('nav.audit_logs')}
        </Header>
      }
    >
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}

        <DataTable
          items={filtered}
          loading={loading}
          loadingText={t('pages.audit_logs_page.loading')}
          columnDefinitions={[
            { id: 'time', header: t('pages.audit_logs_page.col_time'), cell: (l) => formatDateTime(l.created_at), width: 160 },
            { id: 'actor', header: t('pages.audit_logs_page.col_actor'), cell: (l) => l.actor_id || '—' },
            { id: 'action', header: t('pages.audit_logs_page.col_action'), cell: (l) => <Box variant="code">{l.action}</Box> },
            { id: 'category', header: t('pages.audit_logs_page.col_category'), cell: (l) => <Badge color="blue">{l.category}</Badge> },
            { id: 'target', header: t('pages.audit_logs_page.col_target'), cell: (l) => l.target_type ? `${l.target_type}:${l.target_id}` : '—' },
            { id: 'result', header: t('pages.audit_logs_page.col_result'), cell: (l) => <StatusIndicator type={resultType(l.result)}>{l.result}</StatusIndicator> },
            { id: 'ip', header: t('pages.audit_logs_page.col_ip'), cell: (l) => l.ip_address || '—' },
          ]}
          header={
            <Header variant="h2" counter={`(${filtered.length})`}>
              {t('pages.audit_logs_page.audit_trail')}
            </Header>
          }
          filter={
            <SpaceBetween size="xs" direction="horizontal">
              <TextFilter
                filteringText={filter}
                filteringPlaceholder={t('pages.audit_logs_page.search_placeholder')}
                onChange={(e) => setFilter(e.detail.filteringText)}
                countText={`${filtered.length}`}
              />
              <Select
                selectedOption={CATEGORY_OPTIONS.find(o => o.value === categoryFilter) ?? CATEGORY_OPTIONS[0]}
                options={CATEGORY_OPTIONS}
                onChange={(e) => setCategoryFilter(e.detail.selectedOption.value ?? '')}
              />
              <Select
                selectedOption={TARGET_TYPE_OPTIONS.find(o => o.value === targetTypeFilter) ?? TARGET_TYPE_OPTIONS[0]}
                options={TARGET_TYPE_OPTIONS}
                onChange={(e) => setTargetTypeFilter(e.detail.selectedOption.value ?? '')}
              />
              <FormField label={t('pages.audit_logs_page.action_filter')}>
                <Input value={actionFilter} onChange={(e) => setActionFilter(e.detail.value)} placeholder={t('pages.audit_logs_page.action_filter_placeholder')} />
              </FormField>
              <FormField label={t('pages.audit_logs_page.date_from')}>
                <Input value={fromDate} onChange={(e) => setFromDate(e.detail.value)} placeholder="2026-05-01T00:00:00Z" />
              </FormField>
              <FormField label={t('pages.audit_logs_page.date_to')}>
                <Input value={toDate} onChange={(e) => setToDate(e.detail.value)} placeholder="2026-05-31T23:59:59Z" />
              </FormField>
              <Select
                selectedOption={LIMIT_OPTIONS.find(o => o.value === limit) ?? LIMIT_OPTIONS[1]}
                options={LIMIT_OPTIONS}
                onChange={(e) => setLimit(e.detail.selectedOption.value ?? '100')}
              />
            </SpaceBetween>
          }
          empty={<Box textAlign="center" padding="l" color="inherit">{t('pages.audit_logs_page.no_logs')}</Box>}
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
