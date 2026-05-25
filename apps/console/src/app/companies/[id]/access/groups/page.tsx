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
  Select,
} from '@cloudscape-design/components';
import { useState, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';
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
  const { data: memberships = [], isLoading: loading } = useDirectoryGroupMemberships(companyId);
  const [filter, setFilter] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newMembership, setNewMembership] = useState<NewMembership>({
    group_id: '',
    member_kind: DirectoryGroupMembershipCreateRequestMember_kind.user,
    member_id: '',
    role: DirectoryGroupMembershipCreateRequestRole.member,
  });
  const [creating, setCreating] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const createMembership = useCreateDirectoryGroupMembership();
  const deleteMembership = useDeleteDirectoryGroupMembership();

  const handleCreate = async () => {
    if (!newMembership.group_id.trim() || !newMembership.member_id.trim()) return;
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
    } catch {
      // mutation error handled by caller
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      if (!companyId) return;
      await deleteMembership.mutateAsync({
        id,
        companyId,
      });
    } catch {
      // mutation error handled by caller
    } finally {
      setDeletingId(null);
    }
  };

  const memberKindOptions = [
    { label: t('pages.groups.member_kind_user'), value: 'user' },
    { label: t('pages.groups.member_kind_group'), value: 'group' },
  ];

  const roleOptions = [
    { label: t('pages.groups.role_member'), value: 'member' },
    { label: t('pages.groups.role_owner'), value: 'owner' },
    { label: t('pages.groups.role_admin'), value: 'manager' },
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
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              {t('pages.groups.add_member')}
            </Button>
          }
        >
          {t('pages.groups.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
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
              cell: (item: DirectoryGroupMembership) => (
                <Badge color={item.role === 'manager' ? 'red' : item.role === 'owner' ? 'blue' : 'grey'}>
                  {item.role}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: t('pages.groups.status'),
              cell: (item: DirectoryGroupMembership) => item.status || '—',
              width: '15%',
            },
            {
              header: t('common.actions'),
              cell: (item: DirectoryGroupMembership) => (
                <Button
                  variant="inline-link"
                  onClick={() => handleDelete(item.id)}
                  loading={deletingId === item.id}
                >
                  {t('common.delete')}
                </Button>
              ),
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
    </ContentLayout>
  );
}
