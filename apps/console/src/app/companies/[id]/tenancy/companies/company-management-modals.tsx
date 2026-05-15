'use client';

import {
  Modal,
  Box,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Alert,
} from '@cloudscape-design/components';
import type { Company } from '@/hooks';

type CompanyDraft = { name: string; quota_gb: string };
type TFunc = (key: string) => string;

interface Props {
  t: TFunc;
  showCreateModal: boolean;
  onDismissCreate: () => void;
  newCompany: CompanyDraft;
  onChangeNewCompany: (next: CompanyDraft) => void;
  onCreate: () => void;
  creating: boolean;
  editTarget: Company | null;
  onDismissEdit: () => void;
  editForm: CompanyDraft;
  onChangeEditForm: (next: CompanyDraft) => void;
  onSaveEdit: () => void;
  saving: boolean;
  saveError: string;
  deleteTarget: Company | null;
  onDismissDelete: () => void;
  onDelete: () => void;
  deleting: boolean;
  deleteError: string;
}

export function CompanyManagementModals({
  t,
  showCreateModal,
  onDismissCreate,
  newCompany,
  onChangeNewCompany,
  onCreate,
  creating,
  editTarget,
  onDismissEdit,
  editForm,
  onChangeEditForm,
  onSaveEdit,
  saving,
  saveError,
  deleteTarget,
  onDismissDelete,
  onDelete,
  deleting,
  deleteError,
}: Props) {
  return (
    <>
      <Modal
        onDismiss={onDismissCreate}
        visible={showCreateModal}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={onDismissCreate}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={onCreate}
                loading={creating}
                disabled={!newCompany.name.trim()}
              >
                {t('pages.companies.create_company_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.companies.create_modal_title')}
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.companies.company_name')} constraintText={t('pages.companies.name_constraint')}>
            <Input
              value={newCompany.name}
              onChange={(e) => onChangeNewCompany({ ...newCompany, name: e.detail.value })}
              placeholder={t('pages.companies.name_placeholder')}
              autoFocus
            />
          </FormField>
          <FormField label={t('pages.companies.quota_label')} description={t('pages.companies.quota_desc')}>
            <Input
              type="number"
              value={newCompany.quota_gb}
              onChange={(e) => onChangeNewCompany({ ...newCompany, quota_gb: e.detail.value })}
              placeholder={t('pages.companies.quota_placeholder')}
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      <Modal
        visible={!!editTarget}
        onDismiss={onDismissEdit}
        header={`${t('common.edit') || '회사 수정'} — ${editTarget?.name ?? ''}`}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={onDismissEdit}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={onSaveEdit} loading={saving} disabled={!editForm.name.trim()}>
                {t('common.save') || '저장'}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.companies.company_name')}>
            <Input
              value={editForm.name}
              onChange={(e) => onChangeEditForm({ ...editForm, name: e.detail.value })}
              placeholder={t('pages.companies.name_placeholder')}
            />
          </FormField>
          <FormField label={t('pages.companies.quota_label') || '스토리지 할당량 (GB)'} description="0 = 무제한">
            <Input
              type="number"
              value={editForm.quota_gb}
              onChange={(e) => onChangeEditForm({ ...editForm, quota_gb: e.detail.value })}
              placeholder="0 = 무제한"
            />
          </FormField>
          {saveError ? <Alert type="error">{saveError}</Alert> : null}
        </SpaceBetween>
      </Modal>

      <Modal
        visible={!!deleteTarget}
        onDismiss={onDismissDelete}
        header={t('common.delete') || '회사 삭제'}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={onDismissDelete}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={onDelete} loading={deleting}>
                {t('common.delete') || '삭제'}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <Box><strong>{deleteTarget?.name}</strong> 회사를 삭제하시겠습니까? 도메인이 있는 경우 삭제할 수 없습니다.</Box>
          {deleteError ? <Alert type="error">{deleteError}</Alert> : null}
        </SpaceBetween>
      </Modal>
    </>
  );
}
