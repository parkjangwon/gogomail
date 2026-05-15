'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  Box,
  Spinner,
  TextFilter,
  Badge,
  Modal,
  FormField,
  Input,
  Select,
  SelectProps,
  Flashbar,
  FlashbarProps,
  SegmentedControl,
  ExpandableSection,
  ColumnLayout,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import {
  DirectoryDelegationCreateRequestDelegate_kind,
  DirectoryDelegationCreateRequestOwner_kind,
  DirectoryDelegationCreateRequestRole,
  DirectoryDelegationCreateRequestScope,
} from '@gogomail/api-types';
import {
  type DirectoryDelegation,
  useCreateDirectoryDelegation,
  useDeleteDirectoryDelegation,
  useDirectoryDelegations,
} from '@/hooks/useDirectory';

const roleColor = (role: string): 'blue' | 'green' | 'red' | 'grey' => {
  if (role === 'manage') return 'red';
  if (role === 'write') return 'blue';
  if (role === 'read') return 'green';
  return 'grey';
};

const statusType = (status: string): 'success' | 'error' | 'pending' => {
  if (status === 'active') return 'success';
  if (status === 'revoked') return 'error';
  return 'pending';
};

export default function DelegationsPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;

  const { data: delegations = [], isLoading: loading } = useDirectoryDelegations(cid);
  const [filter, setFilter] = useState('');
  const [viewMode, setViewMode] = useState<'graph' | 'list'>('graph');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [showModal, setShowModal] = useState(false);
  const [creating, setCreating] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const createDelegation = useCreateDirectoryDelegation();
  const deleteDelegation = useDeleteDirectoryDelegation();

  const [form, setForm] = useState({
    owner_kind: DirectoryDelegationCreateRequestOwner_kind.user,
    owner_id: '',
    delegate_kind: DirectoryDelegationCreateRequestDelegate_kind.user,
    delegate_id: '',
    scope: DirectoryDelegationCreateRequestScope.mailbox,
    role: DirectoryDelegationCreateRequestRole.read,
  });

  const kindOptions: SelectProps.Option[] = [
    { label: t('pages.delegations_page.kind_user'), value: 'user' },
    { label: t('pages.delegations_page.kind_group'), value: 'group' },
    { label: t('pages.delegations_page.kind_domain'), value: 'organization' },
  ];

  const roleOptions: SelectProps.Option[] = [
    { label: t('pages.delegations_page.role_viewer'), value: 'read' },
    { label: t('pages.delegations_page.role_editor'), value: 'write' },
    { label: t('pages.delegations_page.role_admin'), value: 'manage' },
  ];

  const scopeOptions: SelectProps.Option[] = [
    { label: 'Mailbox', value: DirectoryDelegationCreateRequestScope.mailbox },
    { label: 'Calendar', value: DirectoryDelegationCreateRequestScope.calendar },
    { label: 'Contacts', value: DirectoryDelegationCreateRequestScope.contacts },
    { label: 'Drive', value: DirectoryDelegationCreateRequestScope.drive },
  ];

  const filtered = useMemo(() => {
    if (!filter) return delegations;
    const q = filter.toLowerCase();
    return delegations.filter(d =>
      d.owner_id.toLowerCase().includes(q) ||
      d.delegate_id.toLowerCase().includes(q) ||
      d.scope.toLowerCase().includes(q) ||
      d.role.toLowerCase().includes(q)
    );
  }, [delegations, filter]);

  const byOwner = useMemo(() => {
    const map = new Map<string, DirectoryDelegation[]>();
    for (const d of filtered) {
      const key = `${d.owner_kind}:${d.owner_id}`;
      if (!map.has(key)) map.set(key, []);
      map.get(key)!.push(d);
    }
    return map;
  }, [filtered]);

  const handleCreate = async () => {
    setCreating(true);
    try {
      if (!cid) return;
      await createDelegation.mutateAsync({
        companyId: cid,
        data: { ...form, company_id: cid },
      });
      setFlash([{ type: 'success', content: t('pages.delegations_page.created'), dismissible: true, onDismiss: () => setFlash([]) }]);
      setShowModal(false);
      setForm({
        owner_kind: DirectoryDelegationCreateRequestOwner_kind.user,
        owner_id: '',
        delegate_kind: DirectoryDelegationCreateRequestDelegate_kind.user,
        delegate_id: '',
        scope: DirectoryDelegationCreateRequestScope.mailbox,
        role: DirectoryDelegationCreateRequestRole.read,
      });
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      if (!cid) return;
      await deleteDelegation.mutateAsync({ id, companyId: cid });
      setFlash([{ type: 'success', content: t('pages.delegations_page.revoked'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } catch {
      setFlash([{ type: 'error', content: t('pages.delegations_page.failed_revoke'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setDeletingId(null);
    }
  };

  if (loading) return <Box padding="xl"><Spinner /></Box>;

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.delegations_page.total_delegations').replace('{n}', String(delegations.length))}
          actions={
            <SpaceBetween size="xs" direction="horizontal">
              <SegmentedControl
                selectedId={viewMode}
                onChange={({ detail }) => setViewMode(detail.selectedId as 'graph' | 'list')}
                options={[
                  { id: 'graph', text: t('pages.delegations_page.graph_view') },
                  { id: 'list', text: t('pages.delegations_page.list_view') },
                ]}
              />
              <Button variant="primary" onClick={() => setShowModal(true)}>{t('pages.delegations_page.grant_delegation')}</Button>
            </SpaceBetween>
          }
        >
          {t('nav.delegations')}
        </Header>
      }
    >
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}

        <TextFilter
          filteringText={filter}
          onChange={({ detail }) => setFilter(detail.filteringText)}
          filteringPlaceholder={t('pages.delegations_page.search_placeholder')}
          countText={t('pages.delegations_page.filtered_count').replace('{n}', String(filtered.length))}
        />

        {viewMode === 'graph' ? (
          <SpaceBetween size="s">
            {byOwner.size === 0 && (
              <Box textAlign="center" padding="xl" color="inherit">{t('pages.delegations_page.no_delegations')}</Box>
            )}
            {[...byOwner.entries()].map(([ownerKey, delegates]) => {
              const [ownerKind, ownerID] = ownerKey.split(':');
              return (
                <ExpandableSection
                  key={ownerKey}
                  defaultExpanded
                  headerText={
                    <SpaceBetween size="xs" direction="horizontal">
                      <Badge color="grey">{ownerKind}</Badge>
                      <Box fontWeight="bold">{ownerID}</Box>
                      <Box color="text-status-inactive">
                        → {t('pages.delegations_page.delegate_count').replace('{n}', String(delegates.length))}
                      </Box>
                    </SpaceBetween>
                  }
                >
                  <SpaceBetween size="xs">
                    {delegates.map(d => (
                      <Container key={d.id}>
                        <ColumnLayout columns={5} variant="text-grid">
                          <div>
                            <Box variant="awsui-key-label">{t('pages.delegations_page.delegate')}</Box>
                            <SpaceBetween size="xs" direction="horizontal">
                              <Badge color="grey">{d.delegate_kind}</Badge>
                              <span>{d.delegate_id}</span>
                            </SpaceBetween>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">{t('pages.delegations_page.scope')}</Box>
                            <Box variant="code">{d.scope || 'all'}</Box>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">{t('pages.delegations_page.role')}</Box>
                            <Badge color={roleColor(d.role)}>{d.role}</Badge>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">{t('users.status')}</Box>
                            <StatusIndicator type={statusType(d.status)}>{d.status}</StatusIndicator>
                          </div>
                          <div>
                            <Button
                              variant="inline-link"
                              loading={deletingId === d.id}
                              onClick={() => handleDelete(d.id)}
                            >
                              {t('pages.delegations_page.revoke')}
                            </Button>
                          </div>
                        </ColumnLayout>
                      </Container>
                    ))}
                  </SpaceBetween>
                </ExpandableSection>
              );
            })}
          </SpaceBetween>
        ) : (
          <Container>
            <SpaceBetween size="xs">
              {filtered.length === 0 && (
                <Box textAlign="center" padding="xl" color="inherit">{t('pages.delegations_page.no_delegations')}</Box>
              )}
              {filtered.map(d => (
                <Box key={d.id} padding="s">
                  <ColumnLayout columns={6} variant="text-grid">
                    <div>
                      <Box variant="awsui-key-label">{t('pages.delegations_page.owner')}</Box>
                      <SpaceBetween size="xs" direction="horizontal">
                        <Badge color="grey">{d.owner_kind}</Badge>
                        <span>{d.owner_id}</span>
                      </SpaceBetween>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">{t('pages.delegations_page.delegate')}</Box>
                      <SpaceBetween size="xs" direction="horizontal">
                        <Badge color="grey">{d.delegate_kind}</Badge>
                        <span>{d.delegate_id}</span>
                      </SpaceBetween>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">{t('pages.delegations_page.scope')}</Box>
                      <Box variant="code">{d.scope || 'all'}</Box>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">{t('pages.delegations_page.role')}</Box>
                      <Badge color={roleColor(d.role)}>{d.role}</Badge>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">{t('users.status')}</Box>
                      <StatusIndicator type={statusType(d.status)}>{d.status}</StatusIndicator>
                    </div>
                    <div>
                      <Button variant="inline-link" loading={deletingId === d.id} onClick={() => handleDelete(d.id)}>{t('pages.delegations_page.revoke')}</Button>
                    </div>
                  </ColumnLayout>
                </Box>
              ))}
            </SpaceBetween>
          </Container>
        )}

        <Modal
          visible={showModal}
          onDismiss={() => setShowModal(false)}
          header={t('pages.delegations_page.grant_delegation')}
          footer={
            <Box float="right">
              <SpaceBetween size="xs" direction="horizontal">
                <Button variant="link" onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
                <Button variant="primary" loading={creating} onClick={handleCreate}>{t('pages.delegations_page.grant')}</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <ColumnLayout columns={2}>
              <FormField label={t('pages.delegations_page.owner_type')}>
                <Select
                  selectedOption={kindOptions.find(o => o.value === form.owner_kind) ?? kindOptions[0]}
                  options={kindOptions}
                  onChange={({ detail }) => setForm(f => ({ ...f, owner_kind: detail.selectedOption.value as DirectoryDelegationCreateRequestOwner_kind }))}
                />
              </FormField>
              <FormField label={t('pages.delegations_page.owner_id')} constraintText={t('pages.delegations_page.owner_id_hint')}>
                <Input value={form.owner_id} onChange={({ detail }) => setForm(f => ({ ...f, owner_id: detail.value }))} />
              </FormField>
            </ColumnLayout>
            <ColumnLayout columns={2}>
              <FormField label={t('pages.delegations_page.delegate_type')}>
                <Select
                  selectedOption={kindOptions.find(o => o.value === form.delegate_kind) ?? kindOptions[0]}
                  options={kindOptions}
                  onChange={({ detail }) => setForm(f => ({ ...f, delegate_kind: detail.selectedOption.value as DirectoryDelegationCreateRequestDelegate_kind }))}
                />
              </FormField>
              <FormField label={t('pages.delegations_page.delegate_id')} constraintText={t('pages.delegations_page.delegate_id_hint')}>
                <Input value={form.delegate_id} onChange={({ detail }) => setForm(f => ({ ...f, delegate_id: detail.value }))} />
              </FormField>
            </ColumnLayout>
            <FormField label={t('pages.delegations_page.scope')} constraintText={t('pages.delegations_page.scope_hint')}>
              <Select
                selectedOption={scopeOptions.find(o => o.value === form.scope) ?? scopeOptions[0]}
                options={scopeOptions}
                onChange={({ detail }) => setForm(f => ({ ...f, scope: detail.selectedOption.value as DirectoryDelegationCreateRequestScope }))}
              />
            </FormField>
            <FormField label={t('pages.delegations_page.role')}>
              <Select
                selectedOption={roleOptions.find(o => o.value === form.role) ?? roleOptions[0]}
                options={roleOptions}
                onChange={({ detail }) => setForm(f => ({ ...f, role: detail.selectedOption.value as DirectoryDelegationCreateRequestRole }))}
              />
            </FormField>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
