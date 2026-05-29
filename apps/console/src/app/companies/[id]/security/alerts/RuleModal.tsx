'use client';

import {
  Box,
  Button,
  Checkbox,
  FormField,
  Input,
  Modal,
  Select,
  SpaceBetween,
} from '@cloudscape-design/components';
import type { SelectProps } from '@cloudscape-design/components';
import { CreateAlertRuleAlert_type } from '@gogomail/api-types';

export type RuleForm = {
  id?: string;
  name: string;
  alert_type: CreateAlertRuleAlert_type;
  description: string;
  threshold: string;
  check_interval_minutes: string;
  is_enabled: boolean;
};

type RuleModalProps = {
  visible: boolean;
  form: RuleForm;
  alertTypeOptions: SelectProps.Option[];
  isLoading: boolean;
  onSave: () => void;
  onClose: () => void;
  onFormChange: (form: RuleForm) => void;
  t: (key: string) => string;
};

export function RuleModal({
  visible,
  form,
  alertTypeOptions,
  isLoading,
  onSave,
  onClose,
  onFormChange,
  t,
}: RuleModalProps) {
  const ruleTypeOption =
    alertTypeOptions.find(option => option.value === form.alert_type) ?? alertTypeOptions[0];

  return (
    <Modal
      onDismiss={onClose}
      visible={visible}
      header={form.id ? t('pages.alerts_page.edit_rule') : t('pages.alerts_page.create_modal_title')}
      footer={
        <Box float="right">
          <SpaceBetween direction="horizontal" size="xs">
            <Button onClick={onClose}>{t('common.cancel')}</Button>
            <Button
              variant="primary"
              onClick={onSave}
              loading={isLoading}
              disabled={!form.name.trim()}
            >
              {form.id ? t('common.save') : t('pages.alerts_page.create_btn')}
            </Button>
          </SpaceBetween>
        </Box>
      }
    >
      <SpaceBetween size="m">
        <FormField label={t('pages.alerts_page.name_label')}>
          <Input
            value={form.name}
            onChange={(e) => onFormChange({ ...form, name: e.detail.value })}
          />
        </FormField>
        <FormField label={t('pages.alerts_page.alert_type_label')}>
          <Select
            selectedOption={ruleTypeOption}
            options={alertTypeOptions}
            onChange={(e: { detail: SelectProps.ChangeDetail }) =>
              onFormChange({ ...form, alert_type: e.detail.selectedOption.value as CreateAlertRuleAlert_type })
            }
            expandToViewport
          />
        </FormField>
        <FormField label={t('pages.alerts_page.description_label')}>
          <Input
            value={form.description}
            onChange={(e) => onFormChange({ ...form, description: e.detail.value })}
          />
        </FormField>
        <FormField label={t('pages.alerts_page.threshold_label')}>
          <Input
            type="number"
            value={form.threshold}
            onChange={(e) => onFormChange({ ...form, threshold: e.detail.value })}
          />
        </FormField>
        <FormField label={t('pages.alerts_page.interval_label')}>
          <Input
            type="number"
            value={form.check_interval_minutes}
            onChange={(e) => onFormChange({ ...form, check_interval_minutes: e.detail.value })}
          />
        </FormField>
        <Checkbox
          checked={form.is_enabled}
          onChange={(e) => onFormChange({ ...form, is_enabled: e.detail.checked })}
        >
          {t('pages.alerts_page.enabled_checkbox_label')}
        </Checkbox>
      </SpaceBetween>
    </Modal>
  );
}
