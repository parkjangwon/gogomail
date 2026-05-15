'use client';

import { Box, Button, Alert, Modal, SpaceBetween } from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import type { User } from '@/lib/users/userUtils';

interface OffboardUserModalProps {
  visible: boolean;
  targetUser: User | null;
  offboarding: boolean;
  onDismiss: () => void;
  onConfirm: () => void;
}

export function OffboardUserModal({
  visible,
  targetUser,
  offboarding,
  onDismiss,
  onConfirm,
}: OffboardUserModalProps) {
  const { t } = useI18n();

  return (
    <Modal
      visible={visible}
      onDismiss={onDismiss}
      size="medium"
      header={`${t('pages.users_page.offboard_title')} — ${targetUser?.username ?? ''}`}
      footer={
        <Box float="right">
          <SpaceBetween direction="horizontal" size="xs">
            <Button onClick={onDismiss}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={onConfirm} loading={offboarding}>
              {t('pages.users_page.suspend_user')}
            </Button>
          </SpaceBetween>
        </Box>
      }
    >
      <SpaceBetween size="m">
        <Alert type="warning">
          {t('pages.users_page.offboard_warning_prefix')} <strong>{targetUser?.username}</strong>{' '}
          {t('pages.users_page.offboard_warning_suffix')}
        </Alert>
        <Alert type="info">
          {t('pages.users_page.offboard_alias_prefix')} <strong>{t('pages.users_page.access_aliases')}</strong>{' '}
          {t('pages.users_page.offboard_alias_middle')} <strong>{targetUser?.username}</strong>{' '}
          {t('pages.users_page.offboard_alias_suffix')}
        </Alert>
      </SpaceBetween>
    </Modal>
  );
}
