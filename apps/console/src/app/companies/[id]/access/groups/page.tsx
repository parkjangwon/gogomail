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
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

interface GroupMembership {
  ID: string;
  GroupID: string;
  CompanyID: string;
  MemberKind: string;
  MemberID: string;
  Role: string;
  Status: string;
}

type NewMembership = {
  group_id: string;
  member_kind: string;
  member_id: string;
  role: string;
};

export default function GroupMembershipsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const [memberships, setMemberships] = useState<GroupMembership[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newMembership, setNewMembership] = useState<NewMembership>({
    group_id: '',
    member_kind: 'user',
    member_id: '',
    role: 'member',
  });
  const [creating, setCreating] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    fetchMemberships();
  }, []);

  const fetchMemberships = async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/directory/group-memberships?company_id=${companyId}&limit=100`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setMemberships(data.directory_group_memberships || []);
      }
    } catch (error) {
      console.error('Failed to fetch group memberships:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newMembership.group_id.trim() || !newMembership.member_id.trim()) return;
    setCreating(true);
    try {
      const res = await fetch('/api/admin/directory/group-memberships', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          group_id: newMembership.group_id,
          member_kind: newMembership.member_kind,
          member_id: newMembership.member_id,
          role: newMembership.role,
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowCreateModal(false);
        setNewMembership({ group_id: '', member_kind: 'user', member_id: '', role: 'member' });
        fetchMemberships();
      } else {
        console.error('Failed to create group membership:', await res.text());
      }
    } catch (error) {
      console.error('Failed to create group membership:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      const res = await fetch(`/api/admin/directory/group-memberships/${id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (res.ok) {
        fetchMemberships();
      } else {
        console.error('Failed to delete group membership:', await res.text());
      }
    } catch (error) {
      console.error('Failed to delete group membership:', error);
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
    { label: t('pages.groups.role_admin'), value: 'admin' },
  ];

  const filteredMemberships = memberships.filter(
    (m) =>
      m.GroupID.toLowerCase().includes(filter.toLowerCase()) ||
      m.MemberID.toLowerCase().includes(filter.toLowerCase())
  );

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
              cell: (item: GroupMembership) => item.GroupID,
              width: '25%',
            },
            {
              header: t('pages.groups.member_id'),
              cell: (item: GroupMembership) => (
                <SpaceBetween size="xxxs">
                  <Box fontWeight="bold">{item.MemberID}</Box>
                  <Box color="text-body-secondary" fontSize="body-s">{item.MemberKind}</Box>
                </SpaceBetween>
              ),
              width: '30%',
            },
            {
              header: t('pages.groups_page.role'),
              cell: (item: GroupMembership) => (
                <Badge color={item.Role === 'admin' ? 'red' : item.Role === 'owner' ? 'blue' : 'grey'}>
                  {item.Role}
                </Badge>
              ),
              width: '20%',
            },
            {
              header: t('pages.groups.status'),
              cell: (item: GroupMembership) => item.Status || '—',
              width: '15%',
            },
            {
              header: t('common.actions'),
              cell: (item: GroupMembership) => (
                <Button
                  variant="inline-link"
                  onClick={() => handleDelete(item.ID)}
                  loading={deletingId === item.ID}
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
                  member_kind: e.detail.selectedOption.value ?? 'user',
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
                  role: e.detail.selectedOption.value ?? 'member',
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
