'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Modal,
  Form,
  FormField,
  Input,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';

interface LegalHold {
  id: string;
  user_id: string;
  user_email: string;
  reason: string;
  created_at: string;
  created_by: string;
}

export default function LegalHoldsPage() {
  const params = useParams();
  const companyId = params?.id as string;

  const [holds, setHolds] = useState<LegalHold[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<LegalHold[]>([]);

  const [createVisible, setCreateVisible] = useState(false);
  const [userEmail, setUserEmail] = useState('');
  const [reason, setReason] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');

  const [deleteTarget, setDeleteTarget] = useState<LegalHold | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchHolds = async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/legal-holds`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setHolds(data.holds || []);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (companyId) fetchHolds();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [companyId]);

  const handleCreate = async () => {
    if (!userEmail || !reason) { setSaveError('User email and reason are required.'); return; }
    setSaving(true);
    setSaveError('');
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/legal-holds`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_email: userEmail, reason }),
      });
      if (!res.ok) { setSaveError((await res.json()).error || 'Failed to create hold'); return; }
      setCreateVisible(false);
      setUserEmail('');
      setReason('');
      fetchHolds();
    } catch (e) {
      setSaveError(String(e));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await fetch(`/api/admin/companies/${companyId}/legal-holds/${deleteTarget.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      setDeleteTarget(null);
      setSelected([]);
      fetchHolds();
    } finally {
      setDeleting(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Legal Holds</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description="Preserve user mailboxes from deletion for compliance or litigation purposes."
          actions={
            <Button variant="primary" onClick={() => setCreateVisible(true)}>
              Create Hold
            </Button>
          }
        >
          Legal Holds
        </Header>
      }
    >
      <Table
        columnDefinitions={[
          {
            header: 'User',
            cell: (item: LegalHold) => item.user_email || item.user_id,
            width: '25%',
          },
          {
            header: 'Reason',
            cell: (item: LegalHold) => item.reason,
            width: '35%',
          },
          {
            header: 'Created By',
            cell: (item: LegalHold) => item.created_by || '—',
            width: '15%',
          },
          {
            header: 'Created At',
            cell: (item: LegalHold) => item.created_at ? new Date(item.created_at).toLocaleString() : '—',
            width: '15%',
          },
          {
            header: '',
            cell: (item: LegalHold) => (
              <Button variant="inline-link" onClick={() => setDeleteTarget(item)}>
                Release
              </Button>
            ),
            width: '10%',
          },
        ]}
        items={holds}
        selectionType="single"
        selectedItems={selected}
        onSelectionChange={e => setSelected(e.detail.selectedItems)}
        header={
          <Header
            variant="h2"
            counter={`(${holds.length})`}
            actions={
              selected.length > 0 && (
                <Button variant="normal" onClick={() => setDeleteTarget(selected[0])}>
                  Release Hold
                </Button>
              )
            }
          >
            Active Holds
          </Header>
        }
        empty={
          <Box textAlign="center" padding="l" color="text-body-secondary">
            No legal holds. Click <strong>Create Hold</strong> to preserve a user&apos;s mailbox.
          </Box>
        }
      />

      {/* Create modal */}
      <Modal
        visible={createVisible}
        onDismiss={() => { setCreateVisible(false); setSaveError(''); }}
        size="medium"
        header="Create Legal Hold"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setCreateVisible(false)}>Cancel</Button>
              <Button variant="primary" onClick={handleCreate} loading={saving}>Create</Button>
            </SpaceBetween>
          </Box>
        }
      >
        <Form errorText={saveError}>
          <SpaceBetween size="m">
            <FormField label="User Email" description="The user whose mailbox will be preserved.">
              <Input
                value={userEmail}
                onChange={e => setUserEmail(e.detail.value)}
                placeholder="user@company.com"
              />
            </FormField>
            <FormField label="Reason" description="Legal or compliance justification for the hold.">
              <Input
                value={reason}
                onChange={e => setReason(e.detail.value)}
                placeholder="Litigation — case #12345"
              />
            </FormField>
          </SpaceBetween>
        </Form>
      </Modal>

      {/* Delete confirmation */}
      <Modal
        visible={!!deleteTarget}
        onDismiss={() => setDeleteTarget(null)}
        size="small"
        header="Release Legal Hold"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setDeleteTarget(null)}>Cancel</Button>
              <Button variant="primary" onClick={handleDelete} loading={deleting}>Release</Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="s">
          <StatusIndicator type="warning">This action cannot be undone.</StatusIndicator>
          <Box>
            Release the hold for <strong>{deleteTarget?.user_email}</strong>?
            The mailbox will no longer be preserved and may be subject to normal retention policies.
          </Box>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
