'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface UserConfig {
  id: string;
  user_email: string;
  config_key: string;
  config_value: string;
  last_updated: string;
}

export default function UserConfigPage() {
  const { t } = useI18n();
  const [configs, setConfigs] = useState<UserConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchUserConfig();
  }, []);

  const fetchUserConfig = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/config/user?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setConfigs(data.config || []);
      }
    } catch (error) {
      console.error('Failed to fetch user config:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredConfigs = configs.filter(c =>
    c.user_email.toLowerCase().includes(filter.toLowerCase()) ||
    c.config_key.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.config_user.title')}</Header>}>
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
          description={t('pages.config_user_page.description')}
          actions={
            <Button variant="primary" disabled>
              {t('pages.config_user_page.add_config_btn')}
            </Button>
          }
        >
          {t('pages.config_user_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.config_user_page.user_email'),
              cell: (item: UserConfig) => item.user_email,
              width: '30%',
            },
            {
              header: t('pages.config_user_page.key'),
              cell: (item: UserConfig) => item.config_key,
              width: '25%',
            },
            {
              header: t('pages.config_user_page.value'),
              cell: (item: UserConfig) => item.config_value,
              width: '25%',
            },
            {
              header: t('pages.config_user_page.last_updated'),
              cell: (item: UserConfig) => new Date(item.last_updated).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredConfigs}
          header={<Header variant="h2" counter={`(${filteredConfigs.length})`}>{t('pages.config_user.title')}</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
