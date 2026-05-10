'use client';

import {
  ContentLayout,
  Header,
  Table,
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

interface Domain {
  ID: string;
  Name: string;
}

interface ConfigEntry {
  ID: string;
  ScopeType: string;
  ScopeID: string;
  Key: string;
  Value: unknown;
  Locked: boolean;
  Version: number;
  UpdatedAt: string;
}

export default function DomainConfigPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [domains, setDomains] = useState<Domain[]>([]);
  const [domainsLoading, setDomainsLoading] = useState(true);
  const [selectedDomainId, setSelectedDomainId] = useState<string>('');

  const [configs, setConfigs] = useState<ConfigEntry[]>([]);
  const [configLoading, setConfigLoading] = useState(false);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchDomains();
  }, [companyId]);

  useEffect(() => {
    if (selectedDomainId) {
      fetchDomainConfig(selectedDomainId);
    } else {
      setConfigs([]);
    }
  }, [selectedDomainId]);

  const fetchDomains = async () => {
    setDomainsLoading(true);
    try {
      const res = await fetch(`/api/admin/domains?company_id=${companyId}&limit=100`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setDomains(data.domains || []);
      }
    } catch (error) {
      console.error('Failed to fetch domains:', error);
    } finally {
      setDomainsLoading(false);
    }
  };

  const fetchDomainConfig = async (domainId: string) => {
    setConfigLoading(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/config`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setConfigs(data.config || []);
      }
    } catch (error) {
      console.error('Failed to fetch domain config:', error);
    } finally {
      setConfigLoading(false);
    }
  };

  const domainOptions = domains.map((d) => ({ label: d.Name || d.ID, value: d.ID }));
  const selectedOption = domainOptions.find((o) => o.value === selectedDomainId) ?? null;

  const filteredConfigs = configs.filter(
    (c) =>
      c.Key.toLowerCase().includes(filter.toLowerCase())
  );

  if (domainsLoading) {
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
        {domains.length === 0 && (
          <Alert type="info">{t('pages.config_domain_page.no_domains')}</Alert>
        )}

        {domains.length > 0 && (
          <FormField label={t('pages.config_domain_page.select_domain')}>
            <Select
              selectedOption={selectedOption}
              options={domainOptions}
              onChange={(e) => setSelectedDomainId(e.detail.selectedOption.value ?? '')}
              placeholder={t('pages.config_domain_page.select_domain_placeholder')}
              expandToViewport
            />
          </FormField>
        )}

        {selectedDomainId && configLoading && (
          <Box textAlign="center" padding="l">
            <Spinner />
          </Box>
        )}

        {selectedDomainId && !configLoading && (
          <Table
            columnDefinitions={[
              {
                header: t('pages.config_domain_page.key'),
                cell: (item: ConfigEntry) => item.Key,
                width: '30%',
              },
              {
                header: t('pages.config_domain_page.value'),
                cell: (item: ConfigEntry) =>
                  typeof item.Value === 'object' ? JSON.stringify(item.Value) : String(item.Value ?? ''),
                width: '50%',
              },
              {
                header: t('pages.config_domain_page.last_updated'),
                cell: (item: ConfigEntry) =>
                  item.UpdatedAt ? new Date(item.UpdatedAt).toLocaleString() : '—',
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
            empty={
              <Box textAlign="center" padding="l">
                {t('pages.config_domain_page.no_config')}
              </Box>
            }
          />
        )}

        {!selectedDomainId && domains.length > 0 && (
          <Alert type="info">{t('pages.config_domain_page.info_message')}</Alert>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
