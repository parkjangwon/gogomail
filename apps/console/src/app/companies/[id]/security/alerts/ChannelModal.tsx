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
  Textarea,
} from '@cloudscape-design/components';
import type { SelectProps } from '@cloudscape-design/components';
import { CreateAlertChannelChannel_type } from '@gogomail/api-types';

export type ChannelForm = {
  id?: string;
  name: string;
  channel_type: CreateAlertChannelChannel_type;
  recipients_text: string;
  url: string;
  auth_header: string;
  is_enabled: boolean;
};

type ChannelModalProps = {
  visible: boolean;
  form: ChannelForm;
  channelTypeOptions: SelectProps.Option[];
  isLoading: boolean;
  onSave: () => void;
  onClose: () => void;
  onFormChange: (form: ChannelForm) => void;
  t: (key: string) => string;
};

export function ChannelModal({
  visible,
  form,
  channelTypeOptions,
  isLoading,
  onSave,
  onClose,
  onFormChange,
  t,
}: ChannelModalProps) {
  const channelTypeOption =
    channelTypeOptions.find(option => option.value === form.channel_type) ?? channelTypeOptions[0];

  return (
    <Modal
      onDismiss={onClose}
      visible={visible}
      header={form.id ? t('pages.alerts_page.edit_channel') : t('pages.alerts_page.create_channel')}
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
              {form.id ? t('common.save') : t('common.create')}
            </Button>
          </SpaceBetween>
        </Box>
      }
    >
      <SpaceBetween size="m">
        <FormField label={t('pages.alerts_page.name')}>
          <Input
            value={form.name}
            onChange={(e) => onFormChange({ ...form, name: e.detail.value })}
          />
        </FormField>
        <FormField label={t('pages.alerts_page.channel_type')}>
          <Select
            selectedOption={channelTypeOption}
            options={channelTypeOptions}
            onChange={(e: { detail: SelectProps.ChangeDetail }) =>
              onFormChange({
                ...form,
                channel_type: e.detail.selectedOption.value as CreateAlertChannelChannel_type,
              })
            }
            expandToViewport
            disabled={!!form.id}
          />
        </FormField>
        {form.id ? (
          <Box color="text-body-secondary">{t('pages.alerts_page.config_readonly')}</Box>
        ) : form.channel_type === 'email' ? (
          <FormField label={t('pages.alerts_page.recipients')}>
            <Textarea
              value={form.recipients_text}
              onChange={({ detail }) => onFormChange({ ...form, recipients_text: detail.value })}
              rows={3}
            />
          </FormField>
        ) : form.channel_type === 'webhook' ? (
          <>
            <FormField label={t('pages.alerts_page.webhook_url')}>
              <Input
                value={form.url}
                onChange={(e) => onFormChange({ ...form, url: e.detail.value })}
                placeholder="https://example.com/webhook"
              />
            </FormField>
            <FormField label={t('pages.alerts_page.auth_header')}>
              <Input
                value={form.auth_header}
                onChange={(e) => onFormChange({ ...form, auth_header: e.detail.value })}
                placeholder="Authorization: Bearer ..."
              />
            </FormField>
          </>
        ) : (
          <Box color="text-body-secondary">{t('pages.alerts_page.dashboard_no_config')}</Box>
        )}
        <Checkbox
          checked={form.is_enabled}
          onChange={(e) => onFormChange({ ...form, is_enabled: e.detail.checked })}
        >
          {t('common.enabled')}
        </Checkbox>
      </SpaceBetween>
    </Modal>
  );
}
