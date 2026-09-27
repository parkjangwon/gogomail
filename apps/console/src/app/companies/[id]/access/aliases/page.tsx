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
  Select,
  Alert,
  Flashbar,
  FlashbarProps,
} from '@cloudscape-design/components';
import { useState, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';
import { useCompany } from '@/contexts/CompanyContext';
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
  const { canMutate } = useCompany();

  const { data: aliases = [], isLoading: loading } = useDirectoryAliases(companyId);
  const [filter, setFilter] = useState('');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createError, setCreateError] = useState('');
  const [newAlias, setNewAlias] = useState<NewAlias>({
    domain_id: '',
    address: '',
    target_kind: DirectoryAliasCreateRequestTarget_kind.user,
    target_id: '',
  });
  const [creating, setCreating] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DirectoryAlias | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const createAlias = useCreateDirectoryAlias();
  const deleteAlias = useDeleteDirectoryAlias();

  const errMessage = (e: unknown, fallback: string) =>
    e instanceof Error && e.message ? e.message : fallback;

  const handleCreate = async () => {
    if (!newAlias.address.trim() || !newAlias.target_id.trim()) return;
    setCreateError('');
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
      setFlash([{ type: 'success', content: t('pages.aliases.created', 'Alias created.'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } catch (e) {
      // Keep the modal open so the admin can fix the input and retry.
      setCreateError(errMessage(e, t('pages.aliases.create_failed', 'Failed to create alias.')));
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (alias: DirectoryAlias) => {
    setDeleteError('');
    setDeleting(true);
    try {
      if (!companyId) return;
      await deleteAlias.mutateAsync({ id: alias.id, companyId });
      setDeleteTarget(null);
      setFlash([{ type: 'success', content: t('pages.aliases.deleted', 'Alias deleted.'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } catch (e) {
      setDeleteError(errMessage(e, t('pages.aliases.delete_failed', 'Failed to delete alias.')));
    } finally {
      setDeleting(false);
    }
  };

  // Values must match the backend PrincipalKind (user|organization|group|resource).
  const targetKindOptions = [
    { label: t('pages.aliases.target_kind_user'), value: 'user' },
    { label: t('pages.aliases.target_kind_group'), value: 'group' },
    { label: t('pages.aliases.target_kind_organization', 'Organization'), value: 'organization' },
    { label: t('pages.aliases.target_kind_resource', 'Resource'), value: 'resource' },
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
            canMutate ? (
              <Button variant="primary" onClick={() => { setCreateError(''); setShowCreateModal(true); }}>
                {t('pages.aliases.add_alias')}
              </Button>
            ) : undefined
          }
        >
          {t('pages.aliases_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {flash.length > 0 && <Flashbar items={flash} />}
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
              cell: (item: DirectoryAlias) =>
                canMutate ? (
                  <Button
                    variant="inline-link"
                    onClick={() => { setDeleteError(''); setDeleteTarget(item); }}
                    loading={deleting && deleteTarget?.id === item.id}
                  >
                    {t('common.delete')}
                  </Button>
                ) : null,
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
          {createError && <Alert type="error">{createError}</Alert>}
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

      <ConfirmModal
        visible={!!deleteTarget}
        header={t('pages.aliases.delete_modal_title', 'Delete alias')}
        onConfirm={() => deleteTarget && handleDelete(deleteTarget)}
        onDismiss={() => { setDeleteTarget(null); setDeleteError(''); }}
        loading={deleting}
        error={deleteError || undefined}
      >
        {t('pages.aliases.delete_confirm', 'Delete the alias')}{' '}
        <strong>{deleteTarget?.address}</strong>?
      </ConfirmModal>
    </ContentLayout>
  );
}
