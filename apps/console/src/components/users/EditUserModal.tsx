'use client';

import { Dispatch, SetStateAction } from 'react';
import { Box, Button, FormField, Input, Modal, Select, SpaceBetween } from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';

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
  username: string;
  editForm: EditUserFormState;
  setEditForm: Dispatch<SetStateAction<EditUserFormState>>;
  saving: boolean;
  roleOptions: SelectOption[];
  onDismiss: () => void;
  onSave: () => void;
}

export function EditUserModal({
  visible,
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
      </SpaceBetween>
    </Modal>
  );
}
