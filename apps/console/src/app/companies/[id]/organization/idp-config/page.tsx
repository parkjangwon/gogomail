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
  Modal,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useDomains } from '@/hooks';
import { api } from '@/lib/api-client';
import { useCompany } from '@/contexts/CompanyContext';

type ProviderType = 'database' | 'ldap' | 'azure_ad' | 'external_rdbms';

interface IdPConfig {
  domain_id: string;
  provider_type: ProviderType;
  settings: Record<string, unknown>;
}

// Write-only secret keys. The backend redacts these on GET (returning the
// SECRET_SET_INDICATOR sentinel when a value is stored) and merges on PUT so a
// blank/sentinel value preserves the stored secret. The client mirrors that
// contract: it never treats the sentinel as a real value and only sends a
// secret when the admin actually types one.
const SECRET_KEYS = ['bind_password', 'client_secret', 'dsn'] as const;
const SECRET_SET_INDICATOR = '__set__';

const PROVIDER_OPTIONS: SelectProps.Option[] = [
  { value: 'database', label: 'Local Database' },
  { value: 'ldap', label: 'LDAP / Active Directory' },
  { value: 'azure_ad', label: 'Azure AD' },
  { value: 'external_rdbms', label: 'External RDBMS' },
];

const defaultSettings = (provider: ProviderType): Record<string, unknown> => {
  switch (provider) {
    case 'ldap':
      return { host: '', port: 389, bind_dn: '', base_dn: '', user_filter: '(objectClass=person)', sync_interval_minutes: 60 };
    case 'azure_ad':
      return { tenant_id: '', client_id: '', sync_interval_minutes: 60 };
    case 'external_rdbms':
      return { query: '', sync_interval_minutes: 60 };
    default:
      return {};
  }
};

export default function IdPConfigPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { canMutate } = useCompany();

  const domainsQuery = useDomains(companyId);
  const domains = useMemo(() => domainsQuery.data ?? [], [domainsQuery.data]);

  // Explicit domain selection; defaults to the first domain, but the admin can
  // change it in a multi-domain company.
  const [domainId, setDomainId] = useState<string>('');
  useEffect(() => {
    if (!domainId && domains.length > 0) {
      setDomainId(domains[0].id);
    }
  }, [domains, domainId]);

  const [providerType, setProviderType] = useState<ProviderType>('database');
  const [settings, setSettings] = useState<Record<string, unknown>>({});
  // Tracks which secret keys the backend reports as already set, and which the
  // admin has typed a new value for this session.
  const [secretIsSet, setSecretIsSet] = useState<Record<string, boolean>>({});
  const [secretTouched, setSecretTouched] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [loadedOk, setLoadedOk] = useState(false);
  const [saving, setSaving] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [portError, setPortError] = useState<string | undefined>(undefined);
  const [resetConfirmVisible, setResetConfirmVisible] = useState(false);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);

  const addNotif = useCallback((type: FlashbarProps.MessageDefinition['type'], content: string) => {
    const id = Date.now().toString();
    setNotifications([{ type, content, dismissible: true, onDismiss: () => setNotifications([]), id }]);
  }, []);

  const loadConfig = useCallback(() => {
    if (!domainId) return;
    setLoading(true);
    setLoadError(false);
    setLoadedOk(false);
    api.get<IdPConfig>(`/domains/${domainId}/idp-config`)
      .then(data => {
        const provider = data.provider_type ?? 'database';
        const incoming = data.settings ?? {};
        // Detect which secrets are already set (sentinel from the backend) and
        // strip the sentinel from local state so we never echo it back as a
        // real value.
        const isSet: Record<string, boolean> = {};
        const cleaned: Record<string, unknown> = { ...incoming };
        for (const key of SECRET_KEYS) {
          const v = incoming[key];
          isSet[key] = v === SECRET_SET_INDICATOR || (typeof v === 'string' && v !== '');
          delete cleaned[key];
        }
        setProviderType(provider);
        setSettings(cleaned);
        setSecretIsSet(isSet);
        setSecretTouched({});
        setLoadedOk(true);
      })
      .catch(() => {
        // A load failure must NOT be silently treated as "no config" — that
        // would let a save overwrite the real config with defaults. Surface an
        // error and disable saving until a successful load.
        setLoadError(true);
        setLoadedOk(false);
      })
      .finally(() => setLoading(false));
  }, [domainId]);

  useEffect(() => { loadConfig(); }, [loadConfig]);

  const handleDomainChange = (newDomainId: string) => {
    setDomainId(newDomainId);
  };

  const handleProviderChange = (newType: ProviderType) => {
    setProviderType(newType);
    setSettings(defaultSettings(newType));
    // Switching provider clears secret state; the new provider has its own
    // (as-yet unset) secrets.
    setSecretIsSet({});
    setSecretTouched({});
    setPortError(undefined);
  };

  // Validate LDAP port when present.
  const validatePort = (): boolean => {
    if (providerType !== 'ldap') return true;
    const raw = settings.port;
    const n = typeof raw === 'number' ? raw : parseInt(String(raw ?? ''), 10);
    if (!Number.isInteger(n) || n < 1 || n > 65535) {
      setPortError(t('pages.idp_config.port_invalid'));
      return false;
    }
    setPortError(undefined);
    return true;
  };

  // Build the settings payload: include only secrets the admin actually typed
  // this session, so unchanged secrets are never sent (the backend preserves
  // them). Never send the sentinel.
  const buildPayload = (): Record<string, unknown> => {
    const payload: Record<string, unknown> = { ...settings };
    for (const key of SECRET_KEYS) {
      if (secretTouched[key]) {
        payload[key] = settings[key] ?? '';
      } else {
        delete payload[key];
      }
    }
    return payload;
  };

  const handleSave = async () => {
    if (!domainId || !loadedOk) return;
    if (!validatePort()) return;
    setSaving(true);
    try {
      await api.put<IdPConfig>(`/domains/${domainId}/idp-config`, { provider_type: providerType, settings: buildPayload() });
      addNotif('success', t('pages.idp_config.save_success'));
      // Reload so secret indicators reflect the newly persisted state.
      loadConfig();
    } catch {
      addNotif('error', t('pages.idp_config.save_error'));
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    if (!domainId) return;
    setResetConfirmVisible(false);
    setResetting(true);
    try {
      await api.delete(`/domains/${domainId}/idp-config`);
      setProviderType('database');
      setSettings({});
      setSecretIsSet({});
      setSecretTouched({});
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

  // Secret field helpers.
  const secretValue = (key: string) => (secretTouched[key] ? ((settings[key] as string) ?? '') : '');
  const onSecretChange = (key: string, value: string) => {
    setSecretTouched(m => ({ ...m, [key]: true }));
    setSettings(s => ({ ...s, [key]: value }));
  };
  const secretDescription = (key: string) =>
    secretIsSet[key] && !secretTouched[key]
      ? t('pages.idp_config.secret_set')
      : t('pages.idp_config.write_only_hint');
  const secretPlaceholder = (key: string) =>
    secretIsSet[key] && !secretTouched[key] ? t('pages.idp_config.secret_placeholder_set') : '••••••••';
  const secretIndicator = (key: string) =>
    secretIsSet[key] && !secretTouched[key]
      ? <StatusIndicator type="success">{t('pages.idp_config.secret_set')}</StatusIndicator>
      : <StatusIndicator type="stopped">{t('pages.idp_config.secret_not_set')}</StatusIndicator>;

  if (loading || domainsQuery.isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.idp_config.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const selectedProvider = PROVIDER_OPTIONS.find(o => o.value === providerType) ?? PROVIDER_OPTIONS[0];
  const domainOptions: SelectProps.Option[] = domains.map(d => ({ value: d.id, label: d.name }));
  const selectedDomain = domainOptions.find(o => o.value === domainId) ?? null;
  const saveDisabled = !loadedOk || loadError || !domainId || !canMutate;

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

        {domains.length === 0 ? (
          <Alert type="warning">{t('pages.idp_config.no_domains')}</Alert>
        ) : (
          <>
            {loadError && (
              <Alert
                type="error"
                header={t('pages.idp_config.load_error')}
                action={<Button onClick={loadConfig}>{t('pages.idp_config.retry')}</Button>}
              />
            )}

            <Container header={<Header variant="h2">{t('pages.idp_config.provider_section')}</Header>}>
              <SpaceBetween size="m">
                <FormField label={t('pages.idp_config.domain_label')} description={t('pages.idp_config.domain_desc')}>
                  <Select
                    selectedOption={selectedDomain}
                    onChange={e => handleDomainChange(e.detail.selectedOption.value ?? '')}
                    options={domainOptions}
                  />
                </FormField>

                <FormField label={t('pages.idp_config.provider_type_label')} description={t('pages.idp_config.provider_type_desc')}>
                  <Select
                    selectedOption={selectedProvider}
                    onChange={e => handleProviderChange((e.detail.selectedOption.value ?? 'database') as ProviderType)}
                    options={PROVIDER_OPTIONS}
                    disabled={saveDisabled}
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
                    <FormField label={t('pages.idp_config.ldap_port')} errorText={portError}>
                      <Input
                        type="number"
                        value={num('port')}
                        onChange={e => { set('port', e.detail.value === '' ? '' : parseInt(e.detail.value, 10)); setPortError(undefined); }}
                      />
                    </FormField>
                    <FormField label={t('pages.idp_config.ldap_bind_dn')}>
                      <Input value={str('bind_dn')} onChange={e => set('bind_dn', e.detail.value)} placeholder="cn=admin,dc=example,dc=com" />
                    </FormField>
                    <FormField
                      label={t('pages.idp_config.ldap_bind_password')}
                      description={secretDescription('bind_password')}
                      secondaryControl={secretIndicator('bind_password')}
                    >
                      <Input type="password" value={secretValue('bind_password')} onChange={e => onSecretChange('bind_password', e.detail.value)} placeholder={secretPlaceholder('bind_password')} />
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
                    <FormField
                      label={t('pages.idp_config.azure_client_secret')}
                      description={secretDescription('client_secret')}
                      secondaryControl={secretIndicator('client_secret')}
                    >
                      <Input type="password" value={secretValue('client_secret')} onChange={e => onSecretChange('client_secret', e.detail.value)} placeholder={secretPlaceholder('client_secret')} />
                    </FormField>
                    <FormField label={t('pages.idp_config.sync_interval')}>
                      <Input type="number" value={num('sync_interval_minutes')} onChange={e => set('sync_interval_minutes', parseInt(e.detail.value) || 60)} />
                    </FormField>
                  </SpaceBetween>
                )}

                {providerType === 'external_rdbms' && (
                  <SpaceBetween size="m">
                    <FormField
                      label={t('pages.idp_config.rdbms_dsn')}
                      description={secretDescription('dsn')}
                      secondaryControl={secretIndicator('dsn')}
                    >
                      <Input type="password" value={secretValue('dsn')} onChange={e => onSecretChange('dsn', e.detail.value)} placeholder={secretPlaceholder('dsn')} />
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
                <Button onClick={() => setResetConfirmVisible(true)} loading={resetting} disabled={providerType === 'database' || saveDisabled}>
                  {t('pages.idp_config.reset_to_database')}
                </Button>
                <Button variant="primary" onClick={handleSave} loading={saving} disabled={saveDisabled}>
                  {t('pages.idp_config.save')}
                </Button>
              </SpaceBetween>
            </Box>
          </>
        )}
      </SpaceBetween>

      {/* Reset-to-database confirmation — destructive DELETE */}
      <Modal
        visible={resetConfirmVisible}
        onDismiss={() => setResetConfirmVisible(false)}
        header={t('pages.idp_config.reset_confirm_title')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="link" onClick={() => setResetConfirmVisible(false)}>
                {t('pages.idp_config.cancel')}
              </Button>
              <Button variant="primary" onClick={handleReset} loading={resetting}>
                {t('pages.idp_config.reset_confirm_action')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <Alert type="warning">{t('pages.idp_config.reset_confirm_body')}</Alert>
      </Modal>
    </ContentLayout>
  );
}
