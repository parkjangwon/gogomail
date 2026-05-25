'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Badge,
  Modal,
  TextFilter,
  ColumnLayout,
  Container,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect, useMemo } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface UserRow {
  id: string;
  username: string;
  display_name: string;
  domain_id: string;
  status: string;
}

interface DomainRow {
  id: string;
}

interface MFAStats {
  total: number;
  enabled: number;
  enrolled: number;
  not_enrolled: number;
}

export default function MFAManagementPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [users, setUsers] = useState<UserRow[]>([]);
  const [stats, setStats] = useState<MFAStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  const [resetTarget, setResetTarget] = useState<UserRow | null>(null);
  const [resetting, setResetting] = useState(false);

  useEffect(() => {
    if (companyId) {
      fetchAll();
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [companyId]);

  const fetchAll = async () => {
    setLoading(true);
    try {
      const [domainsRes, statsRes] = await Promise.all([
        fetch(`/api/admin/domains?company_id=${encodeURIComponent(companyId)}&limit=200`, { credentials: 'include' }),
        fetch(`/api/admin/companies/${companyId}/mfa/stats`, { credentials: 'include' }),
      ]);
      if (domainsRes.ok) {
        const domainsData = await domainsRes.json();
        const domains: DomainRow[] = domainsData.domains || [];
        const userLists = await Promise.all(
          domains.map((domain) =>
            fetch(`/api/admin/users?domain_id=${encodeURIComponent(domain.id)}&limit=200`, { credentials: 'include' })
              .then((res) => res.ok ? res.json() : { users: [] })
          )
        );
        setUsers(userLists.flatMap((data: { users?: UserRow[] }) => data.users || []));
      }
      if (statsRes.ok) {
        const data = await statsRes.json();
        setStats(data.mfa_stats ?? null);
      }
    } catch {
      // mutation error handled by caller
    } finally {
      setLoading(false);
    }
  };

  const handleResetConfirm = async () => {
    if (!resetTarget) return;
    setResetting(true);
    try {
      await fetch(`/api/admin/users/${resetTarget.id}/mfa`, {
        method: 'DELETE',
        credentials: 'include',
      });
      setResetTarget(null);
      fetchAll();
    } catch {
      // mutation error handled by caller
    } finally {
      setResetting(false);
    }
  };

  const filteredUsers = useMemo(() => {
    if (!filter) return users;
    const q = filter.toLowerCase();
    return users.filter(
      u =>
        u.username.toLowerCase().includes(q) ||
        (u.display_name || '').toLowerCase().includes(q) ||
        (u.domain_id || '').toLowerCase().includes(q),
    );
  }, [users, filter]);

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.mfa.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.mfa.description')}>
          {t('pages.mfa.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* KPI Cards */}
        <ColumnLayout columns={4} variant="text-grid" minColumnWidth={140}>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{stats?.total ?? '—'}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.mfa.total_users')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold" color="text-status-success">{stats?.enabled ?? '—'}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.mfa.enabled')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold" color="text-status-info">{stats?.enrolled ?? '—'}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.mfa.enrolled')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold" color="text-body-secondary">{stats?.not_enrolled ?? '—'}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.mfa.not_enrolled')}</Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        {/* User Table */}
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.mfa.username'),
              cell: (u: UserRow) => (
                <SpaceBetween size="xxxs">
                  <Box fontWeight="bold">{u.username}</Box>
                  {u.display_name && (
                    <Box color="text-body-secondary" fontSize="body-s">{u.display_name}</Box>
                  )}
                </SpaceBetween>
              ),
              width: '30%',
            },
            {
              header: t('pages.mfa.domain'),
              cell: (u: UserRow) => (
                <Box color="text-body-secondary" fontSize="body-s">{u.domain_id || '—'}</Box>
              ),
              width: '25%',
            },
            {
              header: t('users.status'),
              cell: (u: UserRow) => (
                <Badge color={u.status === 'active' ? 'green' : 'grey'}>{u.status}</Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.mfa.reset_2fa'),
              cell: (u: UserRow) => (
                <Button
                  variant="inline-link"
                  onClick={() => setResetTarget(u)}
                >
                  {t('pages.mfa.reset_2fa')}
                </Button>
              ),
              width: '30%',
            },
          ]}
          items={filteredUsers}
          header={
            <Header variant="h2" counter={`(${filteredUsers.length})`}>
              {t('pages.mfa.title')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={e => setFilter(e.detail.filteringText)}
            />
          }
          empty={
            <Box textAlign="center" padding="l">
              <StatusIndicator type="info">{t('common.loading')}</StatusIndicator>
            </Box>
          }
        />
      </SpaceBetween>

      {/* Reset Confirmation Modal */}
      <Modal
        onDismiss={() => setResetTarget(null)}
        visible={!!resetTarget}
        size="medium"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setResetTarget(null)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={handleResetConfirm}
                loading={resetting}
              >
                {t('pages.mfa.reset_2fa')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.mfa.reset_confirm_title')}
      >
        <Box>{t('pages.mfa.reset_confirm_msg')} {resetTarget?.username}</Box>
      </Modal>
    </ContentLayout>
  );
}
