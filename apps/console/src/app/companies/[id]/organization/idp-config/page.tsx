'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Select,
  SelectProps,
  Box,
  Spinner,
  Alert,
  Flashbar,
  FlashbarProps,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useDomains } from '@/hooks';
import { api } from '@/lib/api-client';

type ProviderType = 'database' | 'ldap' | 'azure_ad' | 'external_rdbms';

interface IdPConfig {
  domain_id: string;
  provider_type: ProviderType;
  settings: Record<string, unknown>;
}

const PROVIDER_OPTIONS: SelectProps.Option[] = [
  { value: 'database', label: 'Local Database' },
  { value: 'ldap', label: 'LDAP / Active Directory' },
  { value: 'azure_ad', label: 'Azure AD' },
  { value: 'external_rdbms', label: 'External RDBMS' },
];

const defaultSettings = (provider: ProviderType): Record<string, unknown> => {
  switch (provider) {
    case 'ldap':
      return { host: '', port: 389, bind_dn: '', bind_password: '', base_dn: '', user_filter: '(objectClass=person)', sync_interval_minutes: 60 };
    case 'azure_ad':
      return { tenant_id: '', client_id: '', client_secret: '', sync_interval_minutes: 60 };
    case 'external_rdbms':
      return { dsn: '', query: '', sync_interval_minutes: 60 };
    default:
      return {};
  }
};

export default function IdPConfigPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const domainsQuery = useDomains(companyId);
  const domainId = domainsQuery.data?.[0]?.id ?? '';

  const [providerType, setProviderType] = useState<ProviderType>('database');
  const [settings, setSettings] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);

  const addNotif = useCallback((type: FlashbarProps.MessageDefinition['type'], content: string) => {
    const id = Date.now().toString();
    setNotifications([{ type, content, dismissible: true, onDismiss: () => setNotifications([]), id }]);
  }, []);

  useEffect(() => {
    if (!domainId) return;
    setLoading(true);
    api.get<IdPConfig>(`/domains/${domainId}/idp-config`)
      .then(data => {
        setProviderType(data.provider_type ?? 'database');
        setSettings(data.settings ?? defaultSettings(data.provider_type ?? 'database'));
      })
      .catch(() => {
        setProviderType('database');
        setSettings({});
      })
      .finally(() => setLoading(false));
  }, [domainId]);

  const handleProviderChange = (newType: ProviderType) => {
    setProviderType(newType);
    setSettings(defaultSettings(newType));
  };

  const handleSave = async () => {
    if (!domainId) return;
    setSaving(true);
    try {
      await api.put<IdPConfig>(`/domains/${domainId}/idp-config`, { provider_type: providerType, settings });
      addNotif('success', t('pages.idp_config.save_success'));
    } catch {
      addNotif('error', t('pages.idp_config.save_error'));
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    if (!domainId) return;
    setResetting(true);
    try {
      await api.delete(`/domains/${domainId}/idp-config`);
      setProviderType('database');
      setSettings({});
      addNotif('success', t('pages.idp_config.reset_success'));
    } catch {
      addNotif('error', t('pages.idp_config.reset_error'));
    } finally {
      setResetting(false);
    }
  };

  const set = (key: string, value: unknown) => setSettings(s => ({ ...s, [key]: value }));
  const str = (key: string) => (settings[key] as string) ?? '';
  const num = (key: string) => String((settings[key] as number) ?? '');

  if (loading || domainsQuery.isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.idp_config.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const selectedProvider = PROVIDER_OPTIONS.find(o => o.value === providerType) ?? PROVIDER_OPTIONS[0];

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.idp_config.description')}>
          {t('pages.idp_config.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {notifications.length > 0 && <Flashbar items={notifications} />}

        <Container header={<Header variant="h2">{t('pages.idp_config.provider_section')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.idp_config.provider_type_label')} description={t('pages.idp_config.provider_type_desc')}>
              <Select
                selectedOption={selectedProvider}
                onChange={e => handleProviderChange((e.detail.selectedOption.value ?? 'database') as ProviderType)}
                options={PROVIDER_OPTIONS}
              />
            </FormField>

            {providerType === 'database' && (
              <Alert type="info">{t('pages.idp_config.database_hint')}</Alert>
            )}

            {providerType === 'ldap' && (
              <SpaceBetween size="m">
                <FormField label={t('pages.idp_config.ldap_host')}>
                  <Input value={str('host')} onChange={e => set('host', e.detail.value)} placeholder="ldap.example.com" />
                </FormField>
                <FormField label={t('pages.idp_config.ldap_port')}>
                  <Input type="number" value={num('port')} onChange={e => set('port', parseInt(e.detail.value) || 389)} />
                </FormField>
                <FormField label={t('pages.idp_config.ldap_bind_dn')}>
                  <Input value={str('bind_dn')} onChange={e => set('bind_dn', e.detail.value)} placeholder="cn=admin,dc=example,dc=com" />
                </FormField>
                <FormField label={t('pages.idp_config.ldap_bind_password')} description={t('pages.idp_config.write_only_hint')}>
                  <Input type="password" value={str('bind_password')} onChange={e => set('bind_password', e.detail.value)} placeholder="••••••••" />
                </FormField>
                <FormField label={t('pages.idp_config.ldap_base_dn')}>
                  <Input value={str('base_dn')} onChange={e => set('base_dn', e.detail.value)} placeholder="dc=example,dc=com" />
                </FormField>
                <FormField label={t('pages.idp_config.ldap_user_filter')}>
                  <Input value={str('user_filter')} onChange={e => set('user_filter', e.detail.value)} placeholder="(objectClass=person)" />
                </FormField>
                <FormField label={t('pages.idp_config.sync_interval')}>
                  <Input type="number" value={num('sync_interval_minutes')} onChange={e => set('sync_interval_minutes', parseInt(e.detail.value) || 60)} />
                </FormField>
              </SpaceBetween>
            )}

            {providerType === 'azure_ad' && (
              <SpaceBetween size="m">
                <FormField label={t('pages.idp_config.azure_tenant_id')}>
                  <Input value={str('tenant_id')} onChange={e => set('tenant_id', e.detail.value)} placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
                </FormField>
                <FormField label={t('pages.idp_config.azure_client_id')}>
                  <Input value={str('client_id')} onChange={e => set('client_id', e.detail.value)} placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
                </FormField>
                <FormField label={t('pages.idp_config.azure_client_secret')} description={t('pages.idp_config.write_only_hint')}>
                  <Input type="password" value={str('client_secret')} onChange={e => set('client_secret', e.detail.value)} placeholder="••••••••" />
                </FormField>
                <FormField label={t('pages.idp_config.sync_interval')}>
                  <Input type="number" value={num('sync_interval_minutes')} onChange={e => set('sync_interval_minutes', parseInt(e.detail.value) || 60)} />
                </FormField>
              </SpaceBetween>
            )}

            {providerType === 'external_rdbms' && (
              <SpaceBetween size="m">
                <FormField label={t('pages.idp_config.rdbms_dsn')} description={t('pages.idp_config.write_only_hint')}>
                  <Input type="password" value={str('dsn')} onChange={e => set('dsn', e.detail.value)} placeholder="postgres://user:pass@host/db" />
                </FormField>
                <FormField label={t('pages.idp_config.rdbms_query')} description={t('pages.idp_config.rdbms_query_desc')}>
                  <Input value={str('query')} onChange={e => set('query', e.detail.value)} placeholder="SELECT email, display_name FROM users" />
                </FormField>
                <FormField label={t('pages.idp_config.sync_interval')}>
                  <Input type="number" value={num('sync_interval_minutes')} onChange={e => set('sync_interval_minutes', parseInt(e.detail.value) || 60)} />
                </FormField>
              </SpaceBetween>
            )}
          </SpaceBetween>
        </Container>

        <Box float="right">
          <SpaceBetween direction="horizontal" size="xs">
            <Button onClick={handleReset} loading={resetting} disabled={providerType === 'database'}>
              {t('pages.idp_config.reset_to_database')}
            </Button>
            <Button variant="primary" onClick={handleSave} loading={saving}>
              {t('pages.idp_config.save')}
            </Button>
          </SpaceBetween>
        </Box>
      </SpaceBetween>
    </ContentLayout>
  );
}
