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
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface TrustedRelay {
  id: string;
  cidr: string;
  description: string;
  created_at: string;
}

export default function TrustedRelaysPage() {
  const { t } = useI18n();
  const [relays, setRelays] = useState<TrustedRelay[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newRelay, setNewRelay] = useState({ cidr: '', description: '' });
  const [creating, setCreating] = useState(false);

  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<TrustedRelay | null>(null);

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
    } catch (error) {
      console.error('Failed to fetch trusted relays:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newRelay.cidr.trim()) return;
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
        fetchRelays();
      }
    } catch (error) {
      console.error('Failed to create trusted relay:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (relay: TrustedRelay) => {
    setDeletingId(relay.id);
    try {
      await fetch(`/api/admin/trusted-relays/${relay.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      fetchRelays();
    } catch (error) {
      console.error('Failed to delete trusted relay:', error);
    } finally {
      setDeletingId(null);
      setConfirmDelete(null);
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
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              {t('pages.relays.create_relay')}
            </Button>
          }
        >
          {t('pages.relays_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
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
              cell: (item: TrustedRelay) => (
                <Button
                  variant="inline-link"
                  onClick={() => setConfirmDelete(item)}
                  loading={deletingId === item.id}
                >
                  {t('common.delete')}
                </Button>
              ),
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
                onClick={() => confirmDelete && handleDelete(confirmDelete)}
                loading={deletingId === confirmDelete?.id}
              >
                {t('common.delete')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.relays_page.delete_modal_title')}
      >
        <Box>{t('pages.relays_page.delete_confirm')} <strong>{confirmDelete?.cidr}</strong>?</Box>
      </Modal>
    </ContentLayout>
  );
}
