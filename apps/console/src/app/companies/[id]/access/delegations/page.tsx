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
import { useState, useEffect, useCallback, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

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

const roleColor = (role: string): 'blue' | 'green' | 'red' | 'grey' => {
  if (role === 'admin') return 'red';
  if (role === 'editor' || role === 'send_as') return 'blue';
  if (role === 'viewer' || role === 'send_on_behalf') return 'green';
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

  const [delegations, setDelegations] = useState<Delegation[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [viewMode, setViewMode] = useState<'graph' | 'list'>('graph');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [showModal, setShowModal] = useState(false);
  const [creating, setCreating] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [form, setForm] = useState({
    owner_kind: 'user', owner_id: '',
    delegate_kind: 'user', delegate_id: '',
    scope: '', role: 'viewer',
  });

  const kindOptions: SelectProps.Option[] = [
    { label: t('pages.delegations_page.kind_user'), value: 'user' },
    { label: t('pages.delegations_page.kind_group'), value: 'group' },
    { label: t('pages.delegations_page.kind_domain'), value: 'domain' },
  ];

  const roleOptions: SelectProps.Option[] = [
    { label: t('pages.delegations_page.role_viewer'), value: 'viewer' },
    { label: t('pages.delegations_page.role_editor'), value: 'editor' },
    { label: t('pages.delegations_page.role_admin'), value: 'admin' },
    { label: t('pages.delegations_page.role_send_as'), value: 'send_as' },
    { label: t('pages.delegations_page.role_send_on_behalf'), value: 'send_on_behalf' },
  ];

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const res = await fetch(`/admin/v1/directory/delegations?company_id=${cid}&limit=200`);
      const data = await res.json();
      setDelegations(data.directory_delegations ?? []);
    } catch {
      setFlash([{ type: 'error', content: t('pages.delegations_page.failed_load'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setLoading(false);
    }
  }, [cid]);

  useEffect(() => { load(); }, [load]);

  const filtered = useMemo(() => {
    if (!filter) return delegations;
    const q = filter.toLowerCase();
    return delegations.filter(d =>
      d.OwnerID.toLowerCase().includes(q) ||
      d.DelegateID.toLowerCase().includes(q) ||
      d.Scope.toLowerCase().includes(q) ||
      d.Role.toLowerCase().includes(q)
    );
  }, [delegations, filter]);

  const byOwner = useMemo(() => {
    const map = new Map<string, Delegation[]>();
    for (const d of filtered) {
      const key = `${d.OwnerKind}:${d.OwnerID}`;
      if (!map.has(key)) map.set(key, []);
      map.get(key)!.push(d);
    }
    return map;
  }, [filtered]);

  const handleCreate = async () => {
    setCreating(true);
    try {
      const res = await fetch(`/admin/v1/directory/delegations`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...form, company_id: cid }),
      });
      if (!res.ok) throw new Error(await res.text());
      setFlash([{ type: 'success', content: t('pages.delegations_page.created'), dismissible: true, onDismiss: () => setFlash([]) }]);
      setShowModal(false);
      setForm({ owner_kind: 'user', owner_id: '', delegate_kind: 'user', delegate_id: '', scope: '', role: 'viewer' });
      load();
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      await fetch(`/admin/v1/directory/delegations/${id}`, { method: 'DELETE' });
      setFlash([{ type: 'success', content: t('pages.delegations_page.revoked'), dismissible: true, onDismiss: () => setFlash([]) }]);
      load();
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
                      <Container key={d.ID}>
                        <ColumnLayout columns={5} variant="text-grid">
                          <div>
                            <Box variant="awsui-key-label">{t('pages.delegations_page.delegate')}</Box>
                            <SpaceBetween size="xs" direction="horizontal">
                              <Badge color="grey">{d.DelegateKind}</Badge>
                              <span>{d.DelegateID}</span>
                            </SpaceBetween>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">{t('pages.delegations_page.scope')}</Box>
                            <Box variant="code">{d.Scope || 'all'}</Box>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">{t('pages.delegations_page.role')}</Box>
                            <Badge color={roleColor(d.Role)}>{d.Role}</Badge>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">{t('users.status')}</Box>
                            <StatusIndicator type={statusType(d.Status)}>{d.Status}</StatusIndicator>
                          </div>
                          <div>
                            <Button
                              variant="inline-link"
                              loading={deletingId === d.ID}
                              onClick={() => handleDelete(d.ID)}
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
                <Box key={d.ID} padding="s">
                  <ColumnLayout columns={6} variant="text-grid">
                    <div>
                      <Box variant="awsui-key-label">{t('pages.delegations_page.owner')}</Box>
                      <SpaceBetween size="xs" direction="horizontal">
                        <Badge color="grey">{d.OwnerKind}</Badge>
                        <span>{d.OwnerID}</span>
                      </SpaceBetween>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">{t('pages.delegations_page.delegate')}</Box>
                      <SpaceBetween size="xs" direction="horizontal">
                        <Badge color="grey">{d.DelegateKind}</Badge>
                        <span>{d.DelegateID}</span>
                      </SpaceBetween>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">{t('pages.delegations_page.scope')}</Box>
                      <Box variant="code">{d.Scope || 'all'}</Box>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">{t('pages.delegations_page.role')}</Box>
                      <Badge color={roleColor(d.Role)}>{d.Role}</Badge>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">{t('users.status')}</Box>
                      <StatusIndicator type={statusType(d.Status)}>{d.Status}</StatusIndicator>
                    </div>
                    <div>
                      <Button variant="inline-link" loading={deletingId === d.ID} onClick={() => handleDelete(d.ID)}>{t('pages.delegations_page.revoke')}</Button>
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
                  onChange={({ detail }) => setForm(f => ({ ...f, owner_kind: detail.selectedOption.value ?? 'user' }))}
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
                  onChange={({ detail }) => setForm(f => ({ ...f, delegate_kind: detail.selectedOption.value ?? 'user' }))}
                />
              </FormField>
              <FormField label={t('pages.delegations_page.delegate_id')} constraintText={t('pages.delegations_page.delegate_id_hint')}>
                <Input value={form.delegate_id} onChange={({ detail }) => setForm(f => ({ ...f, delegate_id: detail.value }))} />
              </FormField>
            </ColumnLayout>
            <FormField label={t('pages.delegations_page.scope')} constraintText={t('pages.delegations_page.scope_hint')}>
              <Input value={form.scope} placeholder={t('pages.delegations_page.scope_placeholder')} onChange={({ detail }) => setForm(f => ({ ...f, scope: detail.value }))} />
            </FormField>
            <FormField label={t('pages.delegations_page.role')}>
              <Select
                selectedOption={roleOptions.find(o => o.value === form.role) ?? roleOptions[0]}
                options={roleOptions}
                onChange={({ detail }) => setForm(f => ({ ...f, role: detail.selectedOption.value ?? 'viewer' }))}
              />
            </FormField>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
