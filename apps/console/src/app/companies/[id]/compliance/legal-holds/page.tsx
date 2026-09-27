'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Modal,
  Form,
  FormField,
  Input,
  StatusIndicator,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface LegalHold {
  id: string;
  user_id: string;
  user_email: string;
  reason: string;
  created_at: string;
  created_by: string;
}

export default function LegalHoldsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const { canMutate } = useCompany();

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
  const [deleteError, setDeleteError] = useState('');

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
    if (!userEmail || !reason) { setSaveError(t('legal_holds.required_error')); return; }
    setSaving(true);
    setSaveError('');
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/legal-holds`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_email: userEmail, reason }),
      });
      if (!res.ok) { setSaveError((await res.json()).error || t('legal_holds.create_failed')); return; }
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
    setDeleteError('');
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/legal-holds/${deleteTarget.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (!res.ok) {
        // Compliance-critical: a failed release must NOT look like success.
        const data = (await res.json().catch(() => ({}))) as { error?: { message?: string } | string };
        const msg = typeof data.error === 'string' ? data.error : data.error?.message;
        setDeleteError(msg || t('legal_holds.release_failed', 'Failed to release legal hold.'));
        return;
      }
      setDeleteTarget(null);
      setSelected([]);
      fetchHolds();
    } catch (e) {
      setDeleteError(String(e));
    } finally {
      setDeleting(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('legal_holds.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('legal_holds.description')}
          actions={
            canMutate ? (
              <Button variant="primary" onClick={() => setCreateVisible(true)}>
                {t('legal_holds.create_hold')}
              </Button>
            ) : undefined
          }
        >
          {t('legal_holds.title')}
        </Header>
      }
    >
      <DataTable
        columnDefinitions={[
          {
            header: t('legal_holds.user'),
            cell: (item: LegalHold) => item.user_email || item.user_id,
            width: '25%',
          },
          {
            header: t('legal_holds.reason'),
            cell: (item: LegalHold) => item.reason,
            width: '35%',
          },
          {
            header: t('legal_holds.created_by'),
            cell: (item: LegalHold) => item.created_by || '—',
            width: '15%',
          },
          {
            header: t('legal_holds.created_at'),
            cell: (item: LegalHold) => item.created_at ? new Date(item.created_at).toLocaleString() : '—',
            width: '15%',
          },
          {
            header: '',
            cell: (item: LegalHold) =>
              canMutate ? (
                <Button variant="inline-link" onClick={() => { setDeleteError(''); setDeleteTarget(item); }}>
                  {t('legal_holds.release')}
                </Button>
              ) : null,
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
              canMutate && selected.length > 0 && (
                <Button variant="normal" onClick={() => { setDeleteError(''); setDeleteTarget(selected[0]); }}>
                  {t('legal_holds.release_hold')}
                </Button>
              )
            }
          >
            {t('legal_holds.active_holds')}
          </Header>
        }
        empty={
          <Box textAlign="center" padding="l" color="text-body-secondary">
            {t('legal_holds.empty_prefix')} <strong>{t('legal_holds.create_hold')}</strong> {t('legal_holds.empty_suffix')}
          </Box>
        }
      />

      {/* Create modal */}
      <Modal
        visible={createVisible}
        onDismiss={() => { setCreateVisible(false); setSaveError(''); }}
        size="medium"
        header={t('legal_holds.create_modal')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setCreateVisible(false)}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={handleCreate} loading={saving}>{t('common.create')}</Button>
            </SpaceBetween>
          </Box>
        }
      >
        <Form errorText={saveError}>
          <SpaceBetween size="m">
            <FormField label={t('legal_holds.user_email')} description={t('legal_holds.user_email_desc')}>
              <Input
                value={userEmail}
                onChange={e => setUserEmail(e.detail.value)}
                placeholder="user@company.com"
              />
            </FormField>
            <FormField label={t('legal_holds.reason')} description={t('legal_holds.reason_desc')}>
              <Input
                value={reason}
                onChange={e => setReason(e.detail.value)}
                placeholder={t('legal_holds.reason_placeholder')}
              />
            </FormField>
          </SpaceBetween>
        </Form>
      </Modal>

      {/* Delete confirmation */}
      <Modal
        visible={!!deleteTarget}
        onDismiss={() => { setDeleteTarget(null); setDeleteError(''); }}
        size="small"
        header={t('legal_holds.release_modal')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => { setDeleteTarget(null); setDeleteError(''); }} disabled={deleting}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={handleDelete} loading={deleting}>{t('legal_holds.release')}</Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="s">
          <StatusIndicator type="warning">{t('legal_holds.cannot_undo')}</StatusIndicator>
          <Box>
            {t('legal_holds.release_confirm_prefix')} <strong>{deleteTarget?.user_email}</strong>?
            {t('legal_holds.release_confirm_suffix')}
          </Box>
          {deleteError && <Alert type="error">{deleteError}</Alert>}
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
