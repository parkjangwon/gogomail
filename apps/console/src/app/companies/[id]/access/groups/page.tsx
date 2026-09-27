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
  Badge,
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
  DirectoryGroupMembershipCreateRequestMember_kind,
  DirectoryGroupMembershipCreateRequestRole,
} from '@gogomail/api-types';
import {
  type DirectoryGroupMembership,
  useCreateDirectoryGroupMembership,
  useDeleteDirectoryGroupMembership,
  useDirectoryGroupMemberships,
} from '@/hooks/useDirectory';

type NewMembership = {
  group_id: string;
  member_kind: DirectoryGroupMembershipCreateRequestMember_kind;
  member_id: string;
  role: DirectoryGroupMembershipCreateRequestRole;
};

export default function GroupMembershipsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { canMutate } = useCompany();
  const { data: memberships = [], isLoading: loading } = useDirectoryGroupMemberships(companyId);
  const [filter, setFilter] = useState('');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createError, setCreateError] = useState('');
  const [newMembership, setNewMembership] = useState<NewMembership>({
    group_id: '',
    member_kind: DirectoryGroupMembershipCreateRequestMember_kind.user,
    member_id: '',
    role: DirectoryGroupMembershipCreateRequestRole.member,
  });
  const [creating, setCreating] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DirectoryGroupMembership | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const createMembership = useCreateDirectoryGroupMembership();
  const deleteMembership = useDeleteDirectoryGroupMembership();

  const errMessage = (e: unknown, fallback: string) =>
    e instanceof Error && e.message ? e.message : fallback;

  const handleCreate = async () => {
    if (!newMembership.group_id.trim() || !newMembership.member_id.trim()) return;
    setCreateError('');
    setCreating(true);
    try {
      if (!companyId) return;
      await createMembership.mutateAsync({
        companyId,
        data: {
          group_id: newMembership.group_id,
          member_kind: newMembership.member_kind,
          member_id: newMembership.member_id,
          role: newMembership.role,
        },
      });
      setShowCreateModal(false);
      setNewMembership({
        group_id: '',
        member_kind: DirectoryGroupMembershipCreateRequestMember_kind.user,
        member_id: '',
        role: DirectoryGroupMembershipCreateRequestRole.member,
      });
      setFlash([{ type: 'success', content: t('pages.groups.member_added', 'Member added.'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } catch (e) {
      setCreateError(errMessage(e, t('pages.groups.create_failed', 'Failed to add member.')));
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (membership: DirectoryGroupMembership) => {
    setDeleteError('');
    setDeleting(true);
    try {
      if (!companyId) return;
      await deleteMembership.mutateAsync({ id: membership.id, companyId });
      setDeleteTarget(null);
      setFlash([{ type: 'success', content: t('pages.groups.member_removed', 'Member removed.'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } catch (e) {
      setDeleteError(errMessage(e, t('pages.groups.delete_failed', 'Failed to remove member.')));
    } finally {
      setDeleting(false);
    }
  };

  const memberKindOptions = [
    { label: t('pages.groups.member_kind_user'), value: 'user' },
    { label: t('pages.groups.member_kind_group'), value: 'group' },
  ];

  // Values must match the backend group roles (member|manager|owner).
  const roleOptions = [
    { label: t('pages.groups.role_member'), value: 'member' },
    { label: t('pages.groups.role_owner'), value: 'owner' },
    { label: t('pages.groups.role_manager', 'Manager'), value: 'manager' },
  ];

  const filteredMemberships = useMemo(() => memberships.filter(
    (m) =>
      m.group_id.toLowerCase().includes(filter.toLowerCase()) ||
      m.member_id.toLowerCase().includes(filter.toLowerCase())
  ), [memberships, filter]);

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.groups.title')}</Header>}>
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
          description={t('pages.groups.description')}
          actions={
            canMutate ? (
              <Button variant="primary" onClick={() => { setCreateError(''); setShowCreateModal(true); }}>
                {t('pages.groups.add_member')}
              </Button>
            ) : undefined
          }
        >
          {t('pages.groups.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {flash.length > 0 && <Flashbar items={flash} />}
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.groups.group_id'),
              cell: (item: DirectoryGroupMembership) => item.group_id,
              width: '25%',
            },
            {
              header: t('pages.groups.member_id'),
              cell: (item: DirectoryGroupMembership) => (
                <SpaceBetween size="xxxs">
                  <Box fontWeight="bold">{item.member_id}</Box>
                  <Box color="text-body-secondary" fontSize="body-s">{item.member_kind}</Box>
                </SpaceBetween>
              ),
              width: '30%',
            },
            {
              header: t('pages.groups_page.role'),
              cell: (item: DirectoryGroupMembership) => {
                const label = item.role === 'manager'
                  ? t('pages.groups.role_manager', 'Manager')
                  : item.role === 'owner'
                    ? t('pages.groups.role_owner')
                    : t('pages.groups.role_member');
                return (
                  <Badge color={item.role === 'manager' ? 'red' : item.role === 'owner' ? 'blue' : 'grey'}>
                    {label}
                  </Badge>
                );
              },
              width: '20%',
            },
            {
              header: t('pages.groups.status'),
              cell: (item: DirectoryGroupMembership) => item.status || '—',
              width: '15%',
            },
            {
              header: t('common.actions'),
              cell: (item: DirectoryGroupMembership) =>
                canMutate ? (
                  <Button
                    variant="inline-link"
                    onClick={() => { setDeleteError(''); setDeleteTarget(item); }}
                    loading={deleting && deleteTarget?.id === item.id}
                  >
                    {t('common.delete')}
                  </Button>
                ) : null,
              width: '10%',
            },
          ]}
          items={filteredMemberships}
          header={
            <Header variant="h2" counter={`(${filteredMemberships.length})`}>
              {t('pages.groups_page.memberships')}
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
              {t('pages.groups.no_members')}
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
                disabled={!newMembership.group_id.trim() || !newMembership.member_id.trim()}
              >
                {t('pages.groups.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.groups.create_modal_title')}
      >
        <SpaceBetween size="m">
          {createError && <Alert type="error">{createError}</Alert>}
          <FormField label={t('pages.groups.group_id_label')}>
            <Input
              value={newMembership.group_id}
              onChange={(e) => setNewMembership({ ...newMembership, group_id: e.detail.value })}
              placeholder="group-id"
            />
          </FormField>
          <FormField label={t('pages.groups.member_kind_label')}>
            <Select
              selectedOption={
                memberKindOptions.find((o) => o.value === newMembership.member_kind) ??
                memberKindOptions[0]
              }
              options={memberKindOptions}
                onChange={(e) =>
                  setNewMembership({
                    ...newMembership,
                    member_kind: e.detail.selectedOption.value as DirectoryGroupMembershipCreateRequestMember_kind,
                  })
                }
                expandToViewport
            />
          </FormField>
          <FormField label={t('pages.groups.member_id_label')}>
            <Input
              value={newMembership.member_id}
              onChange={(e) => setNewMembership({ ...newMembership, member_id: e.detail.value })}
              placeholder="user-id or group-id"
            />
          </FormField>
          <FormField label={t('pages.groups.role_label')}>
            <Select
              selectedOption={
                roleOptions.find((o) => o.value === newMembership.role) ?? roleOptions[0]
              }
              options={roleOptions}
                onChange={(e) =>
                  setNewMembership({
                    ...newMembership,
                    role: e.detail.selectedOption.value as DirectoryGroupMembershipCreateRequestRole,
                  })
                }
              expandToViewport
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      <ConfirmModal
        visible={!!deleteTarget}
        header={t('pages.groups.remove_modal_title', 'Remove member')}
        onConfirm={() => deleteTarget && handleDelete(deleteTarget)}
        onDismiss={() => { setDeleteTarget(null); setDeleteError(''); }}
        loading={deleting}
        confirmLabel={t('common.delete')}
        error={deleteError || undefined}
      >
        {t('pages.groups.remove_confirm', 'Remove')}{' '}
        <strong>{deleteTarget?.member_id}</strong>{' '}
        {t('pages.groups.remove_confirm_suffix', 'from the group')}{' '}
        <strong>{deleteTarget?.group_id}</strong>?
      </ConfirmModal>
    </ContentLayout>
  );
}
