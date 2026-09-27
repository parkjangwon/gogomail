'use client';
import { DataTable } from '@/components/DataTable';
import { ConfirmModal } from '@/components/ConfirmModal';


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
  StatusIndicator,
  Alert,
  Flashbar,
  FlashbarProps,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface TrustedRelay {
  id: string;
  cidr: string;
  description: string;
  created_at: string;
}

export default function TrustedRelaysPage() {
  const { t } = useI18n();
  const { canMutate } = useCompany();
  const [relays, setRelays] = useState<TrustedRelay[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newRelay, setNewRelay] = useState({ cidr: '', description: '' });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<TrustedRelay | null>(null);
  const [deleteError, setDeleteError] = useState('');

  const extractError = async (res: Response, fallback: string): Promise<string> => {
    const data = (await res.json().catch(() => ({}))) as { error?: { message?: string } | string };
    const msg = typeof data.error === 'string' ? data.error : data.error?.message;
    return msg || fallback;
  };

  useEffect(() => {
    fetchRelays();
  }, []);

  const fetchRelays = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/trusted-relays?limit=100', {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setRelays(data.relays || []);
      }
    } catch {
      // mutation error handled by caller
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newRelay.cidr.trim()) return;
    setCreateError('');
    setCreating(true);
    try {
      const res = await fetch('/api/admin/trusted-relays', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          cidr: newRelay.cidr.trim(),
          description: newRelay.description.trim() || undefined,
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowCreateModal(false);
        setNewRelay({ cidr: '', description: '' });
        setFlash([{ type: 'success', content: t('pages.relays_page.created', 'Relay created.'), dismissible: true, onDismiss: () => setFlash([]) }]);
        fetchRelays();
      } else {
        setCreateError(await extractError(res, t('pages.relays_page.create_failed', 'Failed to create relay.')));
      }
    } catch {
      setCreateError(t('pages.relays_page.create_failed', 'Failed to create relay.'));
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (relay: TrustedRelay) => {
    setDeletingId(relay.id);
    setDeleteError('');
    try {
      const res = await fetch(`/api/admin/trusted-relays/${relay.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (res.ok) {
        setConfirmDelete(null);
        fetchRelays();
      } else {
        setDeleteError(await extractError(res, t('pages.relays_page.delete_failed', 'Failed to delete relay.')));
      }
    } catch {
      setDeleteError(t('pages.relays_page.delete_failed', 'Failed to delete relay.'));
    } finally {
      setDeletingId(null);
    }
  };

  const filteredRelays = relays.filter((r) =>
    r.cidr.toLowerCase().includes(filter.toLowerCase()) ||
    (r.description || '').toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.relays_page.title')}</Header>}>
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
          description={t('pages.relays_page.description')}
          actions={
            canMutate ? (
              <Button variant="primary" onClick={() => { setCreateError(''); setShowCreateModal(true); }}>
                {t('pages.relays.create_relay')}
              </Button>
            ) : undefined
          }
        >
          {t('pages.relays_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {flash.length > 0 && <Flashbar items={flash} />}
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.relays_page.cidr'),
              cell: (item: TrustedRelay) => (
                <Box fontWeight="bold">{item.cidr}</Box>
              ),
              width: '30%',
            },
            {
              header: t('pages.relays_page.description'),
              cell: (item: TrustedRelay) => (
                <Box color="text-body-secondary">{item.description || '—'}</Box>
              ),
              width: '35%',
            },
            {
              header: t('pages.relays_page.created'),
              cell: (item: TrustedRelay) =>
                new Date(item.created_at).toLocaleDateString(),
              width: '20%',
            },
            {
              header: t('pages.relays_page.actions'),
              cell: (item: TrustedRelay) =>
                canMutate ? (
                  <Button
                    variant="inline-link"
                    onClick={() => { setDeleteError(''); setConfirmDelete(item); }}
                    loading={deletingId === item.id}
                  >
                    {t('common.delete')}
                  </Button>
                ) : null,
              width: '15%',
            },
          ]}
          items={filteredRelays}
          header={
            <Header variant="h2" counter={`(${filteredRelays.length})`}>
              {t('pages.relays_page.relays')}
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
              <StatusIndicator type="info">{t('pages.relays_page.no_relays')}</StatusIndicator>
            </Box>
          }
        />
      </SpaceBetween>

      {/* Create Modal */}
      <Modal
        onDismiss={() => setShowCreateModal(false)}
        visible={showCreateModal}
        size="medium"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowCreateModal(false)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={handleCreate}
                loading={creating}
                disabled={!newRelay.cidr.trim()}
              >
                {t('pages.relays_page.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.relays_page.create_modal_title')}
      >
        <SpaceBetween size="m">
          {createError && <Alert type="error">{createError}</Alert>}
          <FormField
            label={t('pages.relays_page.cidr_label')}
            constraintText={t('pages.relays_page.cidr_constraint')}
          >
            <Input
              value={newRelay.cidr}
              onChange={(e) => setNewRelay({ ...newRelay, cidr: e.detail.value })}
              placeholder="192.168.1.0/24"
            />
          </FormField>
          <FormField label={t('pages.relays_page.description_label')}>
            <Input
              value={newRelay.description}
              onChange={(e) => setNewRelay({ ...newRelay, description: e.detail.value })}
              placeholder={t('pages.relays_page.description_placeholder')}
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        visible={!!confirmDelete}
        header={t('pages.relays_page.delete_modal_title')}
        onConfirm={() => confirmDelete && handleDelete(confirmDelete)}
        onDismiss={() => { setConfirmDelete(null); setDeleteError(''); }}
        loading={deletingId === confirmDelete?.id}
        error={deleteError || undefined}
      >
        {t('pages.relays_page.delete_confirm')} <strong>{confirmDelete?.cidr}</strong>?
      </ConfirmModal>
    </ContentLayout>
  );
}
