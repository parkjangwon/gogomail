'use client';

import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Toggle,
  FormField,
  Input,
  Checkbox,
  RadioGroup,
  Container,
} from '@cloudscape-design/components';
import type { RadioGroupProps } from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface IPAccessPolicy {
  enabled: boolean;
  allowlist: string[];
  denylist: string[];
  protocols: string[];
  action: string;
}

const ALL_PROTOCOLS = ['smtp', 'imap', 'api'];

export default function IPAccessPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [policy, setPolicy] = useState<IPAccessPolicy>({
    enabled: false,
    allowlist: [],
    denylist: [],
    protocols: ['smtp', 'imap', 'api'],
    action: 'deny',
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [newAllowIP, setNewAllowIP] = useState('');
  const [newDenyIP, setNewDenyIP] = useState('');

  useEffect(() => {
    fetchPolicy();
  }, [companyId]);

  const fetchPolicy = async () => {
    if (!companyId) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/security/ip-policy`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setPolicy(data.policy);
      }
    } catch {
      // mutation error handled by caller
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await fetch(`/api/admin/companies/${companyId}/security/ip-policy`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
    } catch {
      // mutation error handled by caller
    } finally {
      setSaving(false);
    }
  };

  const addAllowIP = () => {
    const ip = newAllowIP.trim();
    if (!ip || policy.allowlist.includes(ip)) return;
    setPolicy({ ...policy, allowlist: [...policy.allowlist, ip] });
    setNewAllowIP('');
  };

  const removeAllowIP = (ip: string) => {
    setPolicy({ ...policy, allowlist: policy.allowlist.filter(x => x !== ip) });
  };

  const addDenyIP = () => {
    const ip = newDenyIP.trim();
    if (!ip || policy.denylist.includes(ip)) return;
    setPolicy({ ...policy, denylist: [...policy.denylist, ip] });
    setNewDenyIP('');
  };

  const removeDenyIP = (ip: string) => {
    setPolicy({ ...policy, denylist: policy.denylist.filter(x => x !== ip) });
  };

  const toggleProtocol = (proto: string, checked: boolean) => {
    const protocols = checked
      ? [...policy.protocols, proto]
      : policy.protocols.filter(p => p !== proto);
    setPolicy({ ...policy, protocols });
  };

  const actionOptions: RadioGroupProps.RadioButtonDefinition[] = [
    { value: 'deny', label: t('pages.ip_access_page.action_deny') },
    { value: 'log', label: t('pages.ip_access_page.action_log') },
  ];

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.ip_access_page.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.ip_access_page.description')}
          actions={
            <Button variant="primary" onClick={handleSave} loading={saving}>
              {t('pages.ip_access_page.save')}
            </Button>
          }
        >
          {t('pages.ip_access_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h2">{t('pages.ip_access_page.scope_company')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.ip_access_page.enabled_label')}>
              <Toggle
                checked={policy.enabled}
                onChange={(e) => setPolicy({ ...policy, enabled: e.detail.checked })}
              >
                {policy.enabled ? 'Enabled' : 'Disabled'}
              </Toggle>
            </FormField>

            <FormField
              label={t('pages.ip_access_page.allowlist_header')}
              description={t('pages.ip_access_page.allowlist_desc')}
            >
              <SpaceBetween size="xs">
                {policy.allowlist.map(ip => (
                  <Box key={ip} display="inline-block">
                    <SpaceBetween direction="horizontal" size="xs">
                      <Box>{ip}</Box>
                      <Button variant="inline-link" onClick={() => removeAllowIP(ip)}>
                        {t('common.delete')}
                      </Button>
                    </SpaceBetween>
                  </Box>
                ))}
                <SpaceBetween direction="horizontal" size="xs">
                  <Input
                    value={newAllowIP}
                    onChange={(e) => setNewAllowIP(e.detail.value)}
                    placeholder={t('pages.ip_access_page.ip_placeholder')}
                  />
                  <Button onClick={addAllowIP}>{t('pages.ip_access_page.add_ip')}</Button>
                </SpaceBetween>
              </SpaceBetween>
            </FormField>

            <FormField
              label={t('pages.ip_access_page.denylist_header')}
              description={t('pages.ip_access_page.denylist_desc')}
            >
              <SpaceBetween size="xs">
                {policy.denylist.map(ip => (
                  <Box key={ip} display="inline-block">
                    <SpaceBetween direction="horizontal" size="xs">
                      <Box>{ip}</Box>
                      <Button variant="inline-link" onClick={() => removeDenyIP(ip)}>
                        {t('common.delete')}
                      </Button>
                    </SpaceBetween>
                  </Box>
                ))}
                <SpaceBetween direction="horizontal" size="xs">
                  <Input
                    value={newDenyIP}
                    onChange={(e) => setNewDenyIP(e.detail.value)}
                    placeholder={t('pages.ip_access_page.ip_placeholder')}
                  />
                  <Button onClick={addDenyIP}>{t('pages.ip_access_page.add_ip')}</Button>
                </SpaceBetween>
              </SpaceBetween>
            </FormField>

            <FormField label={t('pages.ip_access_page.protocols_label')}>
              <SpaceBetween direction="horizontal" size="m">
                {ALL_PROTOCOLS.map(proto => (
                  <Checkbox
                    key={proto}
                    checked={policy.protocols.includes(proto)}
                    onChange={(e) => toggleProtocol(proto, e.detail.checked)}
                  >
                    {proto.toUpperCase()}
                  </Checkbox>
                ))}
              </SpaceBetween>
            </FormField>

            <FormField label={t('pages.ip_access_page.action_label')}>
              <RadioGroup
                value={policy.action}
                items={actionOptions}
                onChange={(e) => setPolicy({ ...policy, action: e.detail.value })}
              />
            </FormField>
          </SpaceBetween>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
