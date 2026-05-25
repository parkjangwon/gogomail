'use client';
import { DataTable } from '@/components/DataTable';

import {
  ContentLayout,
  Header,
  Button,
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
import { useDomains } from '@/hooks/useDomains';
import {
  useApiKeys,
  useCreateApiKey,
  useDeleteApiKey,
  useRotateApiKey,
  type ApiKey,
} from '@/hooks/useApiKeys';

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

  const { data: domains = [], isLoading: domainsLoading } = useDomains(companyId);
  const [selectedDomainId, setSelectedDomainId] = useState<string>('');
  const { data: keys = [], isLoading: keysLoading } = useApiKeys(selectedDomainId);
  const createKey = useCreateApiKey();
  const deleteKey = useDeleteApiKey();
  const rotateKey = useRotateApiKey();
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [createdSecret, setCreatedSecret] = useState<{ id: string; secret: string } | null>(null);

  const [rotatingId, setRotatingId] = useState<string | null>(null);
  const [rotatedSecret, setRotatedSecret] = useState<{ keyId: string; secret: string } | null>(null);
  const [showRotateModal, setShowRotateModal] = useState(false);

  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [pendingDeleteId, setPendingDeleteId] = useState<string | null>(null);
  const [showDeleteModal, setShowDeleteModal] = useState(false);

  const [flashItems, setFlashItems] = useState<FlashItem[]>([]);

  useEffect(() => {
    if (!selectedDomainId && domains.length > 0) {
      setSelectedDomainId(domains[0].id);
    }
  }, [domains, selectedDomainId]);

  const addFlash = (type: FlashItem['type'], content: string) => {
    const id = Date.now().toString();
    setFlashItems(prev => [...prev, {
      type, content, id, dismissible: true,
      onDismiss: () => setFlashItems(f => f.filter(i => i.id !== id)),
    }]);
  };

  const handleCreateKey = async () => {
    if (!selectedDomainId || !newKeyName.trim()) return;
    try {
      const data = await createKey.mutateAsync({
        domainId: selectedDomainId,
        data: {
          name: newKeyName.trim(),
          created_by: 'admin-console',
        },
      });
      if (!data.secret) {
        throw new Error(t('pages.api_keys_page.create_failed'));
      }
      setCreatedSecret({ id: data.id ?? '', secret: data.secret });
      setNewKeyName('');
      addFlash('success', t('pages.api_keys_page.key_created_success'));
    } catch (error) {
      addFlash('error', error instanceof Error ? error.message : t('pages.api_keys_page.create_failed'));
    }
  };

  const handleDeleteKey = async (keyId: string) => {
    if (!selectedDomainId) return;
    setDeletingId(keyId);
    setShowDeleteModal(false);
    setPendingDeleteId(null);
    try {
      await deleteKey.mutateAsync({ domainId: selectedDomainId, keyId });
      addFlash('success', t('pages.api_keys_page.key_deleted'));
    } catch (error) {
      addFlash('error', error instanceof Error ? error.message : t('pages.api_keys_page.delete_failed'));
    } finally {
      setDeletingId(null);
    }
  };

  const requestDeleteKey = (keyId: string) => {
    setPendingDeleteId(keyId);
    setShowDeleteModal(true);
  };

  const handleRotateKey = async (keyId: string) => {
    if (!selectedDomainId) return;
    setRotatingId(keyId);
    try {
      const data = await rotateKey.mutateAsync({ domainId: selectedDomainId, keyId });
      if (!data.secret) {
        throw new Error(t('pages.api_keys_page.rotate_failed'));
      }
      setRotatedSecret({ keyId, secret: data.secret });
      setShowRotateModal(true);
      addFlash('success', t('pages.api_keys_page.key_rotated'));
    } catch (error) {
      addFlash('error', error instanceof Error ? error.message : t('pages.api_keys_page.rotate_failed'));
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
      <div style={{ display: 'grid', gap: 20 }}>
        {flashItems.length > 0 && <Flashbar key="flashbar" items={flashItems} />}

        {domains.length === 0 && (
          <Alert key="no-domains" type="info">{t('pages.api_keys_page.no_domains')}</Alert>
        )}

        {domains.length > 0 && (
          <FormField key="domain-select" label={t('pages.api_keys_page.select_domain')}>
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
          <Box key="loading-keys" textAlign="center" padding="l">
            <Spinner />
          </Box>
        )}

        {selectedDomainId && !keysLoading && (
          <DataTable
            key="api-keys-table"
            columnDefinitions={[
              {
                id: 'name',
                header: t('pages.api_keys_page.name'),
                cell: (item: ApiKey) => (
                  <div style={{ display: 'grid', gap: 2 }}>
                    <Box key="api-key-name" fontWeight="bold">{item.name}</Box>
                    <Box key="api-key-id" color="text-body-secondary" fontSize="body-s">{item.id}</Box>
                  </div>
                ),
                width: '30%',
              },
              {
                id: 'status',
                header: t('pages.api_keys_page.status'),
                cell: (item: ApiKey) => (
                  <StatusIndicator type={item.is_active ? 'success' : 'stopped'}>
                    {item.is_active ? 'Active' : 'Inactive'}
                  </StatusIndicator>
                ),
                width: '15%',
              },
              {
                id: 'last-used',
                header: t('pages.api_keys.last_used'),
                cell: (item: ApiKey) =>
                  item.last_used_at
                    ? new Date(item.last_used_at).toLocaleString()
                    : <Box color="text-body-secondary">—</Box>,
                width: '20%',
              },
              {
                id: 'created',
                header: t('pages.api_keys.created'),
                cell: (item: ApiKey) => new Date(item.created_at).toLocaleDateString(),
                width: '15%',
              },
              {
                id: 'actions',
                header: t('common.actions'),
                cell: (item: ApiKey) => (
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Button
                      key="rotate"
                      variant="inline-link"
                      onClick={() => handleRotateKey(item.id)}
                      loading={rotatingId === item.id}
                      disabled={(rotatingId !== null && rotatingId !== item.id) || deletingId !== null}
                    >
                      {t('buttons.rotate')}
                    </Button>
                    <Button
                      key="delete"
                      variant="inline-link"
                      onClick={() => requestDeleteKey(item.id)}
                      loading={deletingId === item.id}
                      disabled={(deletingId !== null && deletingId !== item.id) || rotatingId !== null}
                    >
                      {t('common.delete')}
                    </Button>
                  </div>
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
                <div style={{ display: 'grid', gap: 16, justifyItems: 'center' }}>
                  <StatusIndicator key="no-keys" type="info">{t('pages.api_keys_page.no_keys')}</StatusIndicator>
                  <Button key="create-empty" variant="primary" onClick={() => { setCreatedSecret(null); setShowModal(true); }}>
                    {t('pages.api_keys.create_key')}
                  </Button>
                </div>
              </Box>
            }
          />
        )}

        {!selectedDomainId && domains.length > 0 && (
          <Alert key="select-domain-info" type="info">{t('pages.api_keys_page.info_message')}</Alert>
        )}
      </div>

      {/* Create Key Modal */}
      <Modal
        key="create-key-modal"
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
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <Button key="cancel-create" onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
                <Button
                  key="confirm-create"
                  variant="primary"
                  onClick={handleCreateKey}
                  loading={createKey.isPending}
                  disabled={!newKeyName.trim()}
                >
                  {t('pages.api_keys_page.create_btn')}
                </Button>
              </div>
            </Box>
          )
        }
        header={t('pages.api_keys_page.modal_header')}
      >
        {createdSecret ? (
          <div style={{ display: 'grid', gap: 16 }}>
            <Alert key="created-success" type="success">{t('pages.api_keys_page.key_created_success')}</Alert>
            <FormField
              key="created-secret"
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
          </div>
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
        key="rotate-key-modal"
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
        <div style={{ display: 'grid', gap: 16 }}>
          <Alert key="rotate-warning" type="warning">{t('pages.api_keys_page.rotate_warning')}</Alert>
          {rotatedSecret && (
            <FormField
              key="rotated-secret"
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
        </div>
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        key="delete-confirm-modal"
        onDismiss={() => { setShowDeleteModal(false); setPendingDeleteId(null); }}
        visible={showDeleteModal}
        footer={
          <Box float="right">
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <Button key="cancel-delete" onClick={() => { setShowDeleteModal(false); setPendingDeleteId(null); }}>
                {t('common.cancel')}
              </Button>
              <Button
                key="confirm-delete"
                variant="primary"
                onClick={() => { if (pendingDeleteId) handleDeleteKey(pendingDeleteId); }}
              >
                {t('common.delete')}
              </Button>
            </div>
          </Box>
        }
        header={t('common.delete')}
      >
        <Alert type="warning">
          Are you sure you want to delete this API key? This action cannot be undone.
        </Alert>
      </Modal>
    </ContentLayout>
  );
}
