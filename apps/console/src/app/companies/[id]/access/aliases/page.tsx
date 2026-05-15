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
  Modal,
  FormField,
  Input,
  Select,
} from '@cloudscape-design/components';
import { useState, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';
import {
  type DirectoryAlias,
  useCreateDirectoryAlias,
  useDeleteDirectoryAlias,
  useDirectoryAliases,
} from '@/hooks/useDirectory';
import { DirectoryAliasCreateRequestTarget_kind } from '@gogomail/api-types';

type NewAlias = {
  domain_id: string;
  address: string;
  target_kind: DirectoryAliasCreateRequestTarget_kind;
  target_id: string;
};

export default function AliasesPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const { data: aliases = [], isLoading: loading } = useDirectoryAliases(companyId);
  const [filter, setFilter] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newAlias, setNewAlias] = useState<NewAlias>({
    domain_id: '',
    address: '',
    target_kind: DirectoryAliasCreateRequestTarget_kind.user,
    target_id: '',
  });
  const [creating, setCreating] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const createAlias = useCreateDirectoryAlias();
  const deleteAlias = useDeleteDirectoryAlias();

  const handleCreate = async () => {
    if (!newAlias.address.trim() || !newAlias.target_id.trim()) return;
    setCreating(true);
    try {
      if (!companyId) return;
      await createAlias.mutateAsync({
        companyId,
        data: {
          company_id: companyId,
          domain_id: newAlias.domain_id,
          address: newAlias.address,
          target_kind: newAlias.target_kind,
          target_id: newAlias.target_id,
        },
      });
      setShowCreateModal(false);
      setNewAlias({ domain_id: '', address: '', target_kind: DirectoryAliasCreateRequestTarget_kind.user, target_id: '' });
    } catch (error) {
      console.error('Failed to create alias:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      if (!companyId) return;
      await deleteAlias.mutateAsync({
        id,
        companyId,
      });
    } catch (error) {
      console.error('Failed to delete alias:', error);
    } finally {
      setDeletingId(null);
    }
  };

  const targetKindOptions = [
    { label: t('pages.aliases.target_kind_user'), value: 'user' },
    { label: t('pages.aliases.target_kind_group'), value: 'group' },
    { label: t('pages.aliases.target_kind_external'), value: 'organization' },
    { label: 'Resource', value: 'resource' },
  ];

  const filteredAliases = useMemo(() => aliases.filter(
    (a) =>
      a.address.toLowerCase().includes(filter.toLowerCase()) ||
      a.target_id.toLowerCase().includes(filter.toLowerCase())
  ), [aliases, filter]);

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.aliases_page.title')}</Header>}>
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
          description={t('pages.aliases_page.description')}
          actions={
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              {t('pages.aliases.add_alias')}
            </Button>
          }
        >
          {t('pages.aliases_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.aliases.address'),
              cell: (item: DirectoryAlias) => item.address,
              width: '30%',
            },
            {
              header: t('pages.aliases.domain'),
              cell: (item: DirectoryAlias) => item.domain_id || '—',
              width: '20%',
            },
            {
              header: t('pages.aliases.target_kind'),
              cell: (item: DirectoryAlias) => item.target_kind,
              width: '15%',
            },
            {
              header: t('pages.aliases.target_id'),
              cell: (item: DirectoryAlias) => item.target_id,
              width: '20%',
            },
            {
              header: t('pages.aliases_page.status'),
              cell: (item: DirectoryAlias) => item.status || '—',
              width: '10%',
            },
            {
              header: t('common.actions'),
              cell: (item: DirectoryAlias) => (
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
          items={filteredAliases}
          header={
            <Header variant="h2" counter={`(${filteredAliases.length})`}>
              {t('pages.aliases_page.aliases')}
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
              {t('pages.aliases.no_aliases')}
            </Box>
          }
        />
      </SpaceBetween>

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
                disabled={!newAlias.address.trim() || !newAlias.target_id.trim()}
              >
                {t('pages.aliases.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.aliases.create_modal_title')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.aliases.domain_label')}>
            <Input
              value={newAlias.domain_id}
              onChange={(e) => setNewAlias({ ...newAlias, domain_id: e.detail.value })}
              placeholder="domain-id"
            />
          </FormField>
          <FormField label={t('pages.aliases.address_label')}>
            <Input
              value={newAlias.address}
              onChange={(e) => setNewAlias({ ...newAlias, address: e.detail.value })}
              placeholder="alias@example.com"
            />
          </FormField>
          <FormField label={t('pages.aliases.target_kind_label')}>
            <Select
              selectedOption={
                targetKindOptions.find((o) => o.value === newAlias.target_kind) ??
                targetKindOptions[0]
              }
              options={targetKindOptions}
              onChange={(e) =>
                setNewAlias({
                  ...newAlias,
                  target_kind: e.detail.selectedOption.value as DirectoryAliasCreateRequestTarget_kind,
                })
              }
              expandToViewport
            />
          </FormField>
          <FormField label={t('pages.aliases.target_id_label')}>
            <Input
              value={newAlias.target_id}
              onChange={(e) => setNewAlias({ ...newAlias, target_id: e.detail.value })}
              placeholder="user-id or group-id"
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
