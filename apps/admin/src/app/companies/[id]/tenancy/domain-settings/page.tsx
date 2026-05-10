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
  Modal,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface DomainSettings {
  id: string;
  domain_name: string;
  setting_key: string;
  setting_value: string;
  last_updated: string;
}

export default function DomainSettingsPage() {
  const { t } = useI18n();
  const [settings, setSettings] = useState<DomainSettings[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newSetting, setNewSetting] = useState({ domain: '', key: '', value: '' });

  useEffect(() => {
    fetchDomainSettings();
  }, []);

  const fetchDomainSettings = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/domain-settings?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setSettings(data.settings || []);
      }
    } catch (error) {
      console.error('Failed to fetch domain settings:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredSettings = settings.filter(s =>
    s.domain_name.toLowerCase().includes(filter.toLowerCase()) ||
    s.setting_key.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.domain_settings.title')}</Header>}>
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
          description="Configure domain-level settings"
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              + Add Setting
            </Button>
          }
        >
          Domain Settings
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Domain',
              cell: (item: DomainSettings) => item.domain_name,
              width: '25%',
            },
            {
              header: 'Setting Key',
              cell: (item: DomainSettings) => item.setting_key,
              width: '25%',
            },
            {
              header: 'Value',
              cell: (item: DomainSettings) => item.setting_value,
              width: '35%',
            },
            {
              header: 'Last Updated',
              cell: (item: DomainSettings) => new Date(item.last_updated).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredSettings}
          header={<Header variant="h2" counter={`(${filteredSettings.length})`}>Settings List</Header>}
          filter={
            <TextFilter
              filteringText={filter}
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
              <Button onClick={() => setShowModal(false)}>Cancel</Button>
              <Button variant="primary">Add Setting</Button>
            </SpaceBetween>
          </Box>
        }
        header="Add Domain Setting"
      >
        <SpaceBetween size="m">
          <FormField label="Domain">
            <Input
              value={newSetting.domain}
              onChange={(e) => setNewSetting({ ...newSetting, domain: e.detail.value })}
              placeholder="domain.com"
            />
          </FormField>
          <FormField label="Setting Key">
            <Input
              value={newSetting.key}
              onChange={(e) => setNewSetting({ ...newSetting, key: e.detail.value })}
              placeholder="e.g., max_users"
            />
          </FormField>
          <FormField label="Setting Value">
            <Input
              value={newSetting.value}
              onChange={(e) => setNewSetting({ ...newSetting, value: e.detail.value })}
              placeholder="Value"
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
