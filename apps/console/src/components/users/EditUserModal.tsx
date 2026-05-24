'use client';

import { Dispatch, SetStateAction, useState } from 'react';
import {
  Box, Button, FormField, Input, Modal, Select,
  SpaceBetween, ExpandableSection, StatusIndicator, Spinner,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import {
  useUserOrgMemberships, useOrgUnits,
  useAssignOrgMember, useUpdateOrgMember, useRemoveOrgMember,
  type OrgMembership,
} from '@/hooks/useOrganization';

interface SelectOption {
  label: string;
  value: string;
}

export interface EditUserFormState {
  display_name: string;
  recovery_email: string;
  quota_gb: string;
  role: string;
}

interface EditUserModalProps {
  visible: boolean;
  userId: string;
  companyId: string;
  username: string;
  editForm: EditUserFormState;
  setEditForm: Dispatch<SetStateAction<EditUserFormState>>;
  saving: boolean;
  roleOptions: SelectOption[];
  onDismiss: () => void;
  onSave: () => void;
}

const ORG_ROLE_OPTIONS: SelectOption[] = [
  { label: 'member', value: 'member' },
  { label: 'manager', value: 'manager' },
  { label: 'admin', value: 'admin' },
];

function OrgMemberRow({
  m, userId, onUpdated, onRemoved,
}: {
  m: OrgMembership;
  userId: string;
  onUpdated: () => void;
  onRemoved: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(m.title);
  const [role, setRole] = useState(m.role);
  const updateMutation = useUpdateOrgMember();
  const removeMutation = useRemoveOrgMember();

  const handleSave = async () => {
    await updateMutation.mutateAsync({ memberId: m.member_id, userId, title, role });
    setEditing(false);
    onUpdated();
  };

  const handleRemove = async () => {
    await removeMutation.mutateAsync({ memberId: m.member_id, userId });
    onRemoved();
  };

  if (editing) {
    return (
      <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', padding: '4px 0' }}>
        <span style={{ fontWeight: 500, minWidth: '100px' }}>{m.unit_name}</span>
        <Input value={title} onChange={e => setTitle(e.detail.value)} placeholder="직책" />
        <Select
          selectedOption={ORG_ROLE_OPTIONS.find(o => o.value === role) ?? ORG_ROLE_OPTIONS[0]}
          options={ORG_ROLE_OPTIONS}
          onChange={e => setRole(e.detail.selectedOption.value ?? 'member')}
        />
        <Button variant="primary" onClick={handleSave} loading={updateMutation.isPending}>저장</Button>
        <Button onClick={() => { setTitle(m.title); setRole(m.role); setEditing(false); }}>취소</Button>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', gap: '8px', alignItems: 'center', padding: '4px 0', flexWrap: 'wrap' }}>
      <span style={{ fontWeight: 500, minWidth: '100px' }}>{m.unit_name}</span>
      <span style={{ color: 'var(--color-text-body-secondary)', flex: 1 }}>
        {m.title ? `${m.title} · ` : ''}{m.role}
        {m.is_primary && <StatusIndicator type="success" colorOverride="grey"> 기본</StatusIndicator>}
      </span>
      <Button variant="inline-link" onClick={() => setEditing(true)}>편집</Button>
      <Button variant="inline-link" onClick={handleRemove} loading={removeMutation.isPending}>제거</Button>
    </div>
  );
}

function OrgMembershipSection({ userId, companyId }: { userId: string; companyId: string }) {
  const membershipsQuery = useUserOrgMemberships(userId, !!userId);
  const unitsQuery = useOrgUnits(companyId, !!companyId);
  const assignMutation = useAssignOrgMember();

  const [addOpen, setAddOpen] = useState(false);
  const [newUnitId, setNewUnitId] = useState('');
  const [newTitle, setNewTitle] = useState('');
  const [newRole, setNewRole] = useState('member');

  const memberships = membershipsQuery.data ?? [];
  const units = unitsQuery.data ?? [];

  const unitOptions = units.map(u => ({
    label: u.display_name || u.name,
    value: u.id,
  }));

  const handleAdd = async () => {
    if (!newUnitId) return;
    await assignMutation.mutateAsync({ unitId: newUnitId, userId, role: newRole, title: newTitle });
    setAddOpen(false);
    setNewUnitId('');
    setNewTitle('');
    setNewRole('member');
  };

  return (
    <ExpandableSection
      headerText="조직도 소속"
      defaultExpanded={memberships.length > 0}
    >
      <SpaceBetween size="xs">
        {membershipsQuery.isLoading ? (
          <Spinner />
        ) : memberships.length === 0 ? (
          <Box color="text-body-secondary">소속된 조직이 없습니다.</Box>
        ) : (
          memberships.map(m => (
            <OrgMemberRow
              key={m.member_id}
              m={m}
              userId={userId}
              onUpdated={() => membershipsQuery.refetch()}
              onRemoved={() => membershipsQuery.refetch()}
            />
          ))
        )}

        {!addOpen ? (
          <Button variant="inline-link" onClick={() => setAddOpen(true)}>
            + 조직 단위 추가
          </Button>
        ) : (
          <div style={{ display: 'flex', gap: '8px', alignItems: 'flex-end', flexWrap: 'wrap', paddingTop: '4px' }}>
            <FormField label="조직 단위">
              <Select
                placeholder="조직 단위 선택"
                selectedOption={unitOptions.find(o => o.value === newUnitId) ?? null}
                options={unitOptions}
                onChange={e => setNewUnitId(e.detail.selectedOption.value ?? '')}
                statusType={unitsQuery.isLoading ? 'loading' : 'finished'}
                empty="조직 단위 없음"
              />
            </FormField>
            <FormField label="직책">
              <Input value={newTitle} onChange={e => setNewTitle(e.detail.value)} placeholder="예: 팀장, 수석 개발자" />
            </FormField>
            <FormField label="역할">
              <Select
                selectedOption={ORG_ROLE_OPTIONS.find(o => o.value === newRole) ?? ORG_ROLE_OPTIONS[0]}
                options={ORG_ROLE_OPTIONS}
                onChange={e => setNewRole(e.detail.selectedOption.value ?? 'member')}
              />
            </FormField>
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="primary" onClick={handleAdd} loading={assignMutation.isPending} disabled={!newUnitId}>추가</Button>
              <Button onClick={() => { setAddOpen(false); setNewUnitId(''); setNewTitle(''); setNewRole('member'); }}>취소</Button>
            </SpaceBetween>
          </div>
        )}
      </SpaceBetween>
    </ExpandableSection>
  );
}

export function EditUserModal({
  visible,
  userId,
  companyId,
  username,
  editForm,
  setEditForm,
  saving,
  roleOptions,
  onDismiss,
  onSave,
}: EditUserModalProps) {
  const { t } = useI18n();

  return (
    <Modal
      onDismiss={onDismiss}
      visible={visible}
      size="medium"
      footer={
        <Box float="right">
          <SpaceBetween direction="horizontal" size="xs">
            <Button onClick={onDismiss}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={onSave} loading={saving}>
              {t('pages.users_page.save_btn')}
            </Button>
          </SpaceBetween>
        </Box>
      }
      header={`${t('pages.users_page.edit_modal_title')} — ${username}`}
    >
      <SpaceBetween size="m">
        <FormField label={t('pages.users_page.display_name_label')}>
          <Box color="text-body-secondary">{editForm.display_name || '—'}</Box>
        </FormField>
        <FormField
          label={t('pages.users_page.recovery_email_label')}
          description={t('pages.users_page.recovery_email_description')}
        >
          <Input
            type="email"
            value={editForm.recovery_email}
            onChange={(e) => setEditForm({ ...editForm, recovery_email: e.detail.value })}
            placeholder={t('pages.users_page.recovery_email_placeholder')}
          />
        </FormField>
        <FormField label={t('pages.users_page.quota_label')}>
          <Input
            type="number"
            value={editForm.quota_gb}
            onChange={(e) => setEditForm({ ...editForm, quota_gb: e.detail.value })}
          />
        </FormField>
        <FormField
          label={t('pages.users_page.role')}
          description={t('pages.users_page.admin_role_desc')}
        >
          <Select
            selectedOption={roleOptions.find((o) => o.value === editForm.role) ?? roleOptions[0]}
            options={roleOptions}
            onChange={(e) => setEditForm({ ...editForm, role: e.detail.selectedOption.value ?? 'user' })}
          />
        </FormField>

        {visible && userId && (
          <OrgMembershipSection userId={userId} companyId={companyId} />
        )}
      </SpaceBetween>
    </Modal>
  );
}
