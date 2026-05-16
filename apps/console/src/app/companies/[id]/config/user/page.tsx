'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Select,
  FormField,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

interface User {
  id: string;
  username: string;
  display_name: string;
  domain_id: string;
}

interface Domain {
  id: string;
}

interface ConfigEntry {
  ID: string;
  Key: string;
  Value: unknown;
  Locked: boolean;
  UpdatedAt: string;
}

export default function UserConfigPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [users, setUsers] = useState<User[]>([]);
  const [usersLoading, setUsersLoading] = useState(true);
  const [selectedUserId, setSelectedUserId] = useState<string>('');

  const [configs, setConfigs] = useState<ConfigEntry[]>([]);
  const [configLoading, setConfigLoading] = useState(false);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchUsers();
  }, [companyId]);

  useEffect(() => {
    if (selectedUserId) {
      fetchUserConfig(selectedUserId);
    } else {
      setConfigs([]);
    }
  }, [selectedUserId]);

  const fetchUsers = async () => {
    setUsersLoading(true);
    try {
      const domainsRes = await fetch(`/api/admin/domains?company_id=${encodeURIComponent(companyId)}&limit=200`, {
        credentials: 'include',
      });
      if (!domainsRes.ok) return;
      const domainsData = await domainsRes.json();
      const domains: Domain[] = domainsData.domains || [];
      const userLists = await Promise.all(
        domains.map((domain) =>
          fetch(`/api/admin/users?domain_id=${encodeURIComponent(domain.id)}&limit=200`, {
            credentials: 'include',
          }).then((res) => res.ok ? res.json() : { users: [] })
        )
      );
      setUsers(userLists.flatMap((data: { users?: User[] }) => data.users || []));
    } catch (error) {
      console.error('Failed to fetch users:', error);
    } finally {
      setUsersLoading(false);
    }
  };

  const fetchUserConfig = async (userId: string) => {
    setConfigLoading(true);
    try {
      const res = await fetch(`/api/admin/users/${userId}/config`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setConfigs(data.config || []);
      }
    } catch (error) {
      console.error('Failed to fetch user config:', error);
    } finally {
      setConfigLoading(false);
    }
  };

  const userOptions = Array.from(
    new Map(users.map((u) => [u.id, u])).values()
  ).map((u) => ({
    label: u.display_name ? `${u.display_name} (${u.username})` : u.username,
    value: u.id,
  }));
  const selectedOption = userOptions.find((o) => o.value === selectedUserId) ?? null;

  const filteredConfigs = configs.filter((c) =>
    c.Key.toLowerCase().includes(filter.toLowerCase())
  );

  if (usersLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.config_user_page.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.config_user_page.description')}>
          {t('pages.config_user_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {users.length === 0 && (
          <Alert type="info">{t('pages.config_user_page.no_users')}</Alert>
        )}

        {users.length > 0 && (
          <FormField label={t('pages.config_user_page.select_user')}>
            <Select
              selectedOption={selectedOption}
              options={userOptions}
              onChange={(e) => setSelectedUserId(e.detail.selectedOption.value ?? '')}
              placeholder={t('pages.config_user_page.select_user_placeholder')}
              filteringType="auto"
              expandToViewport
            />
          </FormField>
        )}

        {selectedUserId && configLoading && (
          <Box textAlign="center" padding="l">
            <Spinner />
          </Box>
        )}

        {selectedUserId && !configLoading && (
          <DataTable
            columnDefinitions={[
              {
                header: t('pages.config_user_page.key'),
                cell: (item: ConfigEntry) => item.Key,
                width: '30%',
              },
              {
                header: t('pages.config_user_page.value'),
                cell: (item: ConfigEntry) =>
                  typeof item.Value === 'object'
                    ? JSON.stringify(item.Value)
                    : String(item.Value ?? ''),
                width: '50%',
              },
              {
                header: t('pages.config_user_page.last_updated'),
                cell: (item: ConfigEntry) =>
                  item.UpdatedAt ? new Date(item.UpdatedAt).toLocaleString() : '—',
                width: '20%',
              },
            ]}
            items={filteredConfigs}
            header={
              <Header variant="h2" counter={`(${filteredConfigs.length})`}>
                {t('pages.config_user_page.title')}
              </Header>
            }
            filter={
              <TextFilter
                filteringText={filter}
                filteringPlaceholder={t('common.search')}
                onChange={(e) => setFilter(e.detail.filteringText)}
              />
            }
            empty={
              <Box textAlign="center" padding="l">
                {t('pages.config_user_page.no_config')}
              </Box>
            }
          />
        )}

        {!selectedUserId && users.length > 0 && (
          <Alert type="info">{t('pages.config_user_page.info_message')}</Alert>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
