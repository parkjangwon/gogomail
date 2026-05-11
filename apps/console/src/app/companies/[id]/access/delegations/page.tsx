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

const KIND_OPTIONS: SelectProps.Option[] = [
  { label: 'User', value: 'user' },
  { label: 'Group', value: 'group' },
  { label: 'Domain', value: 'domain' },
];

const ROLE_OPTIONS: SelectProps.Option[] = [
  { label: 'Viewer', value: 'viewer' },
  { label: 'Editor', value: 'editor' },
  { label: 'Admin', value: 'admin' },
  { label: 'Send-As', value: 'send_as' },
  { label: 'Send-On-Behalf', value: 'send_on_behalf' },
];

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

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const res = await fetch(`/admin/v1/directory/delegations?company_id=${cid}&limit=200`);
      const data = await res.json();
      setDelegations(data.directory_delegations ?? []);
    } catch {
      setFlash([{ type: 'error', content: 'Failed to load delegations', dismissible: true, onDismiss: () => setFlash([]) }]);
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
      setFlash([{ type: 'success', content: 'Delegation created', dismissible: true, onDismiss: () => setFlash([]) }]);
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
      setFlash([{ type: 'success', content: 'Delegation revoked', dismissible: true, onDismiss: () => setFlash([]) }]);
      load();
    } catch {
      setFlash([{ type: 'error', content: 'Failed to revoke', dismissible: true, onDismiss: () => setFlash([]) }]);
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
          description={`${delegations.length} total delegations`}
          actions={
            <SpaceBetween size="xs" direction="horizontal">
              <SegmentedControl
                selectedId={viewMode}
                onChange={({ detail }) => setViewMode(detail.selectedId as 'graph' | 'list')}
                options={[
                  { id: 'graph', text: 'Graph View' },
                  { id: 'list', text: 'List View' },
                ]}
              />
              <Button variant="primary" onClick={() => setShowModal(true)}>Grant Delegation</Button>
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
          filteringPlaceholder="Search by owner, delegate, scope, or role…"
          countText={`${filtered.length} delegations`}
        />

        {viewMode === 'graph' ? (
          <SpaceBetween size="s">
            {byOwner.size === 0 && (
              <Box textAlign="center" padding="xl" color="inherit">No delegations found</Box>
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
                      <Box color="text-status-inactive">→ {delegates.length} delegate{delegates.length !== 1 ? 's' : ''}</Box>
                    </SpaceBetween>
                  }
                >
                  <SpaceBetween size="xs">
                    {delegates.map(d => (
                      <Container key={d.ID}>
                        <ColumnLayout columns={5} variant="text-grid">
                          <div>
                            <Box variant="awsui-key-label">Delegate</Box>
                            <SpaceBetween size="xs" direction="horizontal">
                              <Badge color="grey">{d.DelegateKind}</Badge>
                              <span>{d.DelegateID}</span>
                            </SpaceBetween>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">Scope</Box>
                            <Box variant="code">{d.Scope || 'all'}</Box>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">Role</Box>
                            <Badge color={roleColor(d.Role)}>{d.Role}</Badge>
                          </div>
                          <div>
                            <Box variant="awsui-key-label">Status</Box>
                            <StatusIndicator type={statusType(d.Status)}>{d.Status}</StatusIndicator>
                          </div>
                          <div>
                            <Button
                              variant="inline-link"
                              loading={deletingId === d.ID}
                              onClick={() => handleDelete(d.ID)}
                            >
                              Revoke
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
                <Box textAlign="center" padding="xl" color="inherit">No delegations found</Box>
              )}
              {filtered.map(d => (
                <Box key={d.ID} padding="s">
                  <ColumnLayout columns={6} variant="text-grid">
                    <div>
                      <Box variant="awsui-key-label">Owner</Box>
                      <SpaceBetween size="xs" direction="horizontal">
                        <Badge color="grey">{d.OwnerKind}</Badge>
                        <span>{d.OwnerID}</span>
                      </SpaceBetween>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">Delegate</Box>
                      <SpaceBetween size="xs" direction="horizontal">
                        <Badge color="grey">{d.DelegateKind}</Badge>
                        <span>{d.DelegateID}</span>
                      </SpaceBetween>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">Scope</Box>
                      <Box variant="code">{d.Scope || 'all'}</Box>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">Role</Box>
                      <Badge color={roleColor(d.Role)}>{d.Role}</Badge>
                    </div>
                    <div>
                      <Box variant="awsui-key-label">Status</Box>
                      <StatusIndicator type={statusType(d.Status)}>{d.Status}</StatusIndicator>
                    </div>
                    <div>
                      <Button variant="inline-link" loading={deletingId === d.ID} onClick={() => handleDelete(d.ID)}>Revoke</Button>
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
          header="Grant Delegation"
          footer={
            <Box float="right">
              <SpaceBetween size="xs" direction="horizontal">
                <Button variant="link" onClick={() => setShowModal(false)}>Cancel</Button>
                <Button variant="primary" loading={creating} onClick={handleCreate}>Grant</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <ColumnLayout columns={2}>
              <FormField label="Owner Type">
                <Select
                  selectedOption={KIND_OPTIONS.find(o => o.value === form.owner_kind) ?? KIND_OPTIONS[0]}
                  options={KIND_OPTIONS}
                  onChange={({ detail }) => setForm(f => ({ ...f, owner_kind: detail.selectedOption.value ?? 'user' }))}
                />
              </FormField>
              <FormField label="Owner ID" constraintText="Email or group ID">
                <Input value={form.owner_id} onChange={({ detail }) => setForm(f => ({ ...f, owner_id: detail.value }))} />
              </FormField>
            </ColumnLayout>
            <ColumnLayout columns={2}>
              <FormField label="Delegate Type">
                <Select
                  selectedOption={KIND_OPTIONS.find(o => o.value === form.delegate_kind) ?? KIND_OPTIONS[0]}
                  options={KIND_OPTIONS}
                  onChange={({ detail }) => setForm(f => ({ ...f, delegate_kind: detail.selectedOption.value ?? 'user' }))}
                />
              </FormField>
              <FormField label="Delegate ID" constraintText="Who receives the permission">
                <Input value={form.delegate_id} onChange={({ detail }) => setForm(f => ({ ...f, delegate_id: detail.value }))} />
              </FormField>
            </ColumnLayout>
            <FormField label="Scope" constraintText="Mailbox path or folder — leave blank for all">
              <Input value={form.scope} placeholder="e.g. INBOX or leave blank" onChange={({ detail }) => setForm(f => ({ ...f, scope: detail.value }))} />
            </FormField>
            <FormField label="Role">
              <Select
                selectedOption={ROLE_OPTIONS.find(o => o.value === form.role) ?? ROLE_OPTIONS[0]}
                options={ROLE_OPTIONS}
                onChange={({ detail }) => setForm(f => ({ ...f, role: detail.selectedOption.value ?? 'viewer' }))}
              />
            </FormField>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
