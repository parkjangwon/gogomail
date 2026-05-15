'use client';
import { DataTable } from '@/components/DataTable';


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
  Badge,
  StatusIndicator,
  Modal,
  FormField,
  Input,
  Textarea,
  Select,
  SelectProps,
} from '@cloudscape-design/components';
import { useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import { useCompanyChangeHistory, usePendingApprovals, useCreatePendingApproval, useApprovePendingApproval, useRejectPendingApproval, type CompanyApproval } from '@/hooks';

const resultType = (r: string): 'success' | 'error' | 'pending' =>
  r === 'success' ? 'success' : r === 'error' ? 'error' : 'pending';

export default function ChangeHistoryPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;
  const [categoryFilter, setCategoryFilter] = useState('');
  const historyQuery = useCompanyChangeHistory(cid, categoryFilter || undefined);
  const approvalsQuery = usePendingApprovals(cid);
  const createApproval = useCreatePendingApproval();
  const approveApproval = useApprovePendingApproval();
  const rejectApproval = useRejectPendingApproval();
  const logs = historyQuery.data ?? [];
  const approvals = approvalsQuery.data ?? [];
  const loadingLogs = historyQuery.isLoading;
  const loadingApprovals = approvalsQuery.isLoading;
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [reviewModal, setReviewModal] = useState<{ item: CompanyApproval; action: 'approve' | 'reject' } | null>(null);
  const [reviewComment, setReviewComment] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createForm, setCreateForm] = useState<CompanyApproval>({ title: '', description: '', category: 'config', requested_by: '' });
  const [creating, setCreating] = useState(false);

  const categoryOptions: SelectProps.Option[] = [
    { label: t('pages.change_history_page.all_categories'), value: '' },
    { label: t('pages.change_history_page.category_config'), value: 'config' },
    { label: t('pages.change_history_page.category_security'), value: 'security' },
    { label: t('pages.change_history_page.category_user'), value: 'user' },
    { label: t('pages.change_history_page.category_domain'), value: 'domain' },
  ];

  const categoryCreateOptions = categoryOptions.filter(o => o.value);

  const handleReview = async () => {
    if (!reviewModal) return;
    setSubmitting(true);
    try {
      if (reviewModal.action === 'approve') {
        await approveApproval.mutateAsync({ companyId: cid!, approvalId: reviewModal.item.id!, data: { comment: reviewComment } });
      } else {
        await rejectApproval.mutateAsync({ companyId: cid!, approvalId: reviewModal.item.id!, data: { comment: reviewComment } });
      }
      setFlash([{
        type: 'success',
        content: reviewModal.action === 'approve'
          ? t('pages.change_history_page.request_approved')
          : t('pages.change_history_page.request_rejected'),
        dismissible: true,
        onDismiss: () => setFlash([]),
      }]);
      setReviewModal(null);
      setReviewComment('');
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setSubmitting(false);
    }
  };

  const handleCreate = async () => {
    setCreating(true);
    try {
      await createApproval.mutateAsync({ companyId: cid!, data: createForm });
      setFlash([{ type: 'success', content: t('pages.change_history_page.approval_submitted'), dismissible: true, onDismiss: () => setFlash([]) }]);
      setShowCreateModal(false);
      setCreateForm({ title: '', description: '', category: 'config', requested_by: '' });
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
              label: t('pages.change_history_page.change_history'),
              id: 'history',
              content: (
                <Container
                  header={
                    <Header
                      variant="h2"
                      actions={
                        <SpaceBetween size="xs" direction="horizontal">
                          <Select
                            selectedOption={categoryOptions.find(o => o.value === categoryFilter) ?? categoryOptions[0]}
                            options={categoryOptions}
                            onChange={({ detail }) => setCategoryFilter(detail.selectedOption.value ?? '')}
                          />
                          <Button iconName="refresh" onClick={() => historyQuery.refetch()} loading={loadingLogs}>{t('common.refresh')}</Button>
                        </SpaceBetween>
                      }
                    >
                      {t('pages.change_history_page.audit_trail')} ({logs.length})
                    </Header>
                  }
                >
                  {loadingLogs ? <Spinner /> : (
                    <DataTable
                      items={logs}
                      columnDefinitions={[
                        { id: 'time', header: t('pages.change_history_page.time'), cell: (i) => new Date(i.created_at ?? Date.now()).toLocaleString(), width: 160 },
                        { id: 'actor', header: t('pages.change_history_page.actor'), cell: (i) => i.actor_id || '—' },
                        { id: 'action', header: t('pages.change_history_page.action'), cell: (i) => <Box variant="code">{i.action ?? '—'}</Box> },
                        { id: 'category', header: t('pages.change_history_page.category'), cell: (i) => <Badge color="blue">{i.category ?? '—'}</Badge> },
                        { id: 'target', header: t('pages.change_history_page.target'), cell: (i) => i.target_type ? `${i.target_type}:${i.target_id ?? ''}` : '—' },
                        { id: 'result', header: t('pages.change_history_page.result'), cell: (i) => <StatusIndicator type={resultType(i.result ?? 'pending')}>{i.result ?? 'pending'}</StatusIndicator> },
                      ]}
                      empty={<Box textAlign="center" color="inherit">{t('pages.change_history_page.no_changes')}</Box>}
                    />
                  )}
                </Container>
              ),
            },
            {
              label: t('pages.change_history_page.pending_approvals'),
              id: 'approvals',
              content: (
                <Container
                  header={
                    <Header
                      variant="h2"
                      actions={
                        <SpaceBetween size="xs" direction="horizontal">
                          <Button onClick={() => setShowCreateModal(true)}>{t('pages.change_history_page.request_approval')}</Button>
                          <Button iconName="refresh" onClick={() => approvalsQuery.refetch()} loading={loadingApprovals}>{t('common.refresh')}</Button>
                        </SpaceBetween>
                      }
                    >
                      {t('pages.change_history_page.pending')} ({approvals.length})
                    </Header>
                  }
                >
                  {loadingApprovals ? <Spinner /> : (
                    <DataTable
                      items={approvals}
                      columnDefinitions={[
                        { id: 'title', header: t('pages.change_history_page.change_request'), cell: (i) => i.title ?? '—' },
                        { id: 'category', header: t('pages.change_history_page.category'), cell: (i) => <Badge color="blue">{i.category ?? '—'}</Badge> },
                        { id: 'requested_by', header: t('pages.change_history_page.requested_by'), cell: (i) => i.requested_by || '—' },
                        { id: 'requested_at', header: t('pages.change_history_page.submitted'), cell: (i) => new Date(i.requested_at ?? Date.now()).toLocaleString() },
                        {
                          id: 'actions', header: t('common.actions'),
                          cell: (i) => (
                            <SpaceBetween size="xs" direction="horizontal">
                              <Button variant="inline-link" onClick={() => { setReviewModal({ item: i, action: 'approve' }); setReviewComment(''); }}>{t('pages.change_history_page.approve')}</Button>
                              <Button variant="inline-link" onClick={() => { setReviewModal({ item: i, action: 'reject' }); setReviewComment(''); }}>{t('pages.change_history_page.reject')}</Button>
                            </SpaceBetween>
                          ),
                        },
                      ]}
                      empty={<Box textAlign="center" color="inherit">{t('pages.change_history_page.no_pending_approvals')}</Box>}
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
            header={`${reviewModal.action === 'approve' ? t('pages.change_history_page.approve') : t('pages.change_history_page.reject')}: ${reviewModal.item.title}`}
            footer={
              <Box float="right">
                <SpaceBetween size="xs" direction="horizontal">
                  <Button variant="link" onClick={() => setReviewModal(null)}>{t('common.cancel')}</Button>
                  <Button
                    variant={reviewModal.action === 'approve' ? 'primary' : 'normal'}
                    loading={submitting}
                    onClick={handleReview}
                  >
                    {reviewModal.action === 'approve' ? t('pages.change_history_page.approve') : t('pages.change_history_page.reject')}
                  </Button>
                </SpaceBetween>
              </Box>
            }
          >
            <SpaceBetween size="m">
              <Box>{reviewModal.item.description ?? '—'}</Box>
              <FormField label={t('pages.change_history_page.comment')}>
                <Textarea value={reviewComment} onChange={({ detail }) => setReviewComment(detail.value)} rows={3} />
              </FormField>
            </SpaceBetween>
          </Modal>
        )}

        <Modal
          visible={showCreateModal}
          onDismiss={() => setShowCreateModal(false)}
          header={t('pages.change_history_page.request_approval')}
          footer={
            <Box float="right">
              <SpaceBetween size="xs" direction="horizontal">
                <Button variant="link" onClick={() => setShowCreateModal(false)}>{t('common.cancel')}</Button>
                <Button variant="primary" loading={creating} onClick={handleCreate}>{t('pages.change_history_page.submit')}</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <FormField label={t('pages.change_history_page.entry_title')} constraintText={t('pages.change_history_page.title_hint')}>
              <Input value={createForm.title ?? ''} onChange={({ detail }) => setCreateForm(f => ({ ...f, title: detail.value }))} />
            </FormField>
            <FormField label={t('pages.change_history_page.change_description')}>
              <Textarea value={createForm.description ?? ''} onChange={({ detail }) => setCreateForm(f => ({ ...f, description: detail.value }))} rows={4} />
            </FormField>
            <FormField label={t('pages.change_history_page.category')}>
              <Select
                selectedOption={categoryCreateOptions.find(o => o.value === createForm.category) ?? categoryCreateOptions[0]!}
                options={categoryCreateOptions}
                onChange={({ detail }) => setCreateForm(f => ({ ...f, category: detail.selectedOption.value ?? 'config' }))}
              />
            </FormField>
            <FormField label={t('pages.change_history_page.requested_by')} constraintText={t('pages.change_history_page.requested_by_hint')}>
              <Input value={createForm.requested_by ?? ''} onChange={({ detail }) => setCreateForm(f => ({ ...f, requested_by: detail.value }))} />
            </FormField>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
