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
  Select,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';

interface Delegation {
  ID: string;
  CompanyID: string;
  OwnerKind: string;
  OwnerID: string;
  DelegateKind: string;
  DelegateID: string;
  Scope: string;
  Role: string;
  Status: string;
}

type NewDelegation = {
  owner_kind: string;
  owner_id: string;
  delegate_kind: string;
  delegate_id: string;
  scope: string;
  role: string;
};

export default function DelegationsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [delegations, setDelegations] = useState<Delegation[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newDelegation, setNewDelegation] = useState<NewDelegation>({
    owner_kind: 'user',
    owner_id: '',
    delegate_kind: 'user',
    delegate_id: '',
    scope: '',
    role: 'viewer',
  });
  const [creating, setCreating] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    fetchDelegations();
  }, []);

  const fetchDelegations = async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/directory/delegations?company_id=${companyId}&limit=100`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setDelegations(data.directory_delegations || []);
      }
    } catch (error) {
      console.error('Failed to fetch delegations:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newDelegation.owner_id.trim() || !newDelegation.delegate_id.trim()) return;
    setCreating(true);
    try {
      const res = await fetch('/api/admin/directory/delegations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          company_id: companyId,
          owner_kind: newDelegation.owner_kind,
          owner_id: newDelegation.owner_id,
          delegate_kind: newDelegation.delegate_kind,
          delegate_id: newDelegation.delegate_id,
          scope: newDelegation.scope,
          role: newDelegation.role,
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowCreateModal(false);
        setNewDelegation({
          owner_kind: 'user',
          owner_id: '',
          delegate_kind: 'user',
          delegate_id: '',
          scope: '',
          role: 'viewer',
        });
        fetchDelegations();
      } else {
        console.error('Failed to create delegation:', await res.text());
      }
    } catch (error) {
      console.error('Failed to create delegation:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      const res = await fetch(`/api/admin/directory/delegations/${id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (res.ok) {
        fetchDelegations();
      } else {
        console.error('Failed to delete delegation:', await res.text());
      }
    } catch (error) {
      console.error('Failed to delete delegation:', error);
    } finally {
      setDeletingId(null);
    }
  };

  const principalKindOptions = [
    { label: t('pages.delegations.kind_user'), value: 'user' },
    { label: t('pages.delegations.kind_group'), value: 'group' },
  ];

  const roleOptions = [
    { label: t('pages.delegations.role_viewer'), value: 'viewer' },
    { label: t('pages.delegations.role_editor'), value: 'editor' },
    { label: t('pages.delegations.role_admin'), value: 'admin' },
  ];

  const filteredDelegations = delegations.filter(
    (d) =>
      d.OwnerID.toLowerCase().includes(filter.toLowerCase()) ||
      d.DelegateID.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.delegations.title')}</Header>}>
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
          description={t('pages.delegations.description')}
          actions={
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              {t('pages.delegations.create_delegation')}
            </Button>
          }
        >
          {t('pages.delegations.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.delegations.owner_id'),
              cell: (item: Delegation) => (
                <SpaceBetween size="xxxs">
                  <Box fontWeight="bold">{item.OwnerID}</Box>
                  <Box color="text-body-secondary" fontSize="body-s">{item.OwnerKind}</Box>
                </SpaceBetween>
              ),
              width: '22%',
            },
            {
              header: t('pages.delegations.delegate_id'),
              cell: (item: Delegation) => (
                <SpaceBetween size="xxxs">
                  <Box fontWeight="bold">{item.DelegateID}</Box>
                  <Box color="text-body-secondary" fontSize="body-s">{item.DelegateKind}</Box>
                </SpaceBetween>
              ),
              width: '22%',
            },
            {
              header: t('pages.delegations.scope'),
              cell: (item: Delegation) => item.Scope || '—',
              width: '25%',
            },
            {
              header: t('pages.delegations.role'),
              cell: (item: Delegation) => (
                <Badge color={item.Role === 'admin' ? 'red' : item.Role === 'editor' ? 'blue' : 'grey'}>
                  {item.Role}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: t('common.actions'),
              cell: (item: Delegation) => (
                <Button
                  variant="inline-link"
                  onClick={() => handleDelete(item.ID)}
                  loading={deletingId === item.ID}
                >
                  {t('common.delete')}
                </Button>
              ),
              width: '16%',
            },
          ]}
          items={filteredDelegations}
          header={
            <Header variant="h2" counter={`(${filteredDelegations.length})`}>
              {t('pages.delegations.title')}
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
              {t('pages.delegations.no_delegations')}
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
                disabled={!newDelegation.owner_id.trim() || !newDelegation.delegate_id.trim()}
              >
                {t('pages.delegations.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.delegations.create_modal_title')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.delegations.owner_kind_label')}>
            <Select
              selectedOption={
                principalKindOptions.find((o) => o.value === newDelegation.owner_kind) ??
                principalKindOptions[0]
              }
              options={principalKindOptions}
              onChange={(e) =>
                setNewDelegation({
                  ...newDelegation,
                  owner_kind: e.detail.selectedOption.value ?? 'user',
                })
              }
              expandToViewport
            />
          </FormField>
          <FormField label={t('pages.delegations.owner_id_label')}>
            <Input
              value={newDelegation.owner_id}
              onChange={(e) => setNewDelegation({ ...newDelegation, owner_id: e.detail.value })}
              placeholder="owner-id"
            />
          </FormField>
          <FormField label={t('pages.delegations.delegate_kind_label')}>
            <Select
              selectedOption={
                principalKindOptions.find((o) => o.value === newDelegation.delegate_kind) ??
                principalKindOptions[0]
              }
              options={principalKindOptions}
              onChange={(e) =>
                setNewDelegation({
                  ...newDelegation,
                  delegate_kind: e.detail.selectedOption.value ?? 'user',
                })
              }
              expandToViewport
            />
          </FormField>
          <FormField label={t('pages.delegations.delegate_id_label')}>
            <Input
              value={newDelegation.delegate_id}
              onChange={(e) => setNewDelegation({ ...newDelegation, delegate_id: e.detail.value })}
              placeholder="delegate-id"
            />
          </FormField>
          <FormField label={t('pages.delegations.scope_label')}>
            <Input
              value={newDelegation.scope}
              onChange={(e) => setNewDelegation({ ...newDelegation, scope: e.detail.value })}
              placeholder="mail:read"
            />
          </FormField>
          <FormField label={t('pages.delegations.role_label')}>
            <Select
              selectedOption={
                roleOptions.find((o) => o.value === newDelegation.role) ?? roleOptions[0]
              }
              options={roleOptions}
              onChange={(e) =>
                setNewDelegation({
                  ...newDelegation,
                  role: e.detail.selectedOption.value ?? 'viewer',
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
