'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Modal,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

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

export default function CompanyConfigPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const [configs, setConfigs] = useState<ConfigEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newConfig, setNewConfig] = useState({ key: '', value: '' });

  useEffect(() => {
    fetchCompanyConfig();
  }, [companyId]);

  const fetchCompanyConfig = async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/config`, {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setConfigs(data.config || []);
      }
    } catch (error) {
      console.error('Failed to fetch company config:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateConfig = async () => {
    if (!newConfig.key.trim()) return;
    try {
      await fetch(`/api/admin/companies/${companyId}/config/${newConfig.key}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value: newConfig.value }),
        credentials: 'include',
      });
      setShowModal(false);
      setNewConfig({ key: '', value: '' });
      fetchCompanyConfig();
    } catch (error) {
      console.error('Failed to create config:', error);
    }
  };

  const filteredConfigs = configs.filter(c =>
    c.Key.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.config_company.title')}</Header>}>
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
          description={t('pages.config_company.description')}
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              {t('pages.config_company_page.add_config_btn')}
            </Button>
          }
        >
          {t('pages.config_company.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.config_company_page.key'),
              cell: (item: ConfigEntry) => item.Key,
              width: '30%',
            },
            {
              header: t('pages.config_company_page.value'),
              cell: (item: ConfigEntry) => typeof item.Value === 'object' ? JSON.stringify(item.Value) : String(item.Value ?? ''),
              width: '45%',
            },
            {
              header: t('pages.config_company_page.last_updated'),
              cell: (item: ConfigEntry) => item.UpdatedAt ? new Date(item.UpdatedAt).toLocaleString() : '—',
              width: '25%',
            },
          ]}
          items={filteredConfigs}
          header={<Header variant="h2" counter={`(${filteredConfigs.length})`}>{t('pages.config_company.title')}</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>

      <Modal
        onDismiss={() => setShowModal(false)}
        visible={showModal}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={handleCreateConfig} disabled={!newConfig.key.trim()}>
                {t('pages.config_company_page.add_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.config_company_page.modal_header')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.config_company_page.key_label')}>
            <Input
              value={newConfig.key}
              onChange={(e) => setNewConfig({ ...newConfig, key: e.detail.value })}
              placeholder={t('pages.config_company_page.key_placeholder')}
            />
          </FormField>
          <FormField label={t('pages.config_company_page.value_label')}>
            <Input
              value={newConfig.value}
              onChange={(e) => setNewConfig({ ...newConfig, value: e.detail.value })}
              placeholder={t('pages.config_company_page.value_placeholder')}
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
