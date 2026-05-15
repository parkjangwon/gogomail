'use client';

import type { RefObject } from 'react';
import { Alert, Box, Button, FormField, Modal, SpaceBetween, Spinner } from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';

interface ImportFailure {
  email: string;
  error: string;
}

export interface ImportUsersResult {
  total: number;
  success: number;
  failed: number;
  failures: ImportFailure[];
}

interface ImportUsersModalProps {
  visible: boolean;
  importing: boolean;
  importResult: ImportUsersResult | null;
  fileInputRef: RefObject<HTMLInputElement | null>;
  onDismiss: () => void;
  onImportFile: (file: File) => void;
}

export function ImportUsersModal({
  visible,
  importing,
  importResult,
  fileInputRef,
  onDismiss,
  onImportFile,
}: ImportUsersModalProps) {
  const { t } = useI18n();

  return (
    <Modal
      onDismiss={onDismiss}
      visible={visible}
      size="medium"
      header={t('pages.users_page.users_bulk.import_modal')}
      footer={
        <Box float="right">
          <Button onClick={onDismiss}>
            {t('pages.users_page.users_bulk.close')}
          </Button>
        </Box>
      }
    >
      <SpaceBetween size="m">
        <Alert type="info">
          {t('pages.users_page.users_bulk.format_hint')}
        </Alert>

        {!importResult && (
          <FormField label={t('pages.users_page.users_bulk.drop_or_click')}>
            <input
              ref={fileInputRef}
              type="file"
              accept=".csv"
              style={{ display: 'block' }}
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) onImportFile(file);
              }}
            />
            {importing && <Box padding={{ top: 's' }}><Spinner /> {t('pages.users_page.users_bulk.importing')}</Box>}
          </FormField>
        )}

        {importResult && (
          <SpaceBetween size="s">
            <Alert type={importResult.failed === 0 ? 'success' : 'warning'}>
              {t('pages.users_page.users_bulk.success_count').replace('{n}', String(importResult.success))}
              {importResult.failed > 0 && (
                <> &mdash; {t('pages.users_page.users_bulk.failed_count').replace('{n}', String(importResult.failed))}</>
              )}
            </Alert>
            {importResult.failures && importResult.failures.length > 0 && (
              <Alert type="error" header={t('pages.users_page.users_bulk.import_result')}>
                <ul style={{ margin: 0, paddingLeft: '1.2em' }}>
                  {importResult.failures.map((f, i) => (
                    <li key={i}><strong>{f.email}</strong>: {f.error}</li>
                  ))}
                </ul>
              </Alert>
            )}
          </SpaceBetween>
        )}
      </SpaceBetween>
    </Modal>
  );
}
