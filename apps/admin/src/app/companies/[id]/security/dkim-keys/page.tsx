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
  Textarea,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface DKIMKey {
  id: string;
  domain_id: string;
  domain: string;
  selector: string;
  status: string;
  dns_verified: boolean;
  created_at: string;
}

export default function DKIMKeysPage() {
  const { t } = useI18n();
  const [keys, setKeys] = useState<DKIMKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newKey, setNewKey] = useState({
    domain_id: '',
    selector: '',
    private_key_pem: '',
    public_key_dns: '',
  });
  const [creating, setCreating] = useState(false);

  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<DKIMKey | null>(null);

  useEffect(() => {
    fetchDKIMKeys();
  }, []);

  const fetchDKIMKeys = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/dkim-keys?limit=100', {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setKeys(data.keys || []);
      }
    } catch (error) {
      console.error('Failed to fetch DKIM keys:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (
      !newKey.domain_id.trim() ||
      !newKey.selector.trim() ||
      !newKey.private_key_pem.trim() ||
      !newKey.public_key_dns.trim()
    )
      return;
    setCreating(true);
    try {
      const res = await fetch('/api/admin/dkim-keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain_id: newKey.domain_id.trim(),
          selector: newKey.selector.trim(),
          private_key_pem: newKey.private_key_pem.trim(),
          public_key_dns: newKey.public_key_dns.trim(),
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowCreateModal(false);
        setNewKey({ domain_id: '', selector: '', private_key_pem: '', public_key_dns: '' });
        fetchDKIMKeys();
      }
    } catch (error) {
      console.error('Failed to create DKIM key:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleDeactivate = async (key: DKIMKey) => {
    setDeletingId(key.id);
    try {
      await fetch(`/api/admin/dkim-keys/${key.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      fetchDKIMKeys();
    } catch (error) {
      console.error('Failed to deactivate DKIM key:', error);
    } finally {
      setDeletingId(null);
      setConfirmDelete(null);
    }
  };

  const filteredKeys = keys.filter(
    (k) =>
      (k.domain || '').toLowerCase().includes(filter.toLowerCase()) ||
      k.selector.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.dkim_page.title')}</Header>}>
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
          description={t('pages.dkim_page.description')}
          actions={
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              {t('pages.dkim_page.generate_key')}
            </Button>
          }
        >
          {t('pages.dkim_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.dkim_keys.domain'),
              cell: (item: DKIMKey) => item.domain || item.domain_id,
              width: '22%',
            },
            {
              header: t('pages.dkim_page.selector'),
              cell: (item: DKIMKey) => item.selector,
              width: '18%',
            },
            {
              header: t('pages.dkim_keys.status'),
              cell: (item: DKIMKey) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: t('pages.dkim_page.dns_verified'),
              cell: (item: DKIMKey) => (
                <Badge color={item.dns_verified ? 'green' : 'red'}>
                  {item.dns_verified ? t('pages.dkim_page.verified') : t('pages.dkim_page.not_verified')}
                </Badge>
              ),
              width: '18%',
            },
            {
              header: t('pages.dkim_page.created'),
              cell: (item: DKIMKey) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
            {
              header: t('pages.dkim_page.actions'),
              cell: (item: DKIMKey) => (
                <Button
                  variant="inline-link"
                  onClick={() => setConfirmDelete(item)}
                  loading={deletingId === item.id}
                >
                  {t('pages.dkim_page.deactivate')}
                </Button>
              ),
              width: '15%',
            },
          ]}
          items={filteredKeys}
          header={
            <Header variant="h2" counter={`(${filteredKeys.length})`}>
              {t('pages.dkim_page.keys')}
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
              <StatusIndicator type="info">{t('pages.dkim_page.no_keys')}</StatusIndicator>
            </Box>
          }
        />
      </SpaceBetween>

      {/* Create Modal */}
      <Modal
        onDismiss={() => setShowCreateModal(false)}
        visible={showCreateModal}
        size="large"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowCreateModal(false)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={handleCreate}
                loading={creating}
                disabled={
                  !newKey.domain_id.trim() ||
                  !newKey.selector.trim() ||
                  !newKey.private_key_pem.trim() ||
                  !newKey.public_key_dns.trim()
                }
              >
                {t('pages.dkim_page.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.dkim_page.create_modal_title')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.dkim_page.domain_id_label')}>
            <Input
              value={newKey.domain_id}
              onChange={(e) => setNewKey({ ...newKey, domain_id: e.detail.value })}
              placeholder="domain-id"
            />
          </FormField>
          <FormField label={t('pages.dkim_page.selector_label')}>
            <Input
              value={newKey.selector}
              onChange={(e) => setNewKey({ ...newKey, selector: e.detail.value })}
              placeholder="default"
            />
          </FormField>
          <FormField label={t('pages.dkim_page.private_key_label')}>
            <Textarea
              value={newKey.private_key_pem}
              onChange={(e) => setNewKey({ ...newKey, private_key_pem: e.detail.value })}
              placeholder="-----BEGIN RSA PRIVATE KEY-----"
              rows={6}
            />
          </FormField>
          <FormField label={t('pages.dkim_page.public_key_label')}>
            <Textarea
              value={newKey.public_key_dns}
              onChange={(e) => setNewKey({ ...newKey, public_key_dns: e.detail.value })}
              placeholder="v=DKIM1; k=rsa; p=..."
              rows={4}
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      {/* Deactivate Confirmation Modal */}
      <Modal
        onDismiss={() => setConfirmDelete(null)}
        visible={!!confirmDelete}
        size="small"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setConfirmDelete(null)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={() => confirmDelete && handleDeactivate(confirmDelete)}
                loading={deletingId === confirmDelete?.id}
              >
                {t('pages.dkim_page.deactivate')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.dkim_page.deactivate_modal_title')}
      >
        <Box>
          {t('pages.dkim_page.deactivate_confirm')}{' '}
          <strong>{confirmDelete?.selector}</strong>?
        </Box>
      </Modal>
    </ContentLayout>
  );
}
