'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  Box,
  Spinner,
  Flashbar,
  FlashbarProps,
  Tabs,
  Table,
  Badge,
  StatusIndicator,
  Modal,
  FormField,
  Input,
  Textarea,
  Select,
  SelectProps,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface AuditLog {
  id: string;
  actor_id: string;
  category: string;
  action: string;
  target_type: string;
  target_id: string;
  result: string;
  created_at: string;
}

interface ApprovalItem {
  id: string;
  title: string;
  description: string;
  category: string;
  requested_by: string;
  requested_at: string;
  status: 'pending' | 'approved' | 'rejected';
  reviewed_by?: string;
  reviewed_at?: string;
  comment?: string;
}

const CATEGORY_OPTIONS: SelectProps.Option[] = [
  { label: 'All Categories', value: '' },
  { label: 'Config', value: 'config' },
  { label: 'Security', value: 'security' },
  { label: 'User', value: 'user' },
  { label: 'Domain', value: 'domain' },
];

const resultType = (r: string): 'success' | 'error' | 'pending' =>
  r === 'success' ? 'success' : r === 'error' ? 'error' : 'pending';

export default function ChangeHistoryPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;

  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [approvals, setApprovals] = useState<ApprovalItem[]>([]);
  const [loadingLogs, setLoadingLogs] = useState(true);
  const [loadingApprovals, setLoadingApprovals] = useState(true);
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [categoryFilter, setCategoryFilter] = useState('');
  const [reviewModal, setReviewModal] = useState<{ item: ApprovalItem; action: 'approve' | 'reject' } | null>(null);
  const [reviewComment, setReviewComment] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createForm, setCreateForm] = useState({ title: '', description: '', category: 'config', requested_by: '' });
  const [creating, setCreating] = useState(false);

  const loadLogs = useCallback(async () => {
    if (!cid) return;
    setLoadingLogs(true);
    try {
      const params = new URLSearchParams({ limit: '100' });
      if (categoryFilter) params.set('category', categoryFilter);
      const res = await fetch(`/admin/v1/companies/${cid}/change-history?${params}`);
      const data = await res.json();
      setLogs(data.changes ?? []);
    } catch {
      setFlash([{ type: 'error', content: 'Failed to load change history', dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setLoadingLogs(false);
    }
  }, [cid, categoryFilter]);

  const loadApprovals = useCallback(async (status = 'pending') => {
    if (!cid) return;
    setLoadingApprovals(true);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/pending-approvals?status=${status}`);
      const data = await res.json();
      setApprovals(data.approvals ?? []);
    } catch {
      setFlash([{ type: 'error', content: 'Failed to load approvals', dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setLoadingApprovals(false);
    }
  }, [cid]);

  useEffect(() => { loadLogs(); }, [loadLogs]);
  useEffect(() => { loadApprovals(); }, [loadApprovals]);

  const handleReview = async () => {
    if (!reviewModal) return;
    setSubmitting(true);
    try {
      const url = `/admin/v1/companies/${cid}/pending-approvals/${reviewModal.item.id}/${reviewModal.action}`;
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ comment: reviewComment }),
      });
      if (!res.ok) throw new Error(await res.text());
      setFlash([{ type: 'success', content: `Request ${reviewModal.action}d`, dismissible: true, onDismiss: () => setFlash([]) }]);
      setReviewModal(null);
      setReviewComment('');
      loadApprovals();
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setSubmitting(false);
    }
  };

  const handleCreate = async () => {
    setCreating(true);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/pending-approvals`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(createForm),
      });
      if (!res.ok) throw new Error(await res.text());
      setFlash([{ type: 'success', content: 'Approval request submitted', dismissible: true, onDismiss: () => setFlash([]) }]);
      setShowCreateModal(false);
      setCreateForm({ title: '', description: '', category: 'config', requested_by: '' });
      loadApprovals();
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setCreating(false);
    }
  };

  return (
    <ContentLayout header={<Header variant="h1">{t('nav.change_history')}</Header>}>
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}

        <Tabs
          tabs={[
            {
              label: 'Change History',
              id: 'history',
              content: (
                <Container
                  header={
                    <Header
                      variant="h2"
                      actions={
                        <SpaceBetween size="xs" direction="horizontal">
                          <Select
                            selectedOption={CATEGORY_OPTIONS.find(o => o.value === categoryFilter) ?? CATEGORY_OPTIONS[0]}
                            options={CATEGORY_OPTIONS}
                            onChange={({ detail }) => setCategoryFilter(detail.selectedOption.value ?? '')}
                          />
                          <Button iconName="refresh" onClick={loadLogs} loading={loadingLogs}>Refresh</Button>
                        </SpaceBetween>
                      }
                    >
                      Audit Trail ({logs.length})
                    </Header>
                  }
                >
                  {loadingLogs ? <Spinner /> : (
                    <Table
                      items={logs}
                      columnDefinitions={[
                        { id: 'time', header: 'Time', cell: (i) => new Date(i.created_at).toLocaleString(), width: 160 },
                        { id: 'actor', header: 'Actor', cell: (i) => i.actor_id || '—' },
                        { id: 'action', header: 'Action', cell: (i) => <Box variant="code">{i.action}</Box> },
                        { id: 'category', header: 'Category', cell: (i) => <Badge color="blue">{i.category}</Badge> },
                        { id: 'target', header: 'Target', cell: (i) => i.target_type ? `${i.target_type}:${i.target_id}` : '—' },
                        { id: 'result', header: 'Result', cell: (i) => <StatusIndicator type={resultType(i.result)}>{i.result}</StatusIndicator> },
                      ]}
                      empty={<Box textAlign="center" color="inherit">No changes recorded</Box>}
                    />
                  )}
                </Container>
              ),
            },
            {
              label: 'Pending Approvals',
              id: 'approvals',
              content: (
                <Container
                  header={
                    <Header
                      variant="h2"
                      actions={
                        <SpaceBetween size="xs" direction="horizontal">
                          <Button onClick={() => setShowCreateModal(true)}>Request Approval</Button>
                          <Button iconName="refresh" onClick={() => loadApprovals()} loading={loadingApprovals}>Refresh</Button>
                        </SpaceBetween>
                      }
                    >
                      Pending ({approvals.length})
                    </Header>
                  }
                >
                  {loadingApprovals ? <Spinner /> : (
                    <Table
                      items={approvals}
                      columnDefinitions={[
                        { id: 'title', header: 'Change Request', cell: (i) => i.title },
                        { id: 'category', header: 'Category', cell: (i) => <Badge color="blue">{i.category}</Badge> },
                        { id: 'requested_by', header: 'Requested By', cell: (i) => i.requested_by || '—' },
                        { id: 'requested_at', header: 'Submitted', cell: (i) => new Date(i.requested_at).toLocaleString() },
                        {
                          id: 'actions', header: 'Actions',
                          cell: (i) => (
                            <SpaceBetween size="xs" direction="horizontal">
                              <Button variant="inline-link" onClick={() => { setReviewModal({ item: i, action: 'approve' }); setReviewComment(''); }}>Approve</Button>
                              <Button variant="inline-link" onClick={() => { setReviewModal({ item: i, action: 'reject' }); setReviewComment(''); }}>Reject</Button>
                            </SpaceBetween>
                          ),
                        },
                      ]}
                      empty={<Box textAlign="center" color="inherit">No pending approvals</Box>}
                    />
                  )}
                </Container>
              ),
            },
          ]}
        />

        {reviewModal && (
          <Modal
            visible
            onDismiss={() => setReviewModal(null)}
            header={`${reviewModal.action === 'approve' ? 'Approve' : 'Reject'}: ${reviewModal.item.title}`}
            footer={
              <Box float="right">
                <SpaceBetween size="xs" direction="horizontal">
                  <Button variant="link" onClick={() => setReviewModal(null)}>Cancel</Button>
                  <Button
                    variant={reviewModal.action === 'approve' ? 'primary' : 'normal'}
                    loading={submitting}
                    onClick={handleReview}
                  >
                    {reviewModal.action === 'approve' ? 'Approve' : 'Reject'}
                  </Button>
                </SpaceBetween>
              </Box>
            }
          >
            <SpaceBetween size="m">
              <Box>{reviewModal.item.description}</Box>
              <FormField label={t('pages.change_history_page.comment')}>
                <Textarea value={reviewComment} onChange={({ detail }) => setReviewComment(detail.value)} rows={3} />
              </FormField>
            </SpaceBetween>
          </Modal>
        )}

        <Modal
          visible={showCreateModal}
          onDismiss={() => setShowCreateModal(false)}
          header="Request Approval"
          footer={
            <Box float="right">
              <SpaceBetween size="xs" direction="horizontal">
                <Button variant="link" onClick={() => setShowCreateModal(false)}>Cancel</Button>
                <Button variant="primary" loading={creating} onClick={handleCreate}>Submit</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <FormField label={t('pages.change_history_page.entry_title')} constraintText={t('pages.change_history_page.title_hint')}>
              <Input value={createForm.title} onChange={({ detail }) => setCreateForm(f => ({ ...f, title: detail.value }))} />
            </FormField>
            <FormField label={t('pages.change_history_page.change_description')}>
              <Textarea value={createForm.description} onChange={({ detail }) => setCreateForm(f => ({ ...f, description: detail.value }))} rows={4} />
            </FormField>
            <FormField label="Category">
              <Select
                selectedOption={{ label: createForm.category, value: createForm.category }}
                options={[
                  { label: 'config', value: 'config' },
                  { label: 'security', value: 'security' },
                  { label: 'domain', value: 'domain' },
                  { label: 'user', value: 'user' },
                ]}
                onChange={({ detail }) => setCreateForm(f => ({ ...f, category: detail.selectedOption.value ?? 'config' }))}
              />
            </FormField>
            <FormField label="Requested By" constraintText="Your name or email">
              <Input value={createForm.requested_by} onChange={({ detail }) => setCreateForm(f => ({ ...f, requested_by: detail.value }))} />
            </FormField>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
