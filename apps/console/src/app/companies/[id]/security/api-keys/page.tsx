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
  Badge,
  Modal,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface APIKey {
  id: string;
  key_prefix: string;
  name: string;
  status: string;
  last_used: string;
  created_at: string;
}

export default function APIKeysPage() {
  const { t } = useI18n();
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newKey, setNewKey] = useState({ name: '' });

  useEffect(() => {
    fetchAPIKeys();
  }, []);

  const fetchAPIKeys = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/api-keys?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setKeys(data.keys || []);
      }
    } catch (error) {
      console.error('Failed to fetch API keys:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateKey = async () => {
    try {
      await fetch('/api/admin/api-keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newKey),
        credentials: 'include',
      });
      setShowModal(false);
      setNewKey({ name: '' });
      fetchAPIKeys();
    } catch (error) {
      console.error('Failed to create API key:', error);
    }
  };

  const filteredKeys = keys.filter(k =>
    k.name.toLowerCase().includes(filter.toLowerCase()) ||
    k.key_prefix.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.api_keys.title')}</Header>}>
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
          description={t('pages.api_keys_page.description')}
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              {t('pages.api_keys.create_key')}
            </Button>
          }
        >
          {t('pages.api_keys.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.api_keys_page.name'),
              cell: (item: APIKey) => item.name,
              width: '25%',
            },
            {
              header: t('pages.api_keys_page.key_prefix'),
              cell: (item: APIKey) => item.key_prefix,
              width: '25%',
            },
            {
              header: t('pages.api_keys_page.status'),
              cell: (item: APIKey) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.api_keys.last_used'),
              cell: (item: APIKey) => item.last_used ? new Date(item.last_used).toLocaleString() : 'Never',
              width: '20%',
            },
            {
              header: t('pages.api_keys.created'),
              cell: (item: APIKey) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredKeys}
          header={<Header variant="h2" counter={`(${filteredKeys.length})`}>{t('pages.api_keys_page.keys')}</Header>}
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
              <Button variant="primary" onClick={handleCreateKey}>
                {t('pages.api_keys_page.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.api_keys_page.modal_header')}
      >
        <FormField label={t('pages.api_keys_page.key_name_label')} description={t('pages.api_keys_page.key_name_desc')}>
          <Input
            value={newKey.name}
            onChange={(e) => setNewKey({ name: e.detail.value })}
            placeholder={t('pages.api_keys_page.key_placeholder')}
          />
        </FormField>
      </Modal>
    </ContentLayout>
  );
}
