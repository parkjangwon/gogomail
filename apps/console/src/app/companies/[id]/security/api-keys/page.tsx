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
  Select,
  Alert,
  Flashbar,
  CopyToClipboard,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

interface Domain {
  id: string;
  name: string;
  ID?: string;
  Name?: string;
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

type FlashItem = {
  type: 'success' | 'error' | 'info' | 'warning';
  content: string;
  id: string;
  dismissible: boolean;
  onDismiss: () => void;
};

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
  const [createdSecret, setCreatedSecret] = useState<{ id: string; secret: string } | null>(null);

  const [rotatingId, setRotatingId] = useState<string | null>(null);
  const [rotatedSecret, setRotatedSecret] = useState<{ keyId: string; secret: string } | null>(null);
  const [showRotateModal, setShowRotateModal] = useState(false);

  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [flashItems, setFlashItems] = useState<FlashItem[]>([]);

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

  const addFlash = (type: FlashItem['type'], content: string) => {
    const id = Date.now().toString();
    setFlashItems(prev => [...prev, {
      type, content, id, dismissible: true,
      onDismiss: () => setFlashItems(f => f.filter(i => i.id !== id)),
    }]);
  };

  const fetchDomains = async () => {
    setDomainsLoading(true);
    try {
      const res = await fetch(`/api/admin/domains?company_id=${companyId}&limit=100`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        // Normalize domain shape (backend may return ID/Name or id/name)
        const raw: Array<{ id?: string; name?: string; ID?: string; Name?: string }> = data.domains || [];
        setDomains(raw.map(d => ({
          id: d.id ?? d.ID ?? '',
          name: d.name ?? d.Name ?? '',
        })));
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
        setCreatedSecret({ id: data.id, secret: data.secret });
        fetchAPIKeys(selectedDomainId);
        setNewKeyName('');
        addFlash('success', t('pages.api_keys_page.key_created_success'));
      } else {
        const err = await res.json().catch(() => ({}));
        addFlash('error', err.error || t('pages.api_keys_page.create_failed'));
      }
    } catch (error) {
      console.error('Failed to create API key:', error);
      addFlash('error', t('pages.api_keys_page.create_failed'));
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteKey = async (keyId: string) => {
    if (!selectedDomainId) return;
    setDeletingId(keyId);
    try {
      const res = await fetch(`/api/admin/domains/${selectedDomainId}/api-keys/${keyId}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (res.ok) {
        fetchAPIKeys(selectedDomainId);
        addFlash('success', t('pages.api_keys_page.key_deleted'));
      } else {
        addFlash('error', t('pages.api_keys_page.delete_failed'));
      }
    } catch (error) {
      console.error('Failed to delete API key:', error);
      addFlash('error', t('pages.api_keys_page.delete_failed'));
    } finally {
      setDeletingId(null);
    }
  };

  const handleRotateKey = async (keyId: string) => {
    if (!selectedDomainId) return;
    setRotatingId(keyId);
    try {
      const res = await fetch(`/api/admin/domains/${selectedDomainId}/api-keys/${keyId}/rotate`, {
        method: 'POST',
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setRotatedSecret({ keyId, secret: data.secret });
        setShowRotateModal(true);
        fetchAPIKeys(selectedDomainId);
        addFlash('success', t('pages.api_keys_page.key_rotated'));
      } else {
        addFlash('error', t('pages.api_keys_page.rotate_failed'));
      }
    } catch (error) {
      console.error('Failed to rotate API key:', error);
      addFlash('error', t('pages.api_keys_page.rotate_failed'));
    } finally {
      setRotatingId(null);
    }
  };

  const domainOptions = domains.map((d) => ({ label: d.name || d.id, value: d.id }));
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
        {flashItems.length > 0 && <Flashbar items={flashItems} />}

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
                cell: (item: APIKey) => (
                  <SpaceBetween size="xxxs">
                    <Box fontWeight="bold">{item.name}</Box>
                    <Box color="text-body-secondary" fontSize="body-s">{item.id}</Box>
                  </SpaceBetween>
                ),
                width: '30%',
              },
              {
                header: t('pages.api_keys_page.status'),
                cell: (item: APIKey) => (
                  <StatusIndicator type={item.is_active ? 'success' : 'stopped'}>
                    {item.is_active ? 'Active' : 'Inactive'}
                  </StatusIndicator>
                ),
                width: '15%',
              },
              {
                header: t('pages.api_keys.last_used'),
                cell: (item: APIKey) =>
                  item.last_used_at
                    ? new Date(item.last_used_at).toLocaleString()
                    : <Box color="text-body-secondary">—</Box>,
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
                  <SpaceBetween direction="horizontal" size="xs">
                    <Button
                      variant="inline-link"
                      onClick={() => handleRotateKey(item.id)}
                      loading={rotatingId === item.id}
                    >
                      {t('buttons.rotate')}
                    </Button>
                    <Button
                      variant="inline-link"
                      onClick={() => handleDeleteKey(item.id)}
                      loading={deletingId === item.id}
                    >
                      {t('common.delete')}
                    </Button>
                  </SpaceBetween>
                ),
                width: '20%',
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
                <SpaceBetween size="m" alignItems="center">
                  <StatusIndicator type="info">{t('pages.api_keys_page.no_keys')}</StatusIndicator>
                  <Button variant="primary" onClick={() => { setCreatedSecret(null); setShowModal(true); }}>
                    {t('pages.api_keys.create_key')}
                  </Button>
                </SpaceBetween>
              </Box>
            }
          />
        )}

        {!selectedDomainId && domains.length > 0 && (
          <Alert type="info">{t('pages.api_keys_page.info_message')}</Alert>
        )}
      </SpaceBetween>

      {/* Create Key Modal */}
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
            <FormField
              label={t('pages.api_keys_page.secret_label')}
              description={t('pages.api_keys_page.secret_desc')}
            >
              <CopyToClipboard
                copyButtonText={t('buttons.copy')}
                copySuccessText={t('common.success')}
                copyErrorText={t('common.error')}
                textToCopy={createdSecret.secret}
              />
              <Box color="text-body-secondary" fontSize="body-s" padding={{ top: 'xs' }}>
                {createdSecret.secret}
              </Box>
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

      {/* Rotate Secret Modal */}
      <Modal
        onDismiss={() => { setShowRotateModal(false); setRotatedSecret(null); }}
        visible={showRotateModal}
        footer={
          <Box float="right">
            <Button onClick={() => { setShowRotateModal(false); setRotatedSecret(null); }}>
              {t('common.close')}
            </Button>
          </Box>
        }
        header={t('pages.api_keys_page.rotate_modal_header')}
      >
        <SpaceBetween size="m">
          <Alert type="warning">{t('pages.api_keys_page.rotate_warning')}</Alert>
          {rotatedSecret && (
            <FormField
              label={t('pages.api_keys_page.new_secret_label')}
              description={t('pages.api_keys_page.secret_desc')}
            >
              <CopyToClipboard
                copyButtonText={t('buttons.copy')}
                copySuccessText={t('common.success')}
                copyErrorText={t('common.error')}
                textToCopy={rotatedSecret.secret}
              />
              <Box color="text-body-secondary" fontSize="body-s" padding={{ top: 'xs' }}>
                {rotatedSecret.secret}
              </Box>
            </FormField>
          )}
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
