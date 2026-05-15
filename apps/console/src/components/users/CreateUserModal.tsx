'use client';

import { Dispatch, SetStateAction } from 'react';
import { Alert, Box, Button, FormField, Input, Modal, Select, SpaceBetween, CopyToClipboard } from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import type { CreateUserDraft } from '@/lib/users/userPageUtils';

interface SelectOption {
  label: string;
  value: string;
  description?: string;
}

interface CreateUserModalProps {
  visible: boolean;
  newUser: CreateUserDraft;
  setNewUser: Dispatch<SetStateAction<CreateUserDraft>>;
  inviteLink: string;
  createError: string;
  loadingDomainSettings: boolean;
  creating: boolean;
  registrationMode: 'temp_password' | 'email_invite';
  domainOptions: SelectOption[];
  autoAddress: string;
  onDismiss: () => void;
  onCloseAfterInvite: () => void;
  onDomainChange: (domainId: string) => void;
  onCreate: () => void;
}

export function CreateUserModal({
  visible,
  newUser,
  setNewUser,
  inviteLink,
  createError,
  loadingDomainSettings,
  creating,
  registrationMode,
  domainOptions,
  autoAddress,
  onDismiss,
  onCloseAfterInvite,
  onDomainChange,
  onCreate,
}: CreateUserModalProps) {
  const { t } = useI18n();

  return (
    <Modal
      onDismiss={onDismiss}
      visible={visible}
      size="medium"
      footer={
        inviteLink ? (
          <Box float="right">
            <Button onClick={onCloseAfterInvite}>{t('common.close')}</Button>
          </Box>
        ) : (
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={onDismiss}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={onCreate}
                loading={creating}
                disabled={!newUser.username.trim() || !newUser.domain_id.trim() || loadingDomainSettings}
              >
                {registrationMode === 'email_invite'
                  ? t('pages.users_page.create_and_invite_btn')
                  : t('pages.users_page.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        )
      }
      header={t('pages.users_page.create_modal_title')}
    >
      <SpaceBetween size="m">
        {inviteLink ? (
          <SpaceBetween size="m">
            <Alert type="success">{t('pages.users_page.invite_created')}</Alert>
            <FormField label={t('pages.users_page.invite_link_label')}
              description={t('pages.users_page.invite_link_desc')}>
              <CopyToClipboard
                copyButtonText={t('pages.users_page.copy_invite')}
                copySuccessText={t('pages.users_page.copy_success')}
                copyErrorText={t('pages.users_page.copy_error')}
                textToCopy={inviteLink}
              />
            </FormField>
            <Box color="text-body-secondary" fontSize="body-s">
              {inviteLink}
            </Box>
          </SpaceBetween>
        ) : (
          <>
            <FormField label={t('pages.users_page.domain_label')}>
              <Select
                selectedOption={domainOptions.find((o) => o.value === newUser.domain_id) ?? null}
                options={domainOptions}
                onChange={(e) => onDomainChange(e.detail.selectedOption?.value ?? '')}
                placeholder={t('pages.users_page.domain_placeholder')}
                statusType={loadingDomainSettings ? 'loading' : 'finished'}
                expandToViewport
              />
            </FormField>

            {newUser.domain_id && (
              <Alert type="info">
                {registrationMode === 'email_invite'
                  ? t('pages.users_page.mode_email_invite_info')
                  : t('pages.users_page.mode_temp_password_info')}
              </Alert>
            )}

            <FormField label={t('pages.users_page.username_label')}>
              <Input
                value={newUser.username}
                onChange={(e) => setNewUser({ ...newUser, username: e.detail.value })}
                placeholder="john.doe"
              />
            </FormField>
            <FormField label={t('pages.users_page.display_name_label')}>
              <Input
                value={newUser.display_name}
                onChange={(e) => setNewUser({ ...newUser, display_name: e.detail.value })}
                placeholder={t('pages.users_page.display_name_placeholder')}
              />
            </FormField>

            {autoAddress && (
              <FormField label={t('pages.users_page.address_label')}>
                <Box color="text-body-secondary">{autoAddress}</Box>
              </FormField>
            )}

            {registrationMode === 'temp_password' && (
              <FormField
                label={t('pages.users_page.password_label')}
                description={t('pages.users_page.temp_password_desc')}
              >
                <Input
                  type="password"
                  value={newUser.password}
                  onChange={(e) => setNewUser({ ...newUser, password: e.detail.value })}
                />
              </FormField>
            )}

            <FormField
              label={t('pages.users_page.recovery_email_label')}
              description={t('pages.users_page.recovery_email_description')}
            >
              <Input
                type="email"
                value={newUser.recovery_email}
                onChange={(e) => setNewUser({ ...newUser, recovery_email: e.detail.value })}
                placeholder={t('pages.users_page.recovery_email_placeholder')}
              />
            </FormField>

            <FormField label={t('pages.users_page.quota_label')} description={t('pages.users_page.quota_description')}>
              <Input
                type="number"
                value={newUser.quota_gb}
                onChange={(e) => setNewUser({ ...newUser, quota_gb: e.detail.value })}
              />
            </FormField>

            {createError && <Alert type="error">{createError}</Alert>}
          </>
        )}
      </SpaceBetween>
    </Modal>
  );
}
