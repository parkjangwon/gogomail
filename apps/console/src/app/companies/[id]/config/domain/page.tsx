'use client';

import {
  ContentLayout,
  Header,
  Table,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface DomainConfig {
  id: string;
  domain: string;
  config_key: string;
  config_value: string;
  last_updated: string;
}

export default function DomainConfigPage() {
  const { t } = useI18n();
  const [configs, setConfigs] = useState<DomainConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchDomainConfig();
  }, []);

  const fetchDomainConfig = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/config/domain?limit=100', {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setConfigs(data.config || []);
      }
    } catch (error) {
      console.error('Failed to fetch domain config:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredConfigs = configs.filter(
    (c) =>
      c.domain.toLowerCase().includes(filter.toLowerCase()) ||
      c.config_key.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.config_domain_page.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header variant="h1" description={t('pages.config_domain_page.description')}>
          {t('pages.config_domain_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Alert type="info">{t('pages.config_domain_page.info_message')}</Alert>

        <Table
          columnDefinitions={[
            {
              header: t('pages.config_domain_page.domain'),
              cell: (item: DomainConfig) => item.domain,
              width: '20%',
            },
            {
              header: t('pages.config_domain_page.key'),
              cell: (item: DomainConfig) => item.config_key,
              width: '25%',
            },
            {
              header: t('pages.config_domain_page.value'),
              cell: (item: DomainConfig) => item.config_value,
              width: '35%',
            },
            {
              header: t('pages.config_domain_page.last_updated'),
              cell: (item: DomainConfig) => new Date(item.last_updated).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredConfigs}
          header={
            <Header variant="h2" counter={`(${filteredConfigs.length})`}>
              {t('pages.config_domain_page.title')}
            </Header>
          }
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
