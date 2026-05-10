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
  const [saving, setSaving] = useState(false);

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

  const handleCreateSetting = async () => {
    if (!newSetting.domain.trim() || !newSetting.key.trim()) return;
    setSaving(true);
    try {
      const res = await fetch('/api/admin/domain-settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain_name: newSetting.domain,
          setting_key: newSetting.key,
          setting_value: newSetting.value,
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowModal(false);
        setNewSetting({ domain: '', key: '', value: '' });
        fetchDomainSettings();
      }
    } catch (error) {
      console.error('Failed to create domain setting:', error);
    } finally {
      setSaving(false);
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
          description={t('pages.domain_settings_page.description')}
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              {t('pages.domain_settings_page.add_setting_btn')}
            </Button>
          }
        >
          {t('pages.domain_settings.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.domain_settings_page.domain'),
              cell: (item: DomainSettings) => item.domain_name,
              width: '25%',
            },
            {
              header: t('pages.domain_settings_page.setting_key'),
              cell: (item: DomainSettings) => item.setting_key,
              width: '25%',
            },
            {
              header: t('pages.domain_settings_page.value'),
              cell: (item: DomainSettings) => item.setting_value,
              width: '35%',
            },
            {
              header: t('pages.domain_settings_page.last_updated'),
              cell: (item: DomainSettings) => new Date(item.last_updated).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredSettings}
          header={<Header variant="h2" counter={`(${filteredSettings.length})`}>{t('pages.domain_settings_page.settings_list')}</Header>}
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
              <Button variant="primary" onClick={handleCreateSetting} loading={saving}>
                {t('pages.domain_settings_page.add_setting')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.domain_settings_page.add_setting_modal')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.domain_settings_page.domain_label')}>
            <Input
              value={newSetting.domain}
              onChange={(e) => setNewSetting({ ...newSetting, domain: e.detail.value })}
              placeholder="domain.com"
            />
          </FormField>
          <FormField label={t('pages.domain_settings_page.key_label')}>
            <Input
              value={newSetting.key}
              onChange={(e) => setNewSetting({ ...newSetting, key: e.detail.value })}
              placeholder="e.g., max_users"
            />
          </FormField>
          <FormField label={t('pages.domain_settings_page.value_label')}>
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
