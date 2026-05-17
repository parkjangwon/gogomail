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
import { DataTable } from '@/components/DataTable';

interface SpamFilterPolicy {
  enabled: boolean;
  spam_threshold: number;
  virus_scan_enabled: boolean;
  strict_auth_enabled: boolean;
  rbl_check_enabled: boolean;
  rbl_reject_enabled: boolean;
  rbl_zones: string[];
  blocked_extensions: string[];
  blocked_senders: string[];
  allowed_senders: string[];
  quarantine_enabled: boolean;
  max_attachment_mb: number;
  bulk_recipient_limit: number;
}

interface SpamFilterEvent {
  id: string;
  created_at: string;
  from_addr?: string;
  mail_from?: string;
  rcpt_to?: string;
  subject?: string;
  flow_status: string;
  enhanced_status?: string;
  error_message?: string;
  spam_score?: number;
  spf_result?: string;
  dkim_result?: string;
  dmarc_result?: string;
}

interface SpamFilterStats {
  total_messages: number;
  filtered: number;
  rejected: number;
  delivered: number;
}

const defaultPolicy = (): SpamFilterPolicy => ({
  enabled: true,
  spam_threshold: 5,
  virus_scan_enabled: true,
  strict_auth_enabled: true,
  rbl_check_enabled: false,
  rbl_reject_enabled: true,
  rbl_zones: [],
  blocked_extensions: ['.exe', '.bat', '.cmd', '.scr', '.vbs', '.js', '.ps1', '.jar', '.docm', '.xlsm'],
  blocked_senders: [],
  allowed_senders: [],
  quarantine_enabled: true,
  max_attachment_mb: 25,
  bulk_recipient_limit: 50,
});

export default function SpamFilterPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const [policy, setPolicy] = useState<SpamFilterPolicy>(defaultPolicy());
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [events, setEvents] = useState<SpamFilterEvent[]>([]);
  const [stats, setStats] = useState<SpamFilterStats | null>(null);
  const [scopeDomainId, setScopeDomainId] = useState('');
  const [activeDomainId, setActiveDomainId] = useState('');

  const [newBlockedExt, setNewBlockedExt] = useState('');
  const [newBlockedSender, setNewBlockedSender] = useState('');
  const [newAllowedSender, setNewAllowedSender] = useState('');
  const [newRBLZone, setNewRBLZone] = useState('');

  const fetchPolicy = useCallback(async () => {
    setLoading(true);
    try {
      const domainId = activeDomainId.trim();
      const policyPath = domainId
        ? `/api/admin/domains/${encodeURIComponent(domainId)}/security/spam-filter`
        : `/api/admin/companies/${cid}/security/spam-filter`;
      const domainQuery = domainId ? `&domain_id=${encodeURIComponent(domainId)}` : '';
      const statsPath = domainId
        ? `/api/admin/companies/${cid}/security/spam-filter/stats?domain_id=${encodeURIComponent(domainId)}`
        : `/api/admin/companies/${cid}/security/spam-filter/stats`;
      const [policyRes, eventsRes, statsRes] = await Promise.all([
        fetch(policyPath, { credentials: 'include' }),
        fetch(`/api/admin/companies/${cid}/security/spam-filter/events?limit=25${domainQuery}`, { credentials: 'include' }),
        fetch(statsPath, { credentials: 'include' }),
      ]);
      if (policyRes.ok) {
        const data = await policyRes.json();
        const p = data.policy ?? {};
        setPolicy({
          ...defaultPolicy(),
          ...p,
          blocked_extensions: p.blocked_extensions ?? [],
          blocked_senders: p.blocked_senders ?? [],
          allowed_senders: p.allowed_senders ?? [],
          rbl_zones: p.rbl_zones ?? [],
        });
      }
      if (eventsRes.ok) {
        const data = await eventsRes.json();
        setEvents(data.spam_filter_events ?? []);
      }
      if (statsRes.ok) {
        const data = await statsRes.json();
        setStats(data.spam_filter_stats ?? null);
      }
    } finally {
      setLoading(false);
    }
  }, [cid, activeDomainId]);

  useEffect(() => { fetchPolicy(); }, [fetchPolicy]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const domainId = activeDomainId.trim();
      const policyPath = domainId
        ? `/api/admin/domains/${encodeURIComponent(domainId)}/security/spam-filter`
        : `/api/admin/companies/${cid}/security/spam-filter`;
      const res = await fetch(policyPath, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
      if (res.ok) {
        setNotifications([{ type: 'success', content: t('pages.spam_filter_page.save_success'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-ok' }]);
      } else {
        setNotifications([{ type: 'error', content: t('pages.spam_filter_page.save_error'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-err' }]);
      }
    } finally {
      setSaving(false);
    }
  };

  const addToList = (field: keyof SpamFilterPolicy, value: string, setter: (v: string) => void) => {
    const trimmed = value.trim();
    if (!trimmed) return;
    setPolicy(p => ({ ...p, [field]: [...(p[field] as string[]), trimmed] }));
    setter('');
  };

  const removeFromList = (field: keyof SpamFilterPolicy, index: number) => {
    setPolicy(p => ({ ...p, [field]: (p[field] as string[]).filter((_, i) => i !== index) }));
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.spam_filter_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.spam_filter_page.description')}>
          {t('pages.spam_filter_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {notifications.length > 0 && <Flashbar items={notifications} />}

        {/* Master toggle */}
        <Container header={<Header variant="h2">{t('pages.spam_filter_page.general_section')}</Header>}>
          <SpaceBetween size="m">
            <FormField
              label="Policy scope"
              description="Leave empty to manage the company default policy. Enter a domain ID to manage that domain override."
            >
              <SpaceBetween direction="horizontal" size="xs">
                <Input
                  value={scopeDomainId}
                  onChange={e => setScopeDomainId(e.detail.value)}
                  placeholder="domain id"
                />
                <Button onClick={() => setActiveDomainId(scopeDomainId.trim())}>Load scope</Button>
              </SpaceBetween>
            </FormField>
            <FormField label={t('pages.spam_filter_page.enabled_label')} description={t('pages.spam_filter_page.enabled_desc')}>
              <Toggle
                checked={policy.enabled}
                onChange={e => setPolicy(p => ({ ...p, enabled: e.detail.checked }))}
              >
                {policy.enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
              </Toggle>
            </FormField>
          </SpaceBetween>
        </Container>

        {policy.enabled && (
          <>
            <ColumnLayout columns={3} variant="text-grid" minColumnWidth={180}>
              <Container>
                <Box variant="awsui-key-label">Filtered</Box>
                <Box variant="h2">{(stats?.filtered ?? 0).toLocaleString()}</Box>
              </Container>
              <Container>
                <Box variant="awsui-key-label">Rejected</Box>
                <Box variant="h2">{(stats?.rejected ?? 0).toLocaleString()}</Box>
              </Container>
              <Container>
                <Box variant="awsui-key-label">Delivered</Box>
                <Box variant="h2">{(stats?.delivered ?? 0).toLocaleString()}</Box>
              </Container>
            </ColumnLayout>

            {/* Spam detection */}
            <Container header={<Header variant="h2">{t('pages.spam_filter_page.detection_section')}</Header>}>
              <SpaceBetween size="m">
                <ColumnLayout columns={2}>
                  <FormField
                    label={t('pages.spam_filter_page.threshold_label')}
                    constraintText={t('pages.spam_filter_page.threshold_hint')}
                  >
                    <Input
                      type="number"
                      value={String(policy.spam_threshold)}
                      onChange={e => {
                        const v = parseInt(e.detail.value) || 1;
                        setPolicy(p => ({ ...p, spam_threshold: Math.max(1, Math.min(10, v)) }));
                      }}
                    />
                  </FormField>
                  <FormField label={t('pages.spam_filter_page.virus_scan_label')} description={t('pages.spam_filter_page.virus_scan_desc')}>
                    <Toggle
                      checked={policy.virus_scan_enabled}
                      onChange={e => setPolicy(p => ({ ...p, virus_scan_enabled: e.detail.checked }))}
                    >
                      {policy.virus_scan_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
                    </Toggle>
                  </FormField>
                  <FormField label="Strict authentication" description="Score SPF, DKIM, and DMARC failures more aggressively.">
                    <Toggle
                      checked={policy.strict_auth_enabled}
                      onChange={e => setPolicy(p => ({ ...p, strict_auth_enabled: e.detail.checked }))}
                    >
                      {policy.strict_auth_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
                    </Toggle>
                  </FormField>
                  <FormField
                    label="Bulk recipient limit"
                    constraintText="Messages above this recipient count receive a stronger bulk-spam score."
                  >
                    <Input
                      type="number"
                      value={String(policy.bulk_recipient_limit)}
                      onChange={e => {
                        const v = parseInt(e.detail.value) || 1;
                        setPolicy(p => ({ ...p, bulk_recipient_limit: Math.max(1, Math.min(500, v)) }));
                      }}
                    />
                  </FormField>
                </ColumnLayout>

                <FormField label={t('pages.spam_filter_page.action_label')} description={t('pages.spam_filter_page.action_desc')}>
                  <RadioGroup
                    value={policy.quarantine_enabled ? 'quarantine' : 'reject'}
                    onChange={e => setPolicy(p => ({ ...p, quarantine_enabled: e.detail.value === 'quarantine' }))}
                    items={[
                      { value: 'quarantine', label: t('pages.spam_filter_page.action_quarantine') },
                      { value: 'reject', label: t('pages.spam_filter_page.action_reject') },
                    ]}
                  />
                </FormField>
              </SpaceBetween>
            </Container>

            <Container header={<Header variant="h2">RBL defense</Header>}>
              <SpaceBetween size="m">
                <ColumnLayout columns={2}>
                  <FormField label="RBL lookup" description="Check the remote SMTP IP against registered DNSBL/RBL zones.">
                    <Toggle
                      checked={policy.rbl_check_enabled}
                      onChange={e => setPolicy(p => ({ ...p, rbl_check_enabled: e.detail.checked }))}
                    >
                      {policy.rbl_check_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
                    </Toggle>
                  </FormField>
                  <FormField label="Reject listed IPs" description="Reject RBL-listed senders during SMTP receive instead of only scoring.">
                    <Toggle
                      checked={policy.rbl_reject_enabled}
                      onChange={e => setPolicy(p => ({ ...p, rbl_reject_enabled: e.detail.checked }))}
                    >
                      {policy.rbl_reject_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
                    </Toggle>
                  </FormField>
                </ColumnLayout>
                <FormField label="RBL zones" description="Register DNSBL zones such as your licensed enterprise RBL provider.">
                  <SpaceBetween size="xs">
                    {policy.rbl_zones.length === 0 && (
                      <Box color="text-body-secondary" fontSize="body-s">No RBL zones registered.</Box>
                    )}
                    <SpaceBetween direction="horizontal" size="xs">
                      {policy.rbl_zones.map((zone, i) => (
                        <SpaceBetween key={zone} direction="horizontal" size="xs">
                          <Badge color="blue">{zone}</Badge>
                          <Button variant="inline-link" onClick={() => removeFromList('rbl_zones', i)}>
                            {t('common.delete')}
                          </Button>
                        </SpaceBetween>
                      ))}
                    </SpaceBetween>
                    <SpaceBetween direction="horizontal" size="xs">
                      <Input
                        value={newRBLZone}
                        onChange={e => setNewRBLZone(e.detail.value)}
                        placeholder="zen.example-rbl.test"
                      />
                      <Button onClick={() => addToList('rbl_zones', newRBLZone, setNewRBLZone)}>
                        {t('common.add')}
                      </Button>
                    </SpaceBetween>
                  </SpaceBetween>
                </FormField>
              </SpaceBetween>
            </Container>

            {/* Attachments */}
            <Container header={<Header variant="h2">{t('pages.spam_filter_page.attachments_section')}</Header>}>
              <SpaceBetween size="m">
                <FormField
                  label={t('pages.spam_filter_page.max_attachment_label')}
                  constraintText={t('pages.spam_filter_page.max_attachment_hint')}
                >
                  <Input
                    type="number"
                    value={String(policy.max_attachment_mb)}
                    onChange={e => setPolicy(p => ({ ...p, max_attachment_mb: parseInt(e.detail.value) || 0 }))}
                  />
                </FormField>

                <FormField label={t('pages.spam_filter_page.blocked_ext_label')} description={t('pages.spam_filter_page.blocked_ext_desc')}>
                  <SpaceBetween size="xs">
                    <SpaceBetween direction="horizontal" size="xs">
                      {policy.blocked_extensions.map((ext, i) => (
                        <SpaceBetween key={i} direction="horizontal" size="xs">
                          <Badge color="red">{ext}</Badge>
                          <Button variant="inline-link" onClick={() => removeFromList('blocked_extensions', i)}>
                            {t('common.delete')}
                          </Button>
                        </SpaceBetween>
                      ))}
                    </SpaceBetween>
                    <SpaceBetween direction="horizontal" size="xs">
                      <Input
                        value={newBlockedExt}
                        onChange={e => setNewBlockedExt(e.detail.value)}
                        placeholder=".exe"
                      />
                      <Button onClick={() => addToList('blocked_extensions', newBlockedExt, setNewBlockedExt)}>
                        {t('common.add')}
                      </Button>
                    </SpaceBetween>
                  </SpaceBetween>
                </FormField>
              </SpaceBetween>
            </Container>

            {/* Sender lists */}
            <Container header={<Header variant="h2">{t('pages.spam_filter_page.senders_section')}</Header>}>
              <SpaceBetween size="m">
                <FormField label={t('pages.spam_filter_page.blocked_senders_label')} description={t('pages.spam_filter_page.blocked_senders_desc')}>
                  <SpaceBetween size="xs">
                    {policy.blocked_senders.length === 0 && (
                      <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_blocked_senders')}</Box>
                    )}
                    {policy.blocked_senders.map((s, i) => (
                      <SpaceBetween key={i} direction="horizontal" size="xs">
                        <Badge color="red">{s}</Badge>
                        <Button variant="inline-link" onClick={() => removeFromList('blocked_senders', i)}>
                          {t('common.delete')}
                        </Button>
                      </SpaceBetween>
                    ))}
                    <SpaceBetween direction="horizontal" size="xs">
                      <Input
                        value={newBlockedSender}
                        onChange={e => setNewBlockedSender(e.detail.value)}
                        placeholder="spam@example.com or @domain.com"
                      />
                      <Button onClick={() => addToList('blocked_senders', newBlockedSender, setNewBlockedSender)}>
                        {t('common.add')}
                      </Button>
                    </SpaceBetween>
                  </SpaceBetween>
                </FormField>

                <FormField label={t('pages.spam_filter_page.allowed_senders_label')} description={t('pages.spam_filter_page.allowed_senders_desc')}>
                  <SpaceBetween size="xs">
                    {policy.allowed_senders.length > 0 && (
                      <Alert type="info">{t('pages.spam_filter_page.allowed_senders_warning')}</Alert>
                    )}
                    {policy.allowed_senders.length === 0 && (
                      <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_allowed_senders')}</Box>
                    )}
                    {policy.allowed_senders.map((s, i) => (
                      <SpaceBetween key={i} direction="horizontal" size="xs">
                        <Badge color="green">{s}</Badge>
                        <Button variant="inline-link" onClick={() => removeFromList('allowed_senders', i)}>
                          {t('common.delete')}
                        </Button>
                      </SpaceBetween>
                    ))}
                    <SpaceBetween direction="horizontal" size="xs">
                      <Input
                        value={newAllowedSender}
                        onChange={e => setNewAllowedSender(e.detail.value)}
                        placeholder="trusted@partner.com or @trusted.com"
                      />
                      <Button onClick={() => addToList('allowed_senders', newAllowedSender, setNewAllowedSender)}>
                        {t('common.add')}
                      </Button>
                    </SpaceBetween>
                  </SpaceBetween>
                </FormField>
              </SpaceBetween>
            </Container>

            <DataTable
              columnDefinitions={[
                {
                  header: 'Time',
                  cell: (item: SpamFilterEvent) => item.created_at ? new Date(item.created_at).toLocaleString() : '—',
                  width: '16%',
                },
                {
                  header: 'From',
                  cell: (item: SpamFilterEvent) => item.from_addr || item.mail_from || '—',
                  width: '18%',
                },
                {
                  header: 'Subject',
                  cell: (item: SpamFilterEvent) => item.subject || '—',
                  width: '24%',
                },
                {
                  header: 'Action',
                  cell: (item: SpamFilterEvent) => item.enhanced_status || item.flow_status,
                  width: '10%',
                },
                {
                  header: 'Score',
                  cell: (item: SpamFilterEvent) => item.spam_score?.toFixed(1) ?? '—',
                  width: '8%',
                },
                {
                  header: 'Reason',
                  cell: (item: SpamFilterEvent) => item.error_message || '—',
                  width: '24%',
                },
              ]}
              items={events}
              header={<Header variant="h2" counter={`(${events.length})`}>Recent spam filter events</Header>}
            />
          </>
        )}

        <Box float="right">
          <Button variant="primary" onClick={handleSave} loading={saving}>
            {t('pages.spam_filter_page.save')}
          </Button>
        </Box>
      </SpaceBetween>
    </ContentLayout>
  );
}
