'use client';
import { DataTable } from '@/components/DataTable';


import {
  Badge,
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Toggle,
  FormField,
  Input,
  Select,
  type SelectProps,
  Container,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { buildLoginAuditsQuery, exportLoginAuditsCsv, type LoginAuditRow } from '@/lib/loginAudits';

interface SessionPolicy {
  timeout_minutes: number;
  max_concurrent_sessions: number;
  require_reauth_for_sensitive_ops: boolean;
  idle_timeout_minutes: number;
}

interface ActiveSession {
  user_id: string;
  email: string;
  ip: string;
  started_at: string;
  last_active: string;
  user_agent: string;
}

interface LoginAudit extends LoginAuditRow {}

const DEFAULT_POLICY: SessionPolicy = {
  timeout_minutes: 480,
  max_concurrent_sessions: 0,
  require_reauth_for_sensitive_ops: false,
  idle_timeout_minutes: 0,
};

const loginAuditBadgeColor = (success: boolean): 'green' | 'red' => (success ? 'green' : 'red');

export default function SessionManagementPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [policy, setPolicy] = useState<SessionPolicy>(DEFAULT_POLICY);
  const [sessions, setSessions] = useState<ActiveSession[]>([]);
  const [loadingPolicy, setLoadingPolicy] = useState(true);
  const [loadingSessions, setLoadingSessions] = useState(true);
  const [loginAudits, setLoginAudits] = useState<LoginAudit[]>([]);
  const [loadingLoginAudits, setLoadingLoginAudits] = useState(true);
  const [saving, setSaving] = useState(false);
  const [terminatingId, setTerminatingId] = useState<string | null>(null);
  const [loginUserId, setLoginUserId] = useState('');
  const [loginSuccess, setLoginSuccess] = useState<SelectProps.Option>({ label: t('pages.session_page.login_all'), value: '' });
  const [loginFromDate, setLoginFromDate] = useState('');
  const [loginToDate, setLoginToDate] = useState('');

  const loginSuccessOptions: SelectProps.Option[] = [
    { label: t('pages.session_page.login_all'), value: '' },
    { label: t('pages.session_page.login_successful'), value: 'true' },
    { label: t('pages.session_page.login_failed'), value: 'false' },
  ];

  const fetchPolicy = useCallback(async () => {
    if (!companyId) return;
    setLoadingPolicy(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/security/session-policy`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setPolicy(data.policy);
      }
    } catch (error) {
      console.error('Failed to fetch session policy:', error);
    } finally {
      setLoadingPolicy(false);
    }
  }, [companyId]);

  const fetchSessions = useCallback(async () => {
    if (!companyId) return;
    setLoadingSessions(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/sessions`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setSessions(data.sessions ?? []);
      }
    } catch (error) {
      console.error('Failed to fetch sessions:', error);
    } finally {
      setLoadingSessions(false);
    }
  }, [companyId]);

  const fetchLoginAudits = useCallback(async () => {
    if (!companyId) return;
    setLoadingLoginAudits(true);
    try {
      const successValue = loginSuccess.value as string;
      const query = buildLoginAuditsQuery({
        companyId,
        userId: loginUserId,
        success: successValue === '' ? undefined : successValue === 'true',
        fromDate: loginFromDate,
        toDate: loginToDate,
        limit: 50,
        offset: 0,
      });
      const res = await fetch(`/api/admin/companies/${companyId}/security/login-audits${query ? `?${query}` : ''}`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setLoginAudits(data.login_audits ?? []);
      }
    } catch (error) {
      console.error('Failed to fetch login audits:', error);
    } finally {
      setLoadingLoginAudits(false);
    }
  }, [companyId, loginFromDate, loginSuccess.value, loginToDate, loginUserId]);

  useEffect(() => {
    fetchPolicy();
    fetchSessions();
    fetchLoginAudits();
  }, [fetchLoginAudits, fetchPolicy, fetchSessions]);

  const handleSavePolicy = async () => {
    setSaving(true);
    try {
      await fetch(`/api/admin/companies/${companyId}/security/session-policy`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
    } catch (error) {
      console.error('Failed to save session policy:', error);
    } finally {
      setSaving(false);
    }
  };

  const handleTerminate = async (userId: string) => {
    setTerminatingId(userId);
    try {
      await fetch(`/api/admin/companies/${companyId}/sessions/${userId}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      await fetchSessions();
    } catch (error) {
      console.error('Failed to terminate session:', error);
    } finally {
      setTerminatingId(null);
    }
  };

  const handleExportLoginAudits = () => {
    if (loginAudits.length === 0) return;
    const csv = exportLoginAuditsCsv(loginAudits.map((audit) => ({
      id: audit.id,
      user_id: audit.user_id,
      company_id: audit.company_id,
      ip_address: audit.ip_address,
      user_agent: audit.user_agent,
      success: audit.success,
      failure_reason: audit.failure_reason,
      timestamp: audit.timestamp,
    })));
    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `login-audits-${companyId}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  if (loadingPolicy && loadingSessions) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.session_page.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.session_page.description')}>
          {t('pages.session_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h2">{t('pages.session_page.policy_header')}</Header>}>
          <SpaceBetween size="m">
            <FormField label={t('pages.session_page.timeout')}>
              <Input
                type="number"
                value={String(policy.timeout_minutes)}
                onChange={(e) => setPolicy({ ...policy, timeout_minutes: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.session_page.idle_timeout')}>
              <Input
                type="number"
                value={String(policy.idle_timeout_minutes)}
                onChange={(e) => setPolicy({ ...policy, idle_timeout_minutes: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.session_page.max_concurrent')}>
              <Input
                type="number"
                value={String(policy.max_concurrent_sessions)}
                onChange={(e) => setPolicy({ ...policy, max_concurrent_sessions: parseInt(e.detail.value) || 0 })}
              />
            </FormField>

            <FormField label={t('pages.session_page.require_reauth')}>
              <Toggle
                checked={policy.require_reauth_for_sensitive_ops}
                onChange={(e) => setPolicy({ ...policy, require_reauth_for_sensitive_ops: e.detail.checked })}
              >
                {policy.require_reauth_for_sensitive_ops ? 'Enabled' : 'Disabled'}
              </Toggle>
            </FormField>

            <Button variant="primary" onClick={handleSavePolicy} loading={saving}>
              {t('pages.session_page.save')}
            </Button>
          </SpaceBetween>
        </Container>

        <DataTable
          header={<Header variant="h2">{t('pages.session_page.sessions_header')}</Header>}
          loading={loadingSessions}
          items={sessions}
          empty={
            <Box textAlign="center" color="inherit">
              {t('pages.session_page.no_sessions')}
            </Box>
          }
          columnDefinitions={[
            {
              id: 'email',
              header: t('pages.session_page.email_col'),
              cell: (item) => item.email,
            },
            {
              id: 'ip',
              header: t('pages.session_page.ip_col'),
              cell: (item) => item.ip,
            },
            {
              id: 'started_at',
              header: t('pages.session_page.started_col'),
              cell: (item) => new Date(item.started_at).toLocaleString(),
            },
            {
              id: 'last_active',
              header: t('pages.session_page.last_active_col'),
              cell: (item) => new Date(item.last_active).toLocaleString(),
            },
            {
              id: 'user_agent',
              header: t('pages.session_page.agent_col'),
              cell: (item) => item.user_agent,
            },
            {
              id: 'actions',
              header: '',
              cell: (item) => (
                <Button
                  variant="link"
                  loading={terminatingId === item.user_id}
                  onClick={() => handleTerminate(item.user_id)}
                >
                  {t('pages.session_page.terminate')}
                </Button>
              ),
            },
          ]}
        />

        <Container header={<Header variant="h2">{t('pages.session_page.login_audits_header')}</Header>}>
          <SpaceBetween size="m">
            <Box color="text-body-secondary">
              {t('pages.session_page.login_audits_description')}
            </Box>
            <SpaceBetween direction="horizontal" size="xs">
              <FormField label={t('pages.session_page.login_user_id')}>
                <Input
                  value={loginUserId}
                  onChange={(e) => setLoginUserId(e.detail.value)}
                  placeholder={t('pages.session_page.login_user_id_placeholder')}
                />
              </FormField>
              <FormField label={t('pages.session_page.login_status')}>
                <Select
                  selectedOption={loginSuccess}
                  options={loginSuccessOptions}
                  onChange={(e) => setLoginSuccess(e.detail.selectedOption)}
                />
              </FormField>
              <FormField label={t('pages.session_page.from_date')}>
                <Input
                  value={loginFromDate}
                  onChange={(e) => setLoginFromDate(e.detail.value)}
                  placeholder="2026-05-01T00:00:00Z"
                />
              </FormField>
              <FormField label={t('pages.session_page.to_date')}>
                <Input
                  value={loginToDate}
                  onChange={(e) => setLoginToDate(e.detail.value)}
                  placeholder="2026-05-31T23:59:59Z"
                />
              </FormField>
              <Button onClick={() => void fetchLoginAudits()} loading={loadingLoginAudits}>
                {t('pages.session_page.refresh_login_audits')}
              </Button>
              <Button onClick={handleExportLoginAudits} disabled={loginAudits.length === 0}>
                {t('pages.session_page.export_login_audits')}
              </Button>
            </SpaceBetween>

            <DataTable
              loading={loadingLoginAudits}
              items={loginAudits}
              empty={<Box textAlign="center" color="inherit">{t('pages.session_page.no_login_audits')}</Box>}
              columnDefinitions={[
                {
                  id: 'timestamp',
                  header: t('pages.session_page.login_time_col'),
                  cell: (item) => new Date(item.timestamp).toLocaleString(),
                },
                {
                  id: 'user_id',
                  header: t('pages.session_page.login_user_col'),
                  cell: (item) => item.user_id,
                },
                {
                  id: 'status',
                  header: t('pages.session_page.login_status_col'),
                  cell: (item) => (
                    <Badge color={loginAuditBadgeColor(item.success)}>
                      {item.success ? t('pages.session_page.login_successful') : t('pages.session_page.login_failed')}
                    </Badge>
                  ),
                },
                {
                  id: 'ip_address',
                  header: t('pages.session_page.login_ip_col'),
                  cell: (item) => item.ip_address || '—',
                },
                {
                  id: 'failure_reason',
                  header: t('pages.session_page.login_reason_col'),
                  cell: (item) => item.failure_reason || '—',
                },
              ]}
            />
          </SpaceBetween>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
