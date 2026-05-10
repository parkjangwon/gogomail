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
  id: string;
  setting_key: string;
  setting_value: string;
  last_updated: string;
}

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
  const [showAddSetting, setShowAddSetting] = useState(false);
  const [newSetting, setNewSetting] = useState({ key: '', value: '' });
  const [savingSetting, setSavingSetting] = useState(false);

  useEffect(() => {
    Promise.all([
      fetch(`/api/admin/domains/${domainId}`, { credentials: 'include' }).then(r => r.ok ? r.json() : null),
      fetch(`/api/admin/users?domain_id=${domainId}&limit=100`, { credentials: 'include' }).then(r => r.ok ? r.json() : { users: [] }),
      fetch(`/api/admin/domain-settings?domain=${domainId}&limit=100`, { credentials: 'include' }).then(r => r.ok ? r.json() : { settings: [] }),
    ]).then(([domainData, usersData, settingsData]) => {
      if (domainData?.domain) setDomain(domainData.domain);
      setUsers(usersData.users || []);
      setSettings(settingsData.settings || []);
    }).catch(() => {}).finally(() => setLoading(false));
  }, [domainId]);

  const handleVerifyDNS = async () => {
    setVerifying(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/verify-dns`, {
        method: 'POST',
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setDomain(prev => prev ? { ...prev, last_dns_check_status: data.status ?? prev.last_dns_check_status } : prev);
      }
    } finally {
      setVerifying(false);
    }
  };

  const handleAddSetting = async () => {
    if (!newSetting.key.trim()) return;
    setSavingSetting(true);
    try {
      const res = await fetch('/api/admin/domain-settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain_name: domain?.name, setting_key: newSetting.key, setting_value: newSetting.value }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowAddSetting(false);
        setNewSetting({ key: '', value: '' });
        const r = await fetch(`/api/admin/domain-settings?domain=${domainId}&limit=100`, { credentials: 'include' });
        if (r.ok) { const d = await r.json(); setSettings(d.settings || []); }
      }
    } finally {
      setSavingSetting(false);
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
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={
            <SpaceBetween direction="horizontal" size="xs">
              <span>{t('pages.domain_detail.company')}: </span>
              <Button
                variant="inline-link"
                onClick={() => router.push(`/companies/${companyId}`)}
              >
                {domain.company_name || domain.company_id}
              </Button>
            </SpaceBetween>
          }
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button
                onClick={handleVerifyDNS}
                loading={verifying}
                disabled={domain.last_dns_check_status === 'pass'}
              >
                {t('pages.domain_detail.verify_dns')}
              </Button>
              <Button onClick={() => router.push(`/companies/${companyId}/users?domain=${domain.name}`)}>
                {t('pages.domain_detail.add_user')}
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
        onChange={(e) => setActiveTab(e.detail.activeTabId)}
        tabs={[
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
                      {domain.over_allocated && (
                        <StatusIndicator type="error">{t('pages.domain_detail.over_allocated')}</StatusIndicator>
                      )}
                    </SpaceBetween>
                  </Container>

                  <Container header={<Header variant="h3">{t('pages.domain_detail.users')}</Header>}>
                    <SpaceBetween size="s">
                      <Box fontSize="display-l" fontWeight="bold">{users.length}</Box>
                      <Box color="text-body-secondary" fontSize="body-s">
                        {users.filter(u => u.status === 'active').length} {t('pages.domain_detail.active')}
                      </Box>
                      <Button
                        variant="inline-link"
                        onClick={() => setActiveTab('users')}
                      >
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

                {domain.last_dns_check_status !== 'pass' && (
                  <Alert
                    type={domain.last_dns_check_status === 'fail' ? 'error' : 'warning'}
                    header={t('pages.domain_detail.dns_verification_required')}
                    action={
                      <Button onClick={handleVerifyDNS} loading={verifying}>
                        {t('pages.domain_detail.run_verification')}
                      </Button>
                    }
                  >
                    {t('pages.domain_detail.dns_verification_desc')}
                  </Alert>
                )}
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
                    <Header
                      variant="h2"
                      actions={
                        <Button
                          variant="primary"
                          onClick={handleVerifyDNS}
                          loading={verifying}
                        >
                          {t('pages.domain_detail.run_full_verification')}
                        </Button>
                      }
                    >
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
                    {domain.last_dns_checked_at && (
                      <Box color="text-body-secondary" fontSize="body-s">
                        {t('pages.domain_detail.last_checked')}: {new Date(domain.last_dns_checked_at).toLocaleString()}
                      </Box>
                    )}
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
                  {
                    header: t('pages.domain_detail.status'),
                    cell: (u: User) => <Badge color={u.status === 'active' ? 'green' : 'grey'}>{u.status}</Badge>,
                    width: '15%',
                  },
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
                  <Header
                    variant="h2"
                    counter={`(${users.length})`}
                    actions={
                      <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/users`)}>
                        {t('pages.domain_detail.add_user')}
                      </Button>
                    }
                  >
                    {t('pages.domain_detail.users_in')} {domain.name}
                  </Header>
                }
                empty={
                  <Box textAlign="center" padding="l">
                    <SpaceBetween size="m" alignItems="center">
                      <StatusIndicator type="info">{t('pages.domain_detail.no_users')}</StatusIndicator>
                      <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/users`)}>
                        {t('pages.domain_detail.create_first_user')}
                      </Button>
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
                    { header: t('pages.domain_detail.setting_key'), cell: (s: DomainSetting) => <Box fontWeight="bold">{s.setting_key}</Box>, width: '35%' },
                    { header: t('pages.domain_detail.setting_value'), cell: (s: DomainSetting) => s.setting_value, width: '45%' },
                    { header: t('pages.domain_detail.updated'), cell: (s: DomainSetting) => new Date(s.last_updated).toLocaleDateString(), width: '20%' },
                  ]}
                  items={settings}
                  header={
                    <Header
                      variant="h2"
                      counter={`(${settings.length})`}
                      actions={
                        <Button variant="primary" onClick={() => setShowAddSetting(true)}>
                          {t('pages.domain_detail.add_setting_btn')}
                        </Button>
                      }
                    >
                      {t('pages.domain_detail.domain_settings_title')}
                    </Header>
                  }
                  empty={
                    <Box textAlign="center" padding="l">
                      <Box color="text-body-secondary">{t('pages.domain_detail.no_custom_settings')}</Box>
                    </Box>
                  }
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
                      <Input
                        value={newSetting.key}
                        onChange={(e) => setNewSetting({ ...newSetting, key: e.detail.value })}
                        placeholder="setting_key"
                        autoFocus
                      />
                    </FormField>
                    <FormField label={t('pages.domain_detail.value_label')}>
                      <Input
                        value={newSetting.value}
                        onChange={(e) => setNewSetting({ ...newSetting, value: e.detail.value })}
                        placeholder="value"
                      />
                    </FormField>
                  </SpaceBetween>
                </Modal>
              </SpaceBetween>
            ),
          },
        ]}
      />
    </ContentLayout>
  );
}
