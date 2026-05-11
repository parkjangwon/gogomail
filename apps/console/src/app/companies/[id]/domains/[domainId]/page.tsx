'use client';

import {
  ContentLayout,
  Header,
  Tabs,
  Container,
  ColumnLayout,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Button,
  ProgressBar,
  KeyValuePairs,
  Table,
  StatusIndicator,
  Alert,
  FormField,
  Input,
  Modal,
  Select,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface DomainDetail {
  id: string;
  company_id: string;
  company_name: string;
  name: string;
  name_ace: string;
  status: string;
  last_dns_check_status: string;
  last_dns_checked_at?: string;
  quota_used: number;
  quota_limit: number;
  quota_remaining: number;
  allocated_user_quota: number;
  allocatable_user_quota: number;
  over_allocated: boolean;
  created_at: string;
}

interface User {
  id: string;
  username: string;
  display_name: string;
  status: string;
  quota_used: number;
  quota_limit: number;
  created_at: string;
}

interface DomainSetting {
  ID: string;
  Key: string;
  Value: unknown;
  UpdatedAt: string;
}

interface DailyCount {
  date: string;
  label: string;
  total: number;
  success: number;
  failed: number;
}

const STATUS_OPTIONS = [
  { label: 'active', value: 'active' },
  { label: 'suspended', value: 'suspended' },
];

export default function DomainDetailPage() {
  const { t } = useI18n();
  const params = useParams();
  const router = useRouter();
  const companyId = params?.id as string;
  const domainId = params?.domainId as string;

  const [domain, setDomain] = useState<DomainDetail | null>(null);
  const [users, setUsers] = useState<User[]>([]);
  const [settings, setSettings] = useState<DomainSetting[]>([]);
  const [loading, setLoading] = useState(true);
  const [verifying, setVerifying] = useState(false);
  const [activeTab, setActiveTab] = useState('overview');

  // Add Setting modal
  const [showAddSetting, setShowAddSetting] = useState(false);
  const [newSetting, setNewSetting] = useState({ key: '', value: '' });
  const [savingSetting, setSavingSetting] = useState(false);

  // Edit modal
  const [showEdit, setShowEdit] = useState(false);
  const [editForm, setEditForm] = useState({ quota_gb: '', status: 'active' });
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');

  // Delete modal
  const [showDelete, setShowDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  // Mail stats
  const [mailStats, setMailStats] = useState<DailyCount[]>([]);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsFetched, setStatsFetched] = useState(false);

  useEffect(() => {
    Promise.all([
      fetch(`/api/admin/domains/${domainId}`, { credentials: 'include' }).then(r => r.ok ? r.json() : null),
      fetch(`/api/admin/users?domain_id=${domainId}&limit=100`, { credentials: 'include' }).then(r => r.ok ? r.json() : { users: [] }),
      fetch(`/api/admin/domains/${domainId}/config`, { credentials: 'include' }).then(r => r.ok ? r.json() : { config: [] }),
    ]).then(([domainData, usersData, settingsData]) => {
      if (domainData?.domain) {
        setDomain(domainData.domain);
        setEditForm({
          quota_gb: domainData.domain.quota_limit > 0 ? String(Math.round(domainData.domain.quota_limit / 1073741824)) : '',
          status: domainData.domain.status,
        });
      }
      setUsers(usersData.users || []);
      setSettings(settingsData.config || []);
    }).catch(() => {}).finally(() => setLoading(false));
  }, [domainId]);

  const handleVerifyDNS = async () => {
    setVerifying(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/dns-check`, {
        method: 'POST',
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setDomain(prev => prev ? { ...prev, last_dns_check_status: data.dns_check?.status ?? prev.last_dns_check_status } : prev);
      }
    } finally {
      setVerifying(false);
    }
  };

  const handleSaveEdit = async () => {
    setSaving(true);
    setSaveError('');
    try {
      const quotaBytes = editForm.quota_gb ? parseInt(editForm.quota_gb, 10) * 1073741824 : 0;
      const statusChanged = domain?.status !== editForm.status;

      const calls: Promise<Response>[] = [
        fetch(`/api/admin/domains/${domainId}/quota`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ quota_limit: isNaN(quotaBytes) ? 0 : quotaBytes }),
          credentials: 'include',
        }),
      ];
      if (statusChanged) {
        calls.push(fetch(`/api/admin/domains/${domainId}/status`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ status: editForm.status }),
          credentials: 'include',
        }));
      }

      const results = await Promise.all(calls);
      const failed = results.find(r => !r.ok);
      if (failed) {
        const errData = await failed.json().catch(() => ({})) as { error?: { message?: string } };
        setSaveError(errData.error?.message ?? '저장 실패');
        return;
      }
      const refreshed = await fetch(`/api/admin/domains/${domainId}`, { credentials: 'include' });
      if (refreshed.ok) {
        const d = await refreshed.json();
        setDomain(d.domain);
      }
      setShowEdit(false);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    setDeleteError('');
    try {
      const res = await fetch(`/api/admin/domains/${domainId}`, { method: 'DELETE', credentials: 'include' });
      if (res.ok) {
        router.push(`/companies/${companyId}/tenancy/domains`);
      } else {
        const data = await res.json().catch(() => ({})) as { error?: { message?: string } };
        setDeleteError(data.error?.message ?? '삭제 실패');
      }
    } finally {
      setDeleting(false);
    }
  };

  const handleAddSetting = async () => {
    if (!newSetting.key.trim()) return;
    setSavingSetting(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/config/${encodeURIComponent(newSetting.key.trim())}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value: newSetting.value }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowAddSetting(false);
        setNewSetting({ key: '', value: '' });
        const r = await fetch(`/api/admin/domains/${domainId}/config`, { credentials: 'include' });
        if (r.ok) { const d = await r.json(); setSettings(d.config || []); }
      }
    } finally {
      setSavingSetting(false);
    }
  };

  const fetchMailStats = async (_domainName: string, force = false) => {
    if (statsFetched && !force) return;
    setStatsLoading(true);
    try {
      const since = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString();
      const qs = new URLSearchParams({
        company_id: companyId,
        domain_id: domainId,
        limit: '500',
        since,
      });
      const res = await fetch(`/api/admin/mail-flow-logs?${qs}`, { credentials: 'include' });
      if (!res.ok) return;
      const data = await res.json();
      const logs: Array<{ created_at: string; status: string }> = data.mail_flow_logs ?? [];

      const countMap = new Map<string, { total: number; success: number; failed: number }>();
      for (let i = 6; i >= 0; i--) {
        const d = new Date(Date.now() - i * 24 * 60 * 60 * 1000);
        const key = d.toISOString().slice(0, 10);
        countMap.set(key, { total: 0, success: 0, failed: 0 });
      }
      for (const log of logs) {
        const key = log.created_at?.slice(0, 10);
        if (key && countMap.has(key)) {
          const entry = countMap.get(key)!;
          entry.total++;
          if (log.status === 'delivered' || log.status === 'sent') entry.success++;
          else entry.failed++;
        }
      }

      const days: DailyCount[] = Array.from(countMap.entries()).map(([date, counts]) => ({
        date,
        label: new Date(date + 'T12:00:00').toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
        ...counts,
      }));
      setMailStats(days);
      setStatsFetched(true);
    } finally {
      setStatsLoading(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.domain_detail.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner size="large" /></Box>
      </ContentLayout>
    );
  }

  if (!domain) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.domain_detail.not_found')}</Header>}>
        <Alert type="error">{t('pages.domain_detail.title')} {domainId}</Alert>
      </ContentLayout>
    );
  }

  const quotaPct = domain.quota_limit > 0 ? Math.round((domain.quota_used / domain.quota_limit) * 100) : 0;
  const dnsColor = domain.last_dns_check_status === 'pass' ? 'green' : domain.last_dns_check_status === 'fail' ? 'red' : 'grey';

  return (
    <>
      <ContentLayout
        header={
          <Header
            variant="h1"
            description={
              <>
                <span>{t('pages.domain_detail.company')}: </span>
                <Button variant="inline-link" onClick={() => router.push(`/companies/${companyId}`)}>
                  {domain.company_name || domain.company_id}
                </Button>
              </>
            }
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                  ← {t('pages.domain_detail.back_to_list') || '도메인 목록'}
                </Button>
                <Button onClick={handleVerifyDNS} loading={verifying}>
                  {t('pages.domain_detail.verify_dns')}
                </Button>
                <Button onClick={() => setShowEdit(true)}>
                  {t('common.edit') || '수정'}
                </Button>
                <Button variant="normal" onClick={() => setShowDelete(true)}>
                  {t('common.delete') || '삭제'}
                </Button>
              </SpaceBetween>
            }
          >
            <SpaceBetween direction="horizontal" size="xs">
              <span>{domain.name}</span>
              <Badge color={domain.status === 'active' ? 'green' : 'grey'}>{domain.status}</Badge>
              <Badge color={dnsColor}>{t('pages.domain_detail.dns')}: {domain.last_dns_check_status || t('pages.domain_detail.unchecked')}</Badge>
            </SpaceBetween>
          </Header>
        }
      >
        <Tabs
          activeTabId={activeTab}
          onChange={(e) => {
            setActiveTab(e.detail.activeTabId);
            if (e.detail.activeTabId === 'mail-stats' && domain) fetchMailStats(domain.name);
          }}
          tabs={[
            {
              id: 'mail-stats',
              label: 'Mail Stats',
              content: (
                <Container header={
                <Header
                  variant="h2"
                  description={t('pages.domain_detail.mail_stats_desc')}
                  actions={
                    <Button iconName="refresh" loading={statsLoading} onClick={() => fetchMailStats(domain?.name ?? '', true)}>
                      Refresh
                    </Button>
                  }
                >
                  Daily Message Volume
                </Header>
              }>
                  {statsLoading ? (
                    <Box textAlign="center" padding="xl"><Spinner /></Box>
                  ) : (
                    <SpaceBetween size="l">
                      {mailStats.length === 0 ? (
                        <Box color="text-body-secondary" textAlign="center" padding="l">No mail data in the last 7 days.</Box>
                      ) : (() => {
                        const maxCount = Math.max(...mailStats.map(d => d.total), 1);
                        return (
                          <SpaceBetween size="m">
                            <div style={{ display: 'flex', alignItems: 'flex-end', gap: '12px', height: '140px', padding: '0 8px' }}>
                              {mailStats.map(day => (
                                <div key={day.date} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', flex: 1, height: '100%', justifyContent: 'flex-end' }}>
                                  {day.total > 0 && (
                                    <Box fontSize="body-s" color="text-body-secondary">{day.total}</Box>
                                  )}
                                  <div style={{ width: '100%', height: `${(day.total / maxCount) * 110}px`, minHeight: day.total > 0 ? '4px' : '0', display: 'flex', flexDirection: 'column', borderRadius: '3px', overflow: 'hidden' }}>
                                    <div style={{ flex: day.success, backgroundColor: '#1d8348' }} />
                                    <div style={{ flex: day.failed, backgroundColor: '#e74c3c' }} />
                                  </div>
                                </div>
                              ))}
                            </div>
                            <div style={{ display: 'flex', gap: '12px', padding: '0 8px' }}>
                              {mailStats.map(day => (
                                <div key={day.date} style={{ flex: 1, textAlign: 'center' }}>
                                  <Box fontSize="body-s" color="text-body-secondary">{day.label}</Box>
                                </div>
                              ))}
                            </div>
                            <SpaceBetween direction="horizontal" size="l">
                              <Box fontSize="body-s"><span style={{ display: 'inline-block', width: 12, height: 12, backgroundColor: '#1d8348', borderRadius: 2, marginRight: 6 }} />Delivered</Box>
                              <Box fontSize="body-s"><span style={{ display: 'inline-block', width: 12, height: 12, backgroundColor: '#e74c3c', borderRadius: 2, marginRight: 6 }} />Failed</Box>
                              <Box fontSize="body-s" color="text-body-secondary">
                                Total last 7d: {mailStats.reduce((s, d) => s + d.total, 0)} messages
                              </Box>
                            </SpaceBetween>
                          </SpaceBetween>
                        );
                      })()}
                    </SpaceBetween>
                  )}
                </Container>
              ),
            },
            {
              id: 'overview',
              label: t('pages.domain_detail.overview_tab'),
              content: (
                <SpaceBetween size="l">
                  <ColumnLayout columns={3}>
                    <Container header={<Header variant="h3">{t('pages.domain_detail.storage')}</Header>}>
                      <SpaceBetween size="s">
                        <ProgressBar
                          value={quotaPct}
                          status={domain.over_allocated ? 'error' : quotaPct > 80 ? 'in-progress' : 'success'}
                          resultText={`${quotaPct}%`}
                          additionalInfo={
                            domain.quota_limit > 0
                              ? `${(domain.quota_used / 1073741824).toFixed(2)} / ${(domain.quota_limit / 1073741824).toFixed(2)} GB`
                              : `${(domain.quota_used / 1073741824).toFixed(2)} GB (unlimited)`
                          }
                        />
                        {domain.over_allocated ? (
                          <StatusIndicator type="error">{t('pages.domain_detail.over_allocated')}</StatusIndicator>
                        ) : null}
                      </SpaceBetween>
                    </Container>

                    <Container header={<Header variant="h3">{t('pages.domain_detail.users')}</Header>}>
                      <SpaceBetween size="s">
                        <Box fontSize="display-l" fontWeight="bold">{users.length}</Box>
                        <Box color="text-body-secondary" fontSize="body-s">
                          {users.filter(u => u.status === 'active').length} {t('pages.domain_detail.active')}
                        </Box>
                        <Button variant="inline-link" onClick={() => setActiveTab('users')}>
                          {t('pages.domain_detail.view_all')} →
                        </Button>
                      </SpaceBetween>
                    </Container>

                    <Container header={<Header variant="h3">{t('pages.domain_detail.domain_info')}</Header>}>
                      <KeyValuePairs
                        items={[
                          { label: t('pages.domain_detail.status'), value: <Badge color={domain.status === 'active' ? 'green' : 'grey'}>{domain.status}</Badge> },
                          { label: t('pages.domain_detail.dns'), value: <Badge color={dnsColor}>{domain.last_dns_check_status || t('pages.domain_detail.unchecked')}</Badge> },
                          { label: t('pages.domain_detail.created'), value: new Date(domain.created_at).toLocaleDateString() },
                          ...(domain.last_dns_checked_at ? [{ label: t('pages.domain_detail.last_checked'), value: new Date(domain.last_dns_checked_at).toLocaleString() }] : []),
                        ]}
                      />
                    </Container>
                  </ColumnLayout>

                  {domain.last_dns_check_status !== 'pass' ? (
                    <Alert
                      type={domain.last_dns_check_status === 'fail' ? 'error' : 'warning'}
                      header={t('pages.domain_detail.dns_verification_required')}
                      action={<Button onClick={handleVerifyDNS} loading={verifying}>{t('pages.domain_detail.run_verification')}</Button>}
                    >
                      {t('pages.domain_detail.dns_verification_desc')}
                    </Alert>
                  ) : null}
                </SpaceBetween>
              ),
            },
            {
              id: 'dns',
              label: t('pages.domain_detail.dns_security_tab'),
              content: (
                <SpaceBetween size="l">
                  <Container
                    header={
                      <Header variant="h2" actions={<Button variant="primary" onClick={handleVerifyDNS} loading={verifying}>{t('pages.domain_detail.run_full_verification')}</Button>}>
                        {t('pages.domain_detail.dns_health_check')}
                      </Header>
                    }
                  >
                    <SpaceBetween size="m">
                      <StatusIndicator type={domain.last_dns_check_status === 'pass' ? 'success' : domain.last_dns_check_status === 'fail' ? 'error' : 'pending'}>
                        {t('pages.domain_detail.overall')} {domain.last_dns_check_status || t('pages.domain_detail.not_checked')}
                      </StatusIndicator>
                      <Box color="text-body-secondary">
                        {t('pages.domain_detail.dns_setup_desc')} <strong>{domain.name}</strong>.
                      </Box>
                      {domain.last_dns_checked_at ? (
                        <Box color="text-body-secondary" fontSize="body-s">
                          {t('pages.domain_detail.last_checked')}: {new Date(domain.last_dns_checked_at).toLocaleString()}
                        </Box>
                      ) : null}
                    </SpaceBetween>
                  </Container>

                  <Container header={<Header variant="h3">{t('pages.domain_detail.dkim_keys')}</Header>}>
                    <SpaceBetween size="s">
                      <Box color="text-body-secondary">{t('pages.domain_detail.manage_dkim_desc')}</Box>
                      <Button onClick={() => router.push(`/companies/${companyId}/security/dkim-keys`)}>
                        {t('pages.domain_detail.manage_dkim_btn')} →
                      </Button>
                    </SpaceBetween>
                  </Container>
                </SpaceBetween>
              ),
            },
            {
              id: 'users',
              label: `${t('pages.domain_detail.users_tab')} (${users.length})`,
              content: (
                <Table
                  columnDefinitions={[
                    { header: t('pages.domain_detail.username'), cell: (u: User) => u.username, width: '30%' },
                    { header: t('pages.domain_detail.display_name'), cell: (u: User) => u.display_name, width: '25%' },
                    { header: t('pages.domain_detail.status'), cell: (u: User) => <Badge color={u.status === 'active' ? 'green' : 'grey'}>{u.status}</Badge>, width: '15%' },
                    {
                      header: t('pages.domain_detail.storage_col'),
                      cell: (u: User) => u.quota_limit > 0
                        ? `${(u.quota_used / 1073741824).toFixed(1)} / ${(u.quota_limit / 1073741824).toFixed(1)} GB`
                        : `${(u.quota_used / 1073741824).toFixed(1)} GB`,
                      width: '20%',
                    },
                    { header: t('pages.domain_detail.joined'), cell: (u: User) => new Date(u.created_at).toLocaleDateString(), width: '10%' },
                  ]}
                  items={users}
                  header={
                    <Header variant="h2" counter={`(${users.length})`} actions={<Button variant="primary" onClick={() => router.push(`/companies/${companyId}/users`)}>{t('pages.domain_detail.add_user')}</Button>}>
                      {t('pages.domain_detail.users_in')} {domain.name}
                    </Header>
                  }
                  empty={
                    <Box textAlign="center" padding="l">
                      <SpaceBetween size="m" alignItems="center">
                        <StatusIndicator type="info">{t('pages.domain_detail.no_users')}</StatusIndicator>
                        <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/users`)}>{t('pages.domain_detail.create_first_user')}</Button>
                      </SpaceBetween>
                    </Box>
                  }
                />
              ),
            },
            {
              id: 'settings',
              label: `${t('pages.domain_detail.settings_tab')} (${settings.length})`,
              content: (
                <SpaceBetween size="l">
                  <Table
                    columnDefinitions={[
                      { header: t('pages.domain_detail.setting_key'), cell: (s: DomainSetting) => <Box fontWeight="bold">{s.Key}</Box>, width: '35%' },
                      { header: t('pages.domain_detail.setting_value'), cell: (s: DomainSetting) => typeof s.Value === 'object' ? JSON.stringify(s.Value) : String(s.Value ?? ''), width: '45%' },
                      { header: t('pages.domain_detail.updated'), cell: (s: DomainSetting) => s.UpdatedAt ? new Date(s.UpdatedAt).toLocaleDateString() : '—', width: '20%' },
                    ]}
                    items={settings}
                    header={
                      <Header variant="h2" counter={`(${settings.length})`} actions={<Button variant="primary" onClick={() => setShowAddSetting(true)}>{t('pages.domain_detail.add_setting_btn')}</Button>}>
                        {t('pages.domain_detail.domain_settings_title')}
                      </Header>
                    }
                    empty={<Box textAlign="center" padding="l"><Box color="text-body-secondary">{t('pages.domain_detail.no_custom_settings')}</Box></Box>}
                  />

                  <Modal
                    visible={showAddSetting}
                    onDismiss={() => setShowAddSetting(false)}
                    header={`${t('pages.domain_detail.add_setting_modal_header')} — ${domain.name}`}
                    footer={
                      <Box float="right">
                        <SpaceBetween direction="horizontal" size="xs">
                          <Button onClick={() => setShowAddSetting(false)}>{t('common.cancel')}</Button>
                          <Button variant="primary" onClick={handleAddSetting} loading={savingSetting} disabled={!newSetting.key.trim()}>
                            {t('pages.domain_detail.save_setting')}
                          </Button>
                        </SpaceBetween>
                      </Box>
                    }
                  >
                    <SpaceBetween size="m">
                      <FormField label={t('pages.domain_detail.key_label')} constraintText={t('pages.domain_detail.key_constraint')}>
                        <Input value={newSetting.key} onChange={(e) => setNewSetting({ ...newSetting, key: e.detail.value })} placeholder="setting_key" autoFocus />
                      </FormField>
                      <FormField label={t('pages.domain_detail.value_label')}>
                        <Input value={newSetting.value} onChange={(e) => setNewSetting({ ...newSetting, value: e.detail.value })} placeholder="value" />
                      </FormField>
                    </SpaceBetween>
                  </Modal>
                </SpaceBetween>
              ),
            },
          ]}
        />
      </ContentLayout>

      {/* Edit Modal */}
      <Modal
        visible={showEdit}
        onDismiss={() => { setShowEdit(false); setSaveError(''); }}
        header={`${t('common.edit') || '도메인 수정'} — ${domain.name}`}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => { setShowEdit(false); setSaveError(''); }}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={handleSaveEdit} loading={saving}>{t('common.save') || '저장'}</Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.domain_detail.status')}>
            <Select
              selectedOption={STATUS_OPTIONS.find(o => o.value === editForm.status) ?? STATUS_OPTIONS[0]}
              options={STATUS_OPTIONS}
              onChange={(e) => setEditForm({ ...editForm, status: e.detail.selectedOption.value ?? 'active' })}
            />
          </FormField>
          <FormField label={t('pages.tenancy_domains.storage_quota_gb') || '스토리지 할당량 (GB)'} description="0 = 무제한">
            <Input
              type="number"
              value={editForm.quota_gb}
              onChange={(e) => setEditForm({ ...editForm, quota_gb: e.detail.value })}
              placeholder="0 = 무제한"
            />
          </FormField>
          {saveError ? <Alert type="error">{saveError}</Alert> : null}
        </SpaceBetween>
      </Modal>

      {/* Delete Confirm Modal */}
      <Modal
        visible={showDelete}
        onDismiss={() => { setShowDelete(false); setDeleteError(''); }}
        header={t('common.delete') || '도메인 삭제'}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => { setShowDelete(false); setDeleteError(''); }}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={handleDelete} loading={deleting}>
                {t('common.delete') || '삭제'}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <Box>
            <strong>{domain.name}</strong> 도메인을 삭제하시겠습니까? 사용자가 있는 경우 삭제할 수 없습니다.
          </Box>
          {deleteError ? <Alert type="error">{deleteError}</Alert> : null}
        </SpaceBetween>
      </Modal>
    </>
  );
}
