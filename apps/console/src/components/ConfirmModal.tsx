'use client';

import { Modal, Box, SpaceBetween, Button, Alert } from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';

interface ConfirmModalProps {
  /** Whether the modal is shown. */
  visible: boolean;
  /** Modal title. */
  header: string;
  /** Body content — typically a sentence naming the target. */
  children: React.ReactNode;
  /** Called when the user confirms the action. */
  onConfirm: () => void;
  /** Called when the modal is dismissed/cancelled. */
  onDismiss: () => void;
  /** Shows a spinner on the confirm button and blocks re-clicks. */
  loading?: boolean;
  /** Label for the confirm button. Defaults to the shared "Delete" label. */
  confirmLabel?: string;
  /**
   * Server/validation error to surface inside the still-open modal on failure.
   * When set, the dialog stays open so the user can read it and retry.
   */
  error?: string;
}

/**
 * Shared Cloudscape confirmation dialog for destructive actions. Keeps the
 * confirm → run → (success closes / failure keeps open + shows error) pattern
 * consistent across the console. Callers are responsible for only clearing
 * their context (closing the modal) on success.
 */
export function ConfirmModal({
  visible,
  header,
  children,
  onConfirm,
  onDismiss,
  loading = false,
  confirmLabel,
  error,
}: ConfirmModalProps) {
  const { t } = useI18n();
  return (
    <Modal
      visible={visible}
      onDismiss={onDismiss}
      size="small"
      header={header}
      footer={
        <Box float="right">
          <SpaceBetween direction="horizontal" size="xs">
            <Button variant="link" onClick={onDismiss} disabled={loading}>
              {t('common.cancel')}
            </Button>
            <Button variant="primary" onClick={onConfirm} loading={loading}>
              {confirmLabel ?? t('common.delete')}
            </Button>
          </SpaceBetween>
        </Box>
      }
    >
      <SpaceBetween size="s">
        <Box>{children}</Box>
        {error && <Alert type="error">{error}</Alert>}
      </SpaceBetween>
    </Modal>
  );
}
