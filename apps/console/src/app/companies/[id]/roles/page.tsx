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
  Badge,
} from '@cloudscape-design/components';
import { useState } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useCreateRole, useRoles, type Role as RoleItem } from '@/hooks/useRoles';

export default function RolesPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { data: roles = [], isLoading } = useRoles(companyId);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newRole, setNewRole] = useState({ name: '', description: '' });
  const createRole = useCreateRole();

  const handleCreate = async () => {
    if (!newRole.name.trim()) return;
    try {
      await createRole.mutateAsync({ companyId, data: newRole });
      setShowModal(false);
      setNewRole({ name: '', description: '' });
    } catch (error) {
      console.error('Failed to create role:', error);
    }
  };

  const filteredRoles = roles.filter((r: RoleItem) =>
    r.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.roles_page.title')}</Header>}>
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
          description={t('pages.roles_page.description')}
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              {t('pages.roles_page.create_role')}
            </Button>
          }
        >
          {t('pages.roles_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.roles_page.type'),
              cell: (item: RoleItem) => (
                <Badge color={item.is_builtin ? 'blue' : 'grey'}>
                  {item.is_builtin ? t('pages.roles_page.builtin_role') : t('pages.roles_page.custom_role')}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.roles_page.name'),
              cell: (item: RoleItem) => item.name,
              width: '20%',
            },
            {
              header: t('pages.roles_page.description_col'),
              cell: (item: RoleItem) => item.description,
              width: '35%',
            },
            {
              header: t('pages.roles_page.permissions'),
              cell: (item: RoleItem) => item.permissions_count,
              width: '15%',
            },
            {
              header: t('pages.roles_page.assigned_users'),
              cell: (item: RoleItem) => item.assigned_users,
              width: '15%',
            },
            {
              header: t('pages.roles_page.created'),
              cell: (item: RoleItem) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredRoles}
          header={
            <Header variant="h2" counter={`(${filteredRoles.length})`}>
              {t('pages.roles_page.roles')}
            </Header>
          }
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
              <Button
                variant="primary"
                onClick={handleCreate}
                loading={createRole.isPending}
                disabled={!newRole.name.trim()}
              >
                {t('pages.roles_page.create_role')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.roles_page.create_role')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.roles_page.name')}>
            <Input
              value={newRole.name}
              onChange={(e) => setNewRole({ ...newRole, name: e.detail.value })}
              placeholder="e.g. support-agent"
            />
          </FormField>
          <FormField label={t('pages.roles_page.description_col')}>
            <Input
              value={newRole.description}
              onChange={(e) => setNewRole({ ...newRole, description: e.detail.value })}
              placeholder={t('pages.roles_page.description_placeholder')}
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
