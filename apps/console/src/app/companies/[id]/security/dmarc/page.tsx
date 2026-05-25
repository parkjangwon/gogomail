'use client';
import { DataTable } from '@/components/DataTable';


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
  RadioGroup,
  Box,
  Spinner,
  Alert,
  Flashbar,
  FlashbarProps,
  ColumnLayout,
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface Domain { id: string; name: string; }
interface DmarcSpfPolicy {
  dmarc_policy: string;
  dmarc_pct: number;
  dmarc_rua: string;
  dmarc_ruf: string;
  dmarc_subdomains: string;
  dmarc_align_mode: string;
  spf_includes: string[];
  spf_all_mechanism: string;
  spf_ip4_list: string[];
}
interface GeneratedRecords {
  dmarc: string;
  spf: string;
  dmarc_host: string;
  spf_host: string;
}

const defaultPolicy = (): DmarcSpfPolicy => ({
  dmarc_policy: 'none',
  dmarc_pct: 100,
  dmarc_rua: '',
  dmarc_ruf: '',
  dmarc_subdomains: 'none',
  dmarc_align_mode: 'r',
  spf_includes: [],
  spf_all_mechanism: '~all',
  spf_ip4_list: [],
});

export default function DmarcSpfPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const [domains, setDomains] = useState<Domain[]>([]);
  const [selectedDomain, setSelectedDomain] = useState<SelectProps.Option | null>(null);
  const [policy, setPolicy] = useState<DmarcSpfPolicy>(defaultPolicy());
  const [generatedRecords, setGeneratedRecords] = useState<GeneratedRecords | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [newSpfInclude, setNewSpfInclude] = useState('');
  const [newIp4, setNewIp4] = useState('');
  const [checkingDns, setCheckingDns] = useState(false);

  useEffect(() => {
    fetch(`/api/admin/domains?limit=100&company_id=${cid}`, { credentials: 'include' })
      .then(r => r.ok ? r.json() : null)
      .then(data => {
        const list: Domain[] = data?.domains ?? [];
        setDomains(list);
        if (list.length > 0) {
          const opt = { label: list[0].name, value: list[0].id };
          setSelectedDomain(opt);
        }
      })
      .catch(() => { });
  }, [cid]);

  const fetchPolicy = useCallback(async (domainId: string) => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/security/dmarc-spf`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setPolicy({ ...defaultPolicy(), ...data.policy, spf_includes: data.policy?.spf_includes ?? [], spf_ip4_list: data.policy?.spf_ip4_list ?? [] });
        setGeneratedRecords(data.generated_records ?? null);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (selectedDomain?.value) fetchPolicy(selectedDomain.value);
  }, [selectedDomain, fetchPolicy]);

  const handleDnsCheck = async () => {
    if (!selectedDomain?.value) return;
    setCheckingDns(true);
    try {
      const res = await fetch(`/api/admin/domains/${selectedDomain.value}/dns-check`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        const check = data.dns_check;
        const passed = check?.status === 'ok' || check?.overall_status === 'pass';
        setNotifications([{
          type: passed ? 'success' : 'warning',
          content: passed
            ? t('pages.dmarc_page.dns_check_passed').replace('{domain}', String(selectedDomain.label))
            : t('pages.dmarc_page.dns_check_issues').replace('{domain}', String(selectedDomain.label)),
          dismissible: true,
          onDismiss: () => setNotifications([]),
          id: 'dns-check',
        }]);
      } else {
        setNotifications([{ type: 'error', content: t('pages.dmarc_page.dns_check_failed'), dismissible: true, onDismiss: () => setNotifications([]), id: 'dns-check-err' }]);
      }
    } finally {
      setCheckingDns(false);
    }
  };

  const handleSave = async () => {
    if (!selectedDomain?.value) return;
    setSaving(true);
    try {
      const res = await fetch(`/api/admin/domains/${selectedDomain.value}/security/dmarc-spf`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setGeneratedRecords(data.generated_records ?? null);
        setNotifications([{ type: 'success', content: t('pages.dmarc_page.save_success'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-ok' }]);
      } else {
        setNotifications([{ type: 'error', content: t('pages.dmarc_page.save_error'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-err' }]);
      }
    } finally {
      setSaving(false);
    }
  };

  const domainOptions: SelectProps.Option[] = domains.map(d => ({ label: d.name, value: d.id }));
  const domainName = selectedDomain?.label ?? '<domain>';

  const recordRows = generatedRecords ? [
    { type: 'TXT', host: `_dmarc.${domainName}`, value: generatedRecords.dmarc, label: 'DMARC' },
    { type: 'TXT', host: domainName, value: generatedRecords.spf, label: 'SPF' },
  ] : [];

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.dmarc_page.description')}>
          {t('pages.dmarc_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {notifications.length > 0 && <Flashbar items={notifications} />}

        <Container header={<Header variant="h2">{t('pages.dmarc_page.domain_selector')}</Header>}>
          <FormField label={t('pages.dmarc_page.select_domain')}>
            <Select
              selectedOption={selectedDomain}
              onChange={e => setSelectedDomain(e.detail.selectedOption)}
              options={domainOptions}
              placeholder={t('pages.dmarc_page.select_domain_placeholder')}
              empty={t('pages.dmarc_page.no_domains')}
            />
          </FormField>
        </Container>

        {loading ? (
          <Box textAlign="center" padding="xl"><Spinner /></Box>
        ) : selectedDomain && (
          <>
            {/* DMARC Policy */}
            <Container header={<Header variant="h2" description={t('pages.dmarc_page.dmarc_desc')}>{t('pages.dmarc_page.dmarc_section')}</Header>}>
              <SpaceBetween size="m">
                {policy.dmarc_policy === 'reject' && (
                  <Alert type="warning">{t('pages.dmarc_page.reject_warning')}</Alert>
                )}
                <ColumnLayout columns={2}>
                  <FormField label={t('pages.dmarc_page.dmarc_policy')} constraintText={t('pages.dmarc_page.dmarc_policy_hint')}>
                    <Select
                      selectedOption={{ label: policy.dmarc_policy, value: policy.dmarc_policy }}
                      onChange={e => setPolicy(p => ({ ...p, dmarc_policy: e.detail.selectedOption.value ?? 'none' }))}
                      options={[
                        { label: t('pages.dmarc_page.policy_none'), value: 'none' },
                        { label: t('pages.dmarc_page.policy_quarantine'), value: 'quarantine' },
                        { label: t('pages.dmarc_page.policy_reject'), value: 'reject' },
                      ]}
                    />
                  </FormField>
                  <FormField label={t('pages.dmarc_page.dmarc_pct')} constraintText="0–100">
                    <Input
                      type="number"
                      value={String(policy.dmarc_pct)}
                      onChange={e => setPolicy(p => ({ ...p, dmarc_pct: parseInt(e.detail.value) || 0 }))}
                    />
                  </FormField>
                  <FormField label={t('pages.dmarc_page.dmarc_rua')} constraintText={t('pages.dmarc_page.rua_hint')}>
                    <Input
                      value={policy.dmarc_rua}
                      onChange={e => setPolicy(p => ({ ...p, dmarc_rua: e.detail.value }))}
                      placeholder="dmarc-reports@example.com"
                    />
                  </FormField>
                  <FormField label={t('pages.dmarc_page.dmarc_ruf')} constraintText={t('pages.dmarc_page.ruf_hint')}>
                    <Input
                      value={policy.dmarc_ruf}
                      onChange={e => setPolicy(p => ({ ...p, dmarc_ruf: e.detail.value }))}
                      placeholder="dmarc-forensic@example.com"
                    />
                  </FormField>
                  <FormField label={t('pages.dmarc_page.subdomain_policy')}>
                    <Select
                      selectedOption={{ label: policy.dmarc_subdomains, value: policy.dmarc_subdomains }}
                      onChange={e => setPolicy(p => ({ ...p, dmarc_subdomains: e.detail.selectedOption.value ?? 'none' }))}
                      options={[
                        { label: 'none', value: 'none' },
                        { label: 'quarantine', value: 'quarantine' },
                        { label: 'reject', value: 'reject' },
                      ]}
                    />
                  </FormField>
                  <FormField label={t('pages.dmarc_page.align_mode')}>
                    <RadioGroup
                      value={policy.dmarc_align_mode}
                      onChange={e => setPolicy(p => ({ ...p, dmarc_align_mode: e.detail.value }))}
                      items={[
                        { value: 'r', label: t('pages.dmarc_page.relaxed') },
                        { value: 's', label: t('pages.dmarc_page.strict') },
                      ]}
                    />
                  </FormField>
                </ColumnLayout>
              </SpaceBetween>
            </Container>

            {/* SPF Policy */}
            <Container header={<Header variant="h2" description={t('pages.dmarc_page.spf_desc')}>{t('pages.dmarc_page.spf_section')}</Header>}>
              <SpaceBetween size="m">
                <Alert type="info">{t('pages.dmarc_page.spf_info')}</Alert>
                <FormField label={t('pages.dmarc_page.spf_includes')} description={t('pages.dmarc_page.spf_includes_desc')}>
                  <SpaceBetween size="xs">
                    {policy.spf_includes.map((inc, i) => (
                      <SpaceBetween key={i} direction="horizontal" size="xs">
                        <Badge color="blue">{`include:${inc}`}</Badge>
                        <Button variant="inline-link" onClick={() => setPolicy(p => ({ ...p, spf_includes: p.spf_includes.filter((_, j) => j !== i) }))}>
                          {t('common.delete')}
                        </Button>
                      </SpaceBetween>
                    ))}
                    <SpaceBetween direction="horizontal" size="xs">
                      <Input value={newSpfInclude} onChange={e => setNewSpfInclude(e.detail.value)} placeholder="mail.example.com" />
                      <Button onClick={() => { if (newSpfInclude.trim()) { setPolicy(p => ({ ...p, spf_includes: [...p.spf_includes, newSpfInclude.trim()] })); setNewSpfInclude(''); } }}>
                        {t('common.add')}
                      </Button>
                    </SpaceBetween>
                  </SpaceBetween>
                </FormField>
                <FormField label={t('pages.dmarc_page.spf_ip4')} description={t('pages.dmarc_page.spf_ip4_desc')}>
                  <SpaceBetween size="xs">
                    {policy.spf_ip4_list.map((ip, i) => (
                      <SpaceBetween key={i} direction="horizontal" size="xs">
                        <Badge color="grey">{`ip4:${ip}`}</Badge>
                        <Button variant="inline-link" onClick={() => setPolicy(p => ({ ...p, spf_ip4_list: p.spf_ip4_list.filter((_, j) => j !== i) }))}>
                          {t('common.delete')}
                        </Button>
                      </SpaceBetween>
                    ))}
                    <SpaceBetween direction="horizontal" size="xs">
                      <Input value={newIp4} onChange={e => setNewIp4(e.detail.value)} placeholder="192.168.1.0/24" />
                      <Button onClick={() => { if (newIp4.trim()) { setPolicy(p => ({ ...p, spf_ip4_list: [...p.spf_ip4_list, newIp4.trim()] })); setNewIp4(''); } }}>
                        {t('common.add')}
                      </Button>
                    </SpaceBetween>
                  </SpaceBetween>
                </FormField>
                <FormField label={t('pages.dmarc_page.all_mechanism')}>
                  <Select
                    selectedOption={{ label: policy.spf_all_mechanism, value: policy.spf_all_mechanism }}
                    onChange={e => setPolicy(p => ({ ...p, spf_all_mechanism: e.detail.selectedOption.value ?? '~all' }))}
                    options={[
                      { label: t('pages.dmarc_page.spf_softfail'), value: '~all' },
                      { label: t('pages.dmarc_page.spf_hardfail'), value: '-all' },
                      { label: t('pages.dmarc_page.spf_neutral'), value: '?all' },
                      { label: t('pages.dmarc_page.spf_pass_all'), value: '+all' },
                    ]}
                  />
                </FormField>
              </SpaceBetween>
            </Container>

            {/* Generated DNS Records */}
            {generatedRecords && (
              <Container header={<Header variant="h2" description={t('pages.dmarc_page.records_desc')}>{t('pages.dmarc_page.records_header')}</Header>}>
                <DataTable
                  columnDefinitions={[
                    { header: t('pages.dmarc_page.record_type'), cell: (r: typeof recordRows[0]) => <Badge color="blue">{r.type}</Badge>, width: '8%' },
                    { header: t('pages.dmarc_page.record_label'), cell: r => r.label, width: '10%' },
                    { header: t('pages.dmarc_page.record_host'), cell: r => <Box variant="code">{r.host}</Box>, width: '28%' },
                    {
                      header: t('pages.dmarc_page.record_value'),
                      cell: r => (
                        <SpaceBetween direction="horizontal" size="xs">
                          <Box variant="code" fontSize="body-s">{r.value.length > 60 ? r.value.slice(0, 60) + '…' : r.value}</Box>
                          <Button
                            variant="inline-link"
                            iconName="copy"
                            onClick={() => navigator.clipboard.writeText(r.value)}
                          >
                            {t('pages.dmarc_page.copy')}
                          </Button>
                        </SpaceBetween>
                      ),
                      width: '54%',
                    },
                  ]}
                  items={recordRows}
                  variant="embedded"
                />
              </Container>
            )}

            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={handleDnsCheck} loading={checkingDns} iconName="check">
                  {t('pages.dmarc_page.verify_dns')}
                </Button>
                <Button variant="primary" onClick={handleSave} loading={saving}>
                  {t('pages.dmarc_page.save')}
                </Button>
              </SpaceBetween>
            </Box>
          </>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
