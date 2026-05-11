'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Toggle,
  Select,
  SelectProps,
  Box,
  Spinner,
  Alert,
  Flashbar,
  FlashbarProps,
  ExpandableSection,
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface SMTPPolicy {
  tls_required: boolean;
  tls_min_version: string;
  starttls_enabled: boolean;
  dedicated_ip_enabled: boolean;
  dedicated_ips: string[];
  retry_count: number;
  retry_interval_minutes: number;
  connection_timeout_seconds: number;
  helo_hostname: string;
  bounce_address: string;
}

interface DomainOption {
  value: string;
  label: string;
}

const defaultPolicy = (): SMTPPolicy => ({
  tls_required: false,
  tls_min_version: 'tls1.2',
  starttls_enabled: true,
  dedicated_ip_enabled: false,
  dedicated_ips: [],
  retry_count: 3,
  retry_interval_minutes: 60,
  connection_timeout_seconds: 30,
  helo_hostname: '',
  bounce_address: '',
});

const TLS_VERSION_OPTIONS: SelectProps.Option[] = [
  { value: 'tls1.2', label: 'TLS 1.2' },
  { value: 'tls1.3', label: 'TLS 1.3' },
];

export default function SMTPPolicyPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const [domains, setDomains] = useState<DomainOption[]>([]);
  const [selectedDomain, setSelectedDomain] = useState<SelectProps.Option | null>(null);
  const [policy, setPolicy] = useState<SMTPPolicy>(defaultPolicy());
  const [loadingDomains, setLoadingDomains] = useState(true);
  const [loadingPolicy, setLoadingPolicy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);

  const fetchDomains = useCallback(async () => {
    setLoadingDomains(true);
    try {
      const res = await fetch(`/api/admin/domains?limit=100&company_id=${cid}`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        const opts: DomainOption[] = (data.domains ?? []).map((d: { id: string; name: string }) => ({
          value: d.id,
          label: d.name,
        }));
        setDomains(opts);
        if (opts.length > 0) {
          setSelectedDomain(opts[0]);
        }
      }
    } finally {
      setLoadingDomains(false);
    }
  }, [cid]);

  const fetchPolicy = useCallback(async (domainId: string) => {
    setLoadingPolicy(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/smtp-policy`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setPolicy({ ...defaultPolicy(), ...(data.policy ?? {}), dedicated_ips: data.policy?.dedicated_ips ?? [] });
      } else {
        setPolicy(defaultPolicy());
      }
    } finally {
      setLoadingPolicy(false);
    }
  }, []);

  useEffect(() => { fetchDomains(); }, [fetchDomains]);

  useEffect(() => {
    if (selectedDomain?.value) {
      fetchPolicy(selectedDomain.value);
    }
  }, [selectedDomain, fetchPolicy]);

  const handleSave = async () => {
    if (!selectedDomain?.value) return;
    setSaving(true);
    try {
      const res = await fetch(`/api/admin/domains/${selectedDomain.value}/smtp-policy`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
      if (res.ok) {
        setNotifications([{ type: 'success', content: t('pages.smtp_policy_page.save_success'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-ok' }]);
      } else {
        setNotifications([{ type: 'error', content: t('pages.smtp_policy_page.save_error'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-err' }]);
      }
    } finally {
      setSaving(false);
    }
  };

  const selectedTLSVersion = TLS_VERSION_OPTIONS.find(o => o.value === policy.tls_min_version) ?? TLS_VERSION_OPTIONS[0];

  if (loadingDomains) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.smtp_policy_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.smtp_policy_page.description')}>
          {t('pages.smtp_policy_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {notifications.length > 0 && <Flashbar items={notifications} />}

        {/* Domain selector */}
        <FormField label={t('pages.smtp_policy_page.domain_label')}>
          <Select
            selectedOption={selectedDomain}
            onChange={e => setSelectedDomain(e.detail.selectedOption)}
            options={domains}
            placeholder={t('pages.smtp_policy_page.select_domain')}
            empty={t('messages.no_data')}
          />
        </FormField>

        {!selectedDomain && (
          <Alert type="info">{t('pages.smtp_policy_page.select_domain')}</Alert>
        )}

        {selectedDomain && (loadingPolicy ? (
          <Box textAlign="center" padding="l"><Spinner /></Box>
        ) : (
          <>
            {/* TLS / Encryption */}
            <Container header={<Header variant="h2">{t('pages.smtp_policy_page.tls_section')}</Header>}>
              <SpaceBetween size="m">
                <FormField label={t('pages.smtp_policy_page.tls_required_label')} description={t('pages.smtp_policy_page.tls_required_desc')}>
                  <SpaceBetween size="xs">
                    <Toggle
                      checked={policy.tls_required}
                      onChange={e => setPolicy(p => ({ ...p, tls_required: e.detail.checked }))}
                    >
                      {policy.tls_required ? t('pages.smtp_policy_page.tls_required_label') : t('common.cancel')}
                    </Toggle>
                    {policy.tls_required && (
                      <Alert type="warning">{t('pages.smtp_policy_page.tls_required_warning')}</Alert>
                    )}
                  </SpaceBetween>
                </FormField>

                <FormField label={t('pages.smtp_policy_page.starttls_label')} description={t('pages.smtp_policy_page.starttls_desc')}>
                  <Toggle
                    checked={policy.starttls_enabled}
                    onChange={e => setPolicy(p => ({ ...p, starttls_enabled: e.detail.checked }))}
                  >
                    {policy.starttls_enabled ? 'Enabled' : 'Disabled'}
                  </Toggle>
                </FormField>

                <FormField label={t('pages.smtp_policy_page.tls_version_label')}>
                  <Select
                    selectedOption={selectedTLSVersion}
                    onChange={e => setPolicy(p => ({ ...p, tls_min_version: e.detail.selectedOption.value ?? 'tls1.2' }))}
                    options={TLS_VERSION_OPTIONS}
                  />
                </FormField>
              </SpaceBetween>
            </Container>

            {/* Delivery Settings */}
            <Container header={<Header variant="h2">{t('pages.smtp_policy_page.delivery_section')}</Header>}>
              <SpaceBetween size="m">
                <FormField
                  label={t('pages.smtp_policy_page.retry_count_label')}
                  constraintText={t('pages.smtp_policy_page.retry_count_hint')}
                >
                  <Input
                    type="number"
                    value={String(policy.retry_count)}
                    onChange={e => {
                      const v = parseInt(e.detail.value) || 1;
                      setPolicy(p => ({ ...p, retry_count: Math.max(1, Math.min(10, v)) }));
                    }}
                  />
                </FormField>

                <FormField
                  label={t('pages.smtp_policy_page.retry_interval_label')}
                  constraintText={t('pages.smtp_policy_page.retry_interval_hint')}
                >
                  <Input
                    type="number"
                    value={String(policy.retry_interval_minutes)}
                    onChange={e => setPolicy(p => ({ ...p, retry_interval_minutes: parseInt(e.detail.value) || 0 }))}
                  />
                </FormField>

                <FormField
                  label={t('pages.smtp_policy_page.connection_timeout_label')}
                  constraintText={t('pages.smtp_policy_page.connection_timeout_hint')}
                >
                  <Input
                    type="number"
                    value={String(policy.connection_timeout_seconds)}
                    onChange={e => setPolicy(p => ({ ...p, connection_timeout_seconds: parseInt(e.detail.value) || 0 }))}
                  />
                </FormField>
              </SpaceBetween>
            </Container>

            {/* Identity */}
            <Container header={<Header variant="h2">{t('pages.smtp_policy_page.identity_section')}</Header>}>
              <SpaceBetween size="m">
                <FormField
                  label={t('pages.smtp_policy_page.helo_hostname_label')}
                  constraintText={t('pages.smtp_policy_page.helo_hostname_hint')}
                >
                  <Input
                    value={policy.helo_hostname}
                    onChange={e => setPolicy(p => ({ ...p, helo_hostname: e.detail.value }))}
                    placeholder="mail.example.com"
                  />
                </FormField>

                <FormField
                  label={t('pages.smtp_policy_page.bounce_address_label')}
                  constraintText={t('pages.smtp_policy_page.bounce_address_hint')}
                >
                  <Input
                    value={policy.bounce_address}
                    onChange={e => setPolicy(p => ({ ...p, bounce_address: e.detail.value }))}
                    placeholder="bounces@example.com"
                  />
                </FormField>
              </SpaceBetween>
            </Container>

            {/* Dedicated IPs */}
            <ExpandableSection headerText={t('pages.smtp_policy_page.dedicated_ip_section')}>
              <SpaceBetween size="m">
                <FormField label={t('pages.smtp_policy_page.dedicated_ip_label')} description={t('pages.smtp_policy_page.dedicated_ip_desc')}>
                  <Toggle
                    checked={policy.dedicated_ip_enabled}
                    onChange={e => setPolicy(p => ({ ...p, dedicated_ip_enabled: e.detail.checked }))}
                  >
                    {policy.dedicated_ip_enabled ? 'Enabled' : 'Disabled'}
                  </Toggle>
                </FormField>

                {policy.dedicated_ip_enabled && (
                  <>
                    <FormField label={t('pages.smtp_policy_page.dedicated_ip_list_label')}>
                      {policy.dedicated_ips.length === 0 ? (
                        <Box color="text-body-secondary" fontSize="body-s">
                          {t('messages.no_data')}
                        </Box>
                      ) : (
                        <SpaceBetween direction="horizontal" size="xs">
                          {policy.dedicated_ips.map((ip, i) => (
                            <Badge key={i} color="blue">{ip}</Badge>
                          ))}
                        </SpaceBetween>
                      )}
                    </FormField>
                    <Alert type="info">{t('pages.smtp_policy_page.dedicated_ip_info')}</Alert>
                  </>
                )}
              </SpaceBetween>
            </ExpandableSection>

            <Box float="right">
              <Button variant="primary" onClick={handleSave} loading={saving}>
                {t('pages.smtp_policy_page.save')}
              </Button>
            </Box>
          </>
        ))}
      </SpaceBetween>
    </ContentLayout>
  );
}
