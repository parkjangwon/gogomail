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
import { useI18n } from '@/app/i18n-provider';

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
            <Button variant="primary" onClick={() => setCreateVisible(true)}>
              {t('legal_holds.create_hold')}
            </Button>
          }
        >
          {t('legal_holds.title')}
        </Header>
      }
    >
      <Table
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
            cell: (item: LegalHold) => (
              <Button variant="inline-link" onClick={() => setDeleteTarget(item)}>
                {t('legal_holds.release')}
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
        onDismiss={() => setDeleteTarget(null)}
        size="small"
        header={t('legal_holds.release_modal')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setDeleteTarget(null)}>{t('common.cancel')}</Button>
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
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
