'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  ColumnLayout,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Button,
  ProgressBar,
  KeyValuePairs,
  StatusIndicator,
  Alert,
  Tabs,
  Modal,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { useState, useEffect, useMemo, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useCompany } from '@/contexts/CompanyContext';
import { useI18n } from '@/app/i18n-provider';

interface Domain {
  id: string;
  name: string;
  status: string;
  last_dns_check_status: string;
  quota_used: number;
  quota_limit: number;
  created_at: string;
}

interface ConfigEntry {
  id?: string;
  key: string;
  value: string;
  last_updated?: string;
}

export default function CompanyOverviewPage() {
  const { t } = useI18n();
  const params = useParams();
  const router = useRouter();
  const companyId = params?.id as string;
  const { companies, currentCompany, refresh } = useCompany();

  const [domains, setDomains] = useState<Domain[]>([]);
  const [loadingDomains, setLoadingDomains] = useState(true);
  const [activeTab, setActiveTab] = useState('overview');

  const [configs, setConfigs] = useState<ConfigEntry[]>([]);
  const [loadingConfigs, setLoadingConfigs] = useState(false);

  const [editQuotaOpen, setEditQuotaOpen] = useState(false);
  const [quotaGb, setQuotaGb] = useState('');
  const [savingQuota, setSavingQuota] = useState(false);

  const [addConfigOpen, setAddConfigOpen] = useState(false);
  const [newConfig, setNewConfig] = useState({ key: '', value: '' });
  const [savingConfig, setSavingConfig] = useState(false);

  const company =
    companies.find(c => c.id === companyId) ??
    (companyId === 'default' ? currentCompany : null);

  useEffect(() => {
    if (!company) return;
    setLoadingDomains(true);
    fetch(`/api/admin/domains?company_id=${company.id}&limit=100`, { credentials: 'include' })
      .then(r => r.ok ? r.json() : { domains: [] })
      .then(d => setDomains(d.domains || []))
      .catch((err) => { console.error('Failed to load domains:', err); })
      .finally(() => setLoadingDomains(false));
  }, [company]);

  const fetchConfigs = useCallback(() => {
    if (!company) return;
    setLoadingConfigs(true);
    fetch(`/api/admin/companies/${company.id}/config`, { credentials: 'include' })
      .then(r => r.ok ? r.json() : { config: [] })
      .then(d => setConfigs(d.config || d.entries || []))
      .catch((err) => { console.error('Failed to load configs:', err); setConfigs([]); })
      .finally(() => setLoadingConfigs(false));
  }, [company]);

  useEffect(() => {
    if (activeTab === 'settings') fetchConfigs();
  }, [activeTab, company?.id]);

  const handleSaveQuota = async () => {
    if (!company) return;
    setSavingQuota(true);
    try {
      const limitBytes = quotaGb.trim() === '' ? 0 : Math.round(parseFloat(quotaGb) * 1073741824);
      const res = await fetch(`/api/admin/companies/${company.id}/quota`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: company.id, quota_limit: limitBytes }),
        credentials: 'include',
      });
      if (res.ok) {
        setEditQuotaOpen(false);
        refresh();
      }
    } finally {
      setSavingQuota(false);
    }
  };

  const handleAddConfig = async () => {
    if (!company || !newConfig.key.trim()) return;
    setSavingConfig(true);
    try {
      const res = await fetch(`/api/admin/companies/${company.id}/config/${encodeURIComponent(newConfig.key.trim())}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value: newConfig.value }),
        credentials: 'include',
      });
      if (res.ok) {
        setAddConfigOpen(false);
        setNewConfig({ key: '', value: '' });
        fetchConfigs();
      }
    } finally {
      setSavingConfig(false);
    }
  };

  const handleDeleteConfig = useCallback(async (key: string) => {
    if (!company) return;
    const res = await fetch(`/api/admin/companies/${company.id}/config/${encodeURIComponent(key)}`, {
      method: 'DELETE',
      credentials: 'include',
    });
    if (res.ok) fetchConfigs();
  }, [company, fetchConfigs]);

  if (!company) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.company_overview.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const quotaLimit = company.quota_limit ?? 0;
  const quotaUsed = company.quota_used ?? 0;
  const quotaPct = quotaLimit > 0 ? Math.round((quotaUsed / quotaLimit) * 100) : 0;
  const dnsIssues = useMemo(() => domains.filter(d => d.last_dns_check_status !== 'pass' && d.last_dns_check_status !== ''), [domains]);
  const activeDomains = useMemo(() => domains.filter(d => d.status === 'active'), [domains]);

  const domainColumnDefs = useMemo(() => [
    {
      header: t('pages.company_overview.domain'),
      cell: (d: Domain) => (
        <Button
          variant="inline-link"
          onClick={() => router.push(`/companies/${companyId}/domains/${d.id}`)}
        >
          {d.name}
        </Button>
      ),
      width: '30%',
    },
    {
      header: t('pages.company_overview.status'),
      cell: (d: Domain) => (
        <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>
      ),
      width: '15%',
    },
    {
      header: t('pages.company_overview.dns'),
      cell: (d: Domain) => {
        const s = d.last_dns_check_status;
        return <Badge color={s === 'pass' ? 'green' : s === 'fail' ? 'red' : 'grey'}>{s || t('pages.company_overview.unchecked')}</Badge>;
      },
      width: '15%',
    },
    {
      header: t('pages.company_overview.storage_col'),
      cell: (d: Domain) => {
        const limit = d.quota_limit ?? 0;
        const used = d.quota_used ?? 0;
        return limit > 0
          ? `${(used / 1073741824).toFixed(1)} / ${(limit / 1073741824).toFixed(1)} GB`
          : `${(used / 1073741824).toFixed(1)} GB`;
      },
      width: '25%',
    },
    {
      header: t('pages.company_overview.added'),
      cell: (d: Domain) => new Date(d.created_at).toLocaleDateString(),
      width: '15%',
    },
  ], [t, router, companyId]);

  const configColumnDefs = useMemo(() => [
    { header: t('pages.company_overview.config_key'), cell: (c: ConfigEntry) => <Box fontWeight="bold">{c.key}</Box>, width: '30%' },
    { header: t('pages.company_overview.config_value'), cell: (c: ConfigEntry) => c.value, width: '40%' },
    { header: t('pages.company_overview.config_updated'), cell: (c: ConfigEntry) => c.last_updated ? new Date(c.last_updated).toLocaleString() : '—', width: '20%' },
    {
      header: t('pages.company_overview.config_actions'),
      cell: (c: ConfigEntry) => (
        <Button variant="inline-link" onClick={() => handleDeleteConfig(c.key)}>
          {t('pages.company_overview.delete')}
        </Button>
      ),
      width: '10%',
    },
  ], [t, handleDeleteConfig]);

  const overviewTab = (
    <SpaceBetween size="l">
      {dnsIssues.length > 0 && (
        <Alert
          type="warning"
          header={`${dnsIssues.length} ${t('pages.company_overview.dns_issues_header')}`}
          action={
            <Button onClick={() => setActiveTab('domains')}>
              {t('pages.company_overview.review_domains')}
            </Button>
          }
        >
          {dnsIssues.map(d => d.name).join(', ')}
        </Alert>
      )}

      <ColumnLayout columns={3} minColumnWidth={170}>
        <Container header={<Header variant="h3">{t('pages.company_overview.storage')}</Header>}>
          <SpaceBetween size="s">
            <ProgressBar
              value={quotaPct}
              status={company.over_allocated ? 'error' : quotaPct > 80 ? 'in-progress' : 'success'}
              resultText={`${quotaPct}%`}
              additionalInfo={
                quotaLimit > 0
                  ? `${(quotaUsed / 1073741824).toFixed(1)} / ${(quotaLimit / 1073741824).toFixed(1)} GB`
                  : `${(quotaUsed / 1073741824).toFixed(1)} GB (${t('pages.company_overview.quota_unlimited')})`
              }
            />
            {company.over_allocated && (
              <StatusIndicator type="error">{t('pages.company_overview.over_allocated')}</StatusIndicator>
            )}
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h3">{t('pages.company_overview.domains')}</Header>}>
          <SpaceBetween size="s">
            <Box fontSize="display-l" fontWeight="bold">{domains.length}</Box>
            <Box color="text-body-secondary" fontSize="body-s">
              {activeDomains.length} {t('pages.company_overview.active')}
              {dnsIssues.length > 0 && ` · ${dnsIssues.length} ${t('pages.company_overview.dns_issues')}`}
            </Box>
            <Button variant="inline-link" onClick={() => setActiveTab('domains')}>
              {t('pages.company_overview.view_all')} →
            </Button>
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h3">{t('pages.company_overview.info')}</Header>}>
          <KeyValuePairs
            items={[
              { label: t('pages.company_overview.status'), value: <Badge color={company.status === 'active' ? 'green' : 'grey'}>{company.status}</Badge> },
              { label: t('pages.company_overview.created'), value: new Date(company.created_at).toLocaleDateString() },
            ]}
          />
        </Container>
      </ColumnLayout>
    </SpaceBetween>
  );

  const domainsTab = (
    <DataTable
      loading={loadingDomains}
      loadingText={t('pages.company_overview.loading_domains')}
      columnDefinitions={domainColumnDefs}
      items={domains}
      header={
        <Header
          variant="h2"
          counter={`(${domains.length})`}
          actions={
            <Button onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
              {t('pages.company_overview.add_domain_btn')}
            </Button>
          }
        >
          {t('pages.company_overview.domains_under')} {company.name}
        </Header>
      }
      empty={
        <Box textAlign="center" padding="l">
          <SpaceBetween size="m" alignItems="center">
            <StatusIndicator type="warning">{t('pages.company_overview.no_domains')}</StatusIndicator>
            <Box color="text-body-secondary">{t('pages.company_overview.no_domains_desc')}</Box>
            <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
              {t('pages.company_overview.add_first_domain')}
            </Button>
          </SpaceBetween>
        </Box>
      }
    />
  );

  const settingsTab = (
    <SpaceBetween size="l">
      <Container header={<Header variant="h2">{t('pages.company_overview.basic_info')}</Header>}>
        <KeyValuePairs
          columns={2}
          items={[
            { label: t('pages.company_overview.name'), value: company.name },
            { label: t('pages.company_overview.company_id'), value: <Box fontSize="body-s" color="text-body-secondary">{company.id}</Box> },
            { label: t('pages.company_overview.status'), value: <Badge color={company.status === 'active' ? 'green' : 'grey'}>{company.status}</Badge> },
            { label: t('pages.company_overview.created'), value: new Date(company.created_at).toLocaleString() },
          ]}
        />
      </Container>

      <Container
        header={
          <Header
            variant="h2"
            actions={
              <Button onClick={() => {
                setQuotaGb(quotaLimit > 0 ? (quotaLimit / 1073741824).toString() : '');
                setEditQuotaOpen(true);
              }}>
                {t('pages.company_overview.edit_quota')}
              </Button>
            }
          >
            {t('pages.company_overview.storage')}
          </Header>
        }
      >
        <SpaceBetween size="m">
          {quotaLimit > 0 ? (
            <>
              <ProgressBar
                value={quotaPct}
                status={company.over_allocated ? 'error' : quotaPct > 80 ? 'in-progress' : 'success'}
                resultText={`${quotaPct}%`}
                additionalInfo={`${(quotaUsed / 1073741824).toFixed(2)} / ${(quotaLimit / 1073741824).toFixed(2)} GB`}
              />
            </>
          ) : (
            <StatusIndicator type="success">{t('pages.company_overview.quota_unlimited')}</StatusIndicator>
          )}
          <KeyValuePairs
            columns={2}
            items={[
              { label: t('pages.company_overview.quota_used_label'), value: `${(quotaUsed / 1073741824).toFixed(2)} GB` },
              { label: t('pages.company_overview.quota_limit_label'), value: quotaLimit > 0 ? `${(quotaLimit / 1073741824).toFixed(2)} GB` : t('pages.company_overview.quota_unlimited') },
            ]}
          />
        </SpaceBetween>
      </Container>

      <DataTable
        loading={loadingConfigs}
        items={configs}
        columnDefinitions={configColumnDefs}
        header={
          <Header
            variant="h2"
            description={t('pages.company_overview.config_desc')}
            counter={`(${configs.length})`}
            actions={
              <Button variant="primary" onClick={() => setAddConfigOpen(true)}>
                {t('pages.company_overview.add_config_btn')}
              </Button>
            }
          >
            {t('pages.company_overview.custom_config')}
          </Header>
        }
        empty={
          <Box textAlign="center" padding="l">
            <Box color="text-body-secondary">{t('pages.company_overview.no_configs')}</Box>
          </Box>
        }
      />
    </SpaceBetween>
  );

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={`ID: ${company.id}`}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => router.push(`/companies/${companyId}/users`)}>
                {t('pages.company_overview.manage_users')}
              </Button>
              <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/dashboard`)}>
                {t('pages.company_overview.dashboard_btn')} →
              </Button>
            </SpaceBetween>
          }
        >
          <SpaceBetween direction="horizontal" size="xs">
            <span>{company.name}</span>
            <Badge color={company.status === 'active' ? 'green' : 'severity-high'}>{company.status}</Badge>
          </SpaceBetween>
        </Header>
      }
    >
      <Tabs
        activeTabId={activeTab}
        onChange={(e) => setActiveTab(e.detail.activeTabId)}
        tabs={[
          { id: 'overview', label: t('pages.company_overview.tab_overview'), content: overviewTab },
          { id: 'domains', label: `${t('pages.company_overview.tab_domains')} (${domains.length})`, content: domainsTab },
          { id: 'settings', label: t('pages.company_overview.tab_settings'), content: settingsTab },
        ]}
      />

      <Modal
        visible={editQuotaOpen}
        onDismiss={() => setEditQuotaOpen(false)}
        header={t('pages.company_overview.quota_modal_title')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setEditQuotaOpen(false)}>{t('common.cancel')}</Button>
              <Button variant="primary" loading={savingQuota} onClick={handleSaveQuota}>
                {t('common.save')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <FormField
          label={t('pages.company_overview.quota_label_gb')}
          description={t('pages.company_overview.quota_help')}
        >
          <Input
            type="number"
            value={quotaGb}
            onChange={(e) => setQuotaGb(e.detail.value)}
            placeholder="0"
            autoFocus
          />
        </FormField>
      </Modal>

      <Modal
        visible={addConfigOpen}
        onDismiss={() => setAddConfigOpen(false)}
        header={t('pages.company_overview.config_modal_title')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setAddConfigOpen(false)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                loading={savingConfig}
                disabled={!newConfig.key.trim()}
                onClick={handleAddConfig}
              >
                {t('common.save')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.company_overview.config_key')}>
            <Input
              value={newConfig.key}
              onChange={(e) => setNewConfig({ ...newConfig, key: e.detail.value })}
              placeholder={t('pages.company_overview.config_key_placeholder')}
              autoFocus
            />
          </FormField>
          <FormField label={t('pages.company_overview.config_value')}>
            <Input
              value={newConfig.value}
              onChange={(e) => setNewConfig({ ...newConfig, value: e.detail.value })}
              placeholder={t('pages.company_overview.config_value_placeholder')}
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
