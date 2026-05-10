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
  Container,
  Table,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

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

const DEFAULT_POLICY: SessionPolicy = {
  timeout_minutes: 480,
  max_concurrent_sessions: 0,
  require_reauth_for_sensitive_ops: false,
  idle_timeout_minutes: 0,
};

export default function SessionManagementPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [policy, setPolicy] = useState<SessionPolicy>(DEFAULT_POLICY);
  const [sessions, setSessions] = useState<ActiveSession[]>([]);
  const [loadingPolicy, setLoadingPolicy] = useState(true);
  const [loadingSessions, setLoadingSessions] = useState(true);
  const [saving, setSaving] = useState(false);
  const [terminatingId, setTerminatingId] = useState<string | null>(null);

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

  useEffect(() => {
    fetchPolicy();
    fetchSessions();
  }, [fetchPolicy, fetchSessions]);

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

        <Table
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
      </SpaceBetween>
    </ContentLayout>
  );
}
