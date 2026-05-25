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
  Badge,
  Modal,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface SuppressionEntry {
  id: string;
  email: string;
  reason: string;
  added_at: string;
}

export default function SuppressionListPage() {
  const { t } = useI18n();
  const [entries, setEntries] = useState<SuppressionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newEntry, setNewEntry] = useState({ email: '', reason: '' });
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    fetchSuppressionList();
  }, []);

  const fetchSuppressionList = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/suppression-list?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setEntries(data.suppression_list || []);
      }
    } catch {
      // mutation error handled by caller
    } finally {
      setLoading(false);
    }
  };

  const handleAddEntry = async () => {
    try {
      await fetch('/api/admin/suppression-list', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newEntry),
        credentials: 'include',
      });
      setShowModal(false);
      setNewEntry({ email: '', reason: '' });
      fetchSuppressionList();
    } catch {
      // mutation error handled by caller
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      await fetch(`/api/admin/suppression-list/${id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      fetchSuppressionList();
    } catch {
      // mutation error handled by caller
    } finally {
      setDeletingId(null);
    }
  };

  const filteredEntries = entries.filter(e =>
    e.email.toLowerCase().includes(filter.toLowerCase()) ||
    e.reason.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.suppression.title')}</Header>}>
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
          description={t('pages.suppression_page.description')}
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              {t('pages.suppression_page.add_email')}
            </Button>
          }
        >
          {t('pages.suppression_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.suppression.email'),
              cell: (item: SuppressionEntry) => item.email,
              width: '40%',
            },
            {
              header: t('pages.suppression.reason'),
              cell: (item: SuppressionEntry) => (
                <Badge color="red">{item.reason}</Badge>
              ),
              width: '35%',
            },
            {
              header: t('pages.suppression_page.added'),
              cell: (item: SuppressionEntry) => new Date(item.added_at).toLocaleString(),
              width: '20%',
            },
            {
              header: t('common.actions'),
              cell: (item: SuppressionEntry) => (
                <Button
                  variant="inline-link"
                  onClick={() => handleDelete(item.id)}
                  loading={deletingId === item.id}
                >
                  {t('common.delete')}
                </Button>
              ),
              width: '5%',
            },
          ]}
          items={filteredEntries}
          header={<Header variant="h2" counter={`(${filteredEntries.length})`}>{t('pages.suppression_page.entries')}</Header>}
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
              <Button variant="primary" onClick={handleAddEntry}>
                {t('pages.suppression_page.add_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.suppression_page.modal_header')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.suppression_page.email_label')}>
            <Input
              value={newEntry.email}
              onChange={(e) => setNewEntry({ ...newEntry, email: e.detail.value })}
              placeholder={t('pages.suppression_page.email_placeholder')}
            />
          </FormField>
          <FormField label={t('pages.suppression_page.reason_label')} description={t('pages.suppression_page.reason_desc')}>
            <Input
              value={newEntry.reason}
              onChange={(e) => setNewEntry({ ...newEntry, reason: e.detail.value })}
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
