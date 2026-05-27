'use client';
import {
  ContentLayout,
  Header,
  Tabs,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Button,
  Alert,
  FormField,
  Input,
  Modal,
  Select,
} from '@cloudscape-design/components';
import { useRouter } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useState } from 'react';
import { useDomainDetail } from './useDomainDetail';
import { DomainOverviewTab } from './DomainOverviewTab';
import { DomainUsersTab } from './DomainUsersTab';
import { DomainSettingsTab } from './DomainSettingsTab';
import { DomainStatsTab } from './DomainStatsTab';
import { DomainDNSTab } from './DomainDNSTab';
import { DomainMCPTab } from './DomainMCPTab';

const STATUS_OPTIONS = [
  { label: 'active', value: 'active' },
  { label: 'suspended', value: 'suspended' },
];

export default function DomainDetailPage() {
  const { t } = useI18n();
  const router = useRouter();
  const [activeTab, setActiveTab] = useState('overview');
  const h = useDomainDetail();

  if (h.loading) return <ContentLayout header={<Header variant="h1">{t('pages.domain_detail.title')}</Header>}><Box textAlign="center" padding="xl"><Spinner size="large" /></Box></ContentLayout>;
  if (h.loadError) return <ContentLayout header={<Header variant="h1">{t('pages.domain_detail.title')}</Header>}><Alert type="error">{h.loadError}</Alert></ContentLayout>;
  if (!h.domain) return <ContentLayout header={<Header variant="h1">{t('pages.domain_detail.not_found')}</Header>}><Alert type="error">{t('pages.domain_detail.title')} {h.domainId}</Alert></ContentLayout>;

  const { domain } = h;
  const dnsColor = domain.last_dns_check_status === 'pass' ? 'green' : domain.last_dns_check_status === 'fail' ? 'red' : 'grey';

  return (
    <>
      <ContentLayout
        header={
          <Header
            variant="h1"
            description={<><span>{t('pages.domain_detail.company')}: </span><Button variant="inline-link" onClick={() => router.push(`/companies/${h.companyId}`)}>{domain.company_name || domain.company_id}</Button></>}
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => router.push(`/companies/${h.companyId}/tenancy/domains`)}>← {t('pages.domain_detail.back_to_list') || '도메인 목록'}</Button>
                <Button onClick={h.handleVerifyDNS} loading={h.verifying}>{t('pages.domain_detail.verify_dns')}</Button>
                <Button onClick={() => h.setShowEdit(true)}>{t('common.edit') || '수정'}</Button>
                <Button variant="normal" onClick={() => h.setShowDelete(true)}>{t('common.delete') || '삭제'}</Button>
              </SpaceBetween>
            }
          >
            <SpaceBetween direction="horizontal" size="xs">
              <span>{domain.name}</span>
              <Badge color={domain.status === 'active' ? 'green' : 'grey'}>{domain.status}</Badge>
              <Badge color={dnsColor as 'green' | 'red' | 'grey'}>{t('pages.domain_detail.dns')}: {domain.last_dns_check_status || t('pages.domain_detail.unchecked')}</Badge>
            </SpaceBetween>
          </Header>
        }
      >
        <Tabs
          activeTabId={activeTab}
          onChange={(e) => { setActiveTab(e.detail.activeTabId); if (e.detail.activeTabId === 'mail-stats') h.fetchMailStats(domain.name); }}
          tabs={[
            { id: 'overview', label: t('pages.domain_detail.overview_tab'), content: <DomainOverviewTab domain={domain} users={h.users} verifying={h.verifying} onVerifyDNS={h.handleVerifyDNS} onSetActiveTab={setActiveTab} t={t} /> },
            { id: 'users', label: `${t('pages.domain_detail.users_tab')} (${h.users.length})`, content: <DomainUsersTab users={h.users} companyId={h.companyId} domainName={domain.name} t={t} /> },
            {
              id: 'settings',
              label: `${t('pages.domain_detail.settings_tab')} (${h.settings.length})`,
              content: <DomainSettingsTab settings={h.settings} domainName={domain.name} showAddSetting={h.showAddSetting} onShowAddSetting={h.setShowAddSetting} newSetting={h.newSetting} onNewSettingChange={h.setNewSetting} savingSetting={h.savingSetting} onAddSetting={h.handleAddSetting} t={t} />,
            },
            {
              id: 'mail-stats',
              label: t('pages.domain_detail.mail_stats'),
              content: <DomainStatsTab mailStats={h.mailStats} statsLoading={h.statsLoading} statsFetched={h.statsFetched} onFetchStats={() => h.fetchMailStats(domain.name, true)} t={t} />,
            },
            { id: 'dns', label: t('pages.domain_detail.dns_security_tab'), content: <DomainDNSTab domain={domain} companyId={h.companyId} verifying={h.verifying} onVerifyDNS={h.handleVerifyDNS} t={t} /> },
            {
              id: 'mcp-policy',
              label: t('pages.domain_detail.mcp_policy_tab', 'MCP Policy'),
              content: <DomainMCPTab mcpPolicy={h.mcpPolicy} mcpPolicyConfig={h.mcpPolicyConfig} mcpPolicyLoading={h.mcpPolicyLoading} mcpPolicySaving={h.mcpPolicySaving} mcpPolicyError={h.mcpPolicyError} mcpPolicySaved={h.mcpPolicySaved} onPolicyChange={h.updateMCPPolicy} onScopeChange={h.setMCPPolicyScope} onRefresh={h.refreshMCPPolicy} onSave={h.handleSaveMCPPolicy} onDismissError={() => h.setMcpPolicyError('')} onDismissSaved={() => h.setMcpPolicySaved(false)} t={t} />,
            },
          ]}
        />
      </ContentLayout>

      <Modal visible={h.showEdit} onDismiss={() => { h.setShowEdit(false); h.setSaveError(''); }} header={`${t('common.edit')} — ${domain.name}`}
        footer={<Box float="right"><SpaceBetween direction="horizontal" size="xs"><Button onClick={() => { h.setShowEdit(false); h.setSaveError(''); }}>{t('common.cancel')}</Button><Button variant="primary" onClick={h.handleSaveEdit} loading={h.saving}>{t('common.save')}</Button></SpaceBetween></Box>}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.domain_detail.status')}><Select selectedOption={STATUS_OPTIONS.find(o => o.value === h.editForm.status) ?? STATUS_OPTIONS[0]} options={STATUS_OPTIONS} onChange={(e) => h.setEditForm({ ...h.editForm, status: e.detail.selectedOption.value ?? 'active' })} /></FormField>
          <FormField label={t('pages.tenancy_domains.storage_quota_gb')} description={t('pages.tenancy_domains.quota_zero_unlimited')}><Input type="number" value={h.editForm.quota_gb} onChange={(e) => h.setEditForm({ ...h.editForm, quota_gb: e.detail.value })} placeholder={t('pages.tenancy_domains.quota_zero_unlimited')} /></FormField>
          {h.saveError ? <Alert type="error">{h.saveError}</Alert> : null}
        </SpaceBetween>
      </Modal>

      <Modal visible={h.showDelete} onDismiss={() => { h.setShowDelete(false); h.setDeleteError(''); }} header={t('common.delete')}
        footer={<Box float="right"><SpaceBetween direction="horizontal" size="xs"><Button onClick={() => { h.setShowDelete(false); h.setDeleteError(''); }}>{t('common.cancel')}</Button><Button variant="primary" onClick={h.handleDelete} loading={h.deleting}>{t('common.delete') || '삭제'}</Button></SpaceBetween></Box>}
      >
        <SpaceBetween size="m">
          <Box><strong>{domain.name}</strong> 도메인을 삭제하시겠습니까? 사용자가 있는 경우 삭제할 수 없습니다.</Box>
          {h.deleteError ? <Alert type="error">{h.deleteError}</Alert> : null}
        </SpaceBetween>
      </Modal>
    </>
  );
}
