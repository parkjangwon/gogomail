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
  Select,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

interface Domain {
  ID: string;
  Name: string;
}

interface APIKey {
  id: string;
  name: string;
  created_by: string;
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
  is_active: boolean;
}

export default function APIKeysPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [domains, setDomains] = useState<Domain[]>([]);
  const [domainsLoading, setDomainsLoading] = useState(true);
  const [selectedDomainId, setSelectedDomainId] = useState<string>('');

  const [keys, setKeys] = useState<APIKey[]>([]);
  const [keysLoading, setKeysLoading] = useState(false);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [creating, setCreating] = useState(false);
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);

  useEffect(() => {
    fetchDomains();
  }, [companyId]);

  useEffect(() => {
    if (selectedDomainId) {
      fetchAPIKeys(selectedDomainId);
    } else {
      setKeys([]);
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

  const fetchAPIKeys = async (domainId: string) => {
    setKeysLoading(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/api-keys`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setKeys(data.keys || []);
      }
    } catch (error) {
      console.error('Failed to fetch API keys:', error);
    } finally {
      setKeysLoading(false);
    }
  };

  const handleCreateKey = async () => {
    if (!selectedDomainId || !newKeyName.trim()) return;
    setCreating(true);
    try {
      const res = await fetch(`/api/admin/domains/${selectedDomainId}/api-keys`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newKeyName.trim() }),
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setCreatedSecret(data.secret || null);
        fetchAPIKeys(selectedDomainId);
        setNewKeyName('');
      }
    } catch (error) {
      console.error('Failed to create API key:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteKey = async (keyId: string) => {
    if (!selectedDomainId) return;
    try {
      await fetch(`/api/admin/domains/${selectedDomainId}/api-keys/${keyId}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      fetchAPIKeys(selectedDomainId);
    } catch (error) {
      console.error('Failed to delete API key:', error);
    }
  };

  const domainOptions = domains.map((d) => ({ label: d.Name || d.ID, value: d.ID }));
  const selectedOption = domainOptions.find((o) => o.value === selectedDomainId) ?? null;

  const filteredKeys = keys.filter((k) =>
    k.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (domainsLoading) {
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
        <Header variant="h1" description={t('pages.api_keys_page.description')}>
          {t('pages.api_keys.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {domains.length === 0 && (
          <Alert type="info">{t('pages.api_keys_page.no_domains')}</Alert>
        )}

        {domains.length > 0 && (
          <FormField label={t('pages.api_keys_page.select_domain')}>
            <Select
              selectedOption={selectedOption}
              options={domainOptions}
              onChange={(e) => setSelectedDomainId(e.detail.selectedOption.value ?? '')}
              placeholder={t('pages.api_keys_page.select_domain_placeholder')}
              expandToViewport
            />
          </FormField>
        )}

        {selectedDomainId && keysLoading && (
          <Box textAlign="center" padding="l">
            <Spinner />
          </Box>
        )}

        {selectedDomainId && !keysLoading && (
          <Table
            columnDefinitions={[
              {
                header: t('pages.api_keys_page.name'),
                cell: (item: APIKey) => item.name,
                width: '25%',
              },
              {
                header: t('pages.api_keys_page.status'),
                cell: (item: APIKey) => (
                  <Badge color={item.is_active ? 'green' : 'grey'}>
                    {item.is_active ? 'active' : 'inactive'}
                  </Badge>
                ),
                width: '15%',
              },
              {
                header: t('pages.api_keys.last_used'),
                cell: (item: APIKey) =>
                  item.last_used_at ? new Date(item.last_used_at).toLocaleString() : '—',
                width: '20%',
              },
              {
                header: t('pages.api_keys.created'),
                cell: (item: APIKey) => new Date(item.created_at).toLocaleDateString(),
                width: '15%',
              },
              {
                header: t('common.actions'),
                cell: (item: APIKey) => (
                  <Button
                    variant="inline-link"
                    onClick={() => handleDeleteKey(item.id)}
                  >
                    {t('common.delete')}
                  </Button>
                ),
                width: '15%',
              },
            ]}
            items={filteredKeys}
            header={
              <Header
                variant="h2"
                counter={`(${filteredKeys.length})`}
                actions={
                  <Button variant="primary" onClick={() => { setCreatedSecret(null); setShowModal(true); }}>
                    {t('pages.api_keys.create_key')}
                  </Button>
                }
              >
                {t('pages.api_keys_page.keys')}
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
                {t('pages.api_keys_page.no_keys')}
              </Box>
            }
          />
        )}

        {!selectedDomainId && domains.length > 0 && (
          <Alert type="info">{t('pages.api_keys_page.info_message')}</Alert>
        )}
      </SpaceBetween>

      <Modal
        onDismiss={() => { setShowModal(false); setCreatedSecret(null); }}
        visible={showModal}
        footer={
          createdSecret ? (
            <Box float="right">
              <Button onClick={() => { setShowModal(false); setCreatedSecret(null); }}>
                {t('common.close')}
              </Button>
            </Box>
          ) : (
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
                <Button
                  variant="primary"
                  onClick={handleCreateKey}
                  loading={creating}
                  disabled={!newKeyName.trim()}
                >
                  {t('pages.api_keys_page.create_btn')}
                </Button>
              </SpaceBetween>
            </Box>
          )
        }
        header={t('pages.api_keys_page.modal_header')}
      >
        {createdSecret ? (
          <SpaceBetween size="m">
            <Alert type="success">{t('pages.api_keys_page.key_created_success')}</Alert>
            <FormField label={t('pages.api_keys_page.secret_label')} description={t('pages.api_keys_page.secret_desc')}>
              <Input value={createdSecret} readOnly onChange={() => {}} />
            </FormField>
          </SpaceBetween>
        ) : (
          <FormField
            label={t('pages.api_keys_page.key_name_label')}
            description={t('pages.api_keys_page.key_name_desc')}
          >
            <Input
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.detail.value)}
              placeholder={t('pages.api_keys_page.key_placeholder')}
            />
          </FormField>
        )}
      </Modal>
    </ContentLayout>
  );
}
