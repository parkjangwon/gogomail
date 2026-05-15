'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  Table,
  Box,
  FormField,
  Input,
  Select,
  Toggle,
  Modal,
  ModalProps,
  Form,
} from '@cloudscape-design/components';
import { useParams } from 'next/navigation';
import { useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useAlertConfigs, useCreateAlertConfig, useUpdateAlertConfig, useDeleteAlertConfig, type AlertConfig } from '@/hooks/useAlertConfigs';

type AlertTypeOption = { label: string; value: 'storage' | 'login_failures' | 'api_errors' };
type ChannelTypeOption = { label: string; value: 'email' | 'webhook' | 'dashboard' };

export default function AlertsPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params.id as string;

  const { data: configs = [], isLoading } = useAlertConfigs(companyId);
  const createMutation = useCreateAlertConfig();
  const updateMutation = useUpdateAlertConfig();
  const deleteMutation = useDeleteAlertConfig();

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [selectedConfig, setSelectedConfig] = useState<AlertConfig | null>(null);
  const [formData, setFormData] = useState<Partial<AlertConfig>>({});

  const alertTypeOptions: AlertTypeOption[] = [
    { label: t('alerts.type.storage'), value: 'storage' },
    { label: t('alerts.type.login_failures'), value: 'login_failures' },
    { label: t('alerts.type.api_errors'), value: 'api_errors' },
  ];

  const channelTypeOptions: ChannelTypeOption[] = [
    { label: t('alerts.channel.email'), value: 'email' },
    { label: t('alerts.channel.webhook'), value: 'webhook' },
    { label: t('alerts.channel.dashboard'), value: 'dashboard' },
  ];

  const handleCreateOpen = () => {
    setFormData({
      alert_type: 'storage',
      check_interval_minutes: 5,
      is_enabled: true,
      channels: [],
    });
    setShowCreateModal(true);
  };

  const handleCreateSubmit = () => {
    if (!formData.alert_type || !formData.threshold || !formData.name) return;

    createMutation.mutate(formData as any, {
      onSuccess: () => {
        setShowCreateModal(false);
        setFormData({});
      },
    });
  };

  const handleDeleteConfig = (id: string) => {
    if (confirm(t('common.confirm_delete'))) {
      deleteMutation.mutate(id);
    }
  };

  const columns = [
    {
      id: 'name',
      header: t('alerts.column.name'),
      cell: (item: AlertConfig) => item.name,
    },
    {
      id: 'alert_type',
      header: t('alerts.column.type'),
      cell: (item: AlertConfig) => {
        const option = alertTypeOptions.find(o => o.value === item.alert_type);
        return option?.label || item.alert_type;
      },
    },
    {
      id: 'threshold',
      header: t('alerts.column.threshold'),
      cell: (item: AlertConfig) => item.threshold.toString(),
    },
    {
      id: 'is_enabled',
      header: t('alerts.column.enabled'),
      cell: (item: AlertConfig) => item.is_enabled ? t('common.yes') : t('common.no'),
    },
    {
      id: 'channels',
      header: t('alerts.column.channels'),
      cell: (item: AlertConfig) => item.channels.length.toString(),
    },
    {
      id: 'actions',
      header: t('common.actions'),
      cell: (item: AlertConfig) => (
        <SpaceBetween direction="horizontal" size="xs">
          <Button
            variant="inline-link"
            onClick={() => setSelectedConfig(item)}
          >
            {t('common.edit')}
          </Button>
          <Button
            variant="inline-link"
            onClick={() => handleDeleteConfig(item.id)}
          >
            {t('common.delete')}
          </Button>
        </SpaceBetween>
      ),
    },
  ];

  return (
    <ContentLayout header={<Header>{t('alerts.title')}</Header>}>
      <SpaceBetween size="l">
        <Container
          header={
            <Header
              actions={
                <Button variant="primary" onClick={handleCreateOpen}>
                  {t('alerts.create_button')}
                </Button>
              }
            >
              {t('alerts.configs_header')}
            </Header>
          }
        >
          <Table
            columnDefinitions={columns}
            items={configs}
            loading={isLoading}
            empty={
              <Box textAlign="center" color="inherit">
                <b>{t('alerts.no_configs')}</b>
              </Box>
            }
          />
        </Container>

        {/* Create/Edit Modal */}
        <Modal
          onDismiss={() => {
            setShowCreateModal(false);
            setSelectedConfig(null);
          }}
          visible={showCreateModal || !!selectedConfig}
          closeAriaLabel={t('common.close')}
          header={selectedConfig ? t('alerts.edit_title') : t('alerts.create_title')}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button
                  variant="link"
                  onClick={() => {
                    setShowCreateModal(false);
                    setSelectedConfig(null);
                  }}
                >
                  {t('common.cancel')}
                </Button>
                <Button
                  variant="primary"
                  onClick={handleCreateSubmit}
                  loading={createMutation.isPending || updateMutation.isPending}
                >
                  {t('common.save')}
                </Button>
              </SpaceBetween>
            </Box>
          }
        >
          <Form>
            <SpaceBetween size="l">
              <FormField label={t('alerts.field.name')}>
                <Input
                  value={formData.name || ''}
                  onChange={e => setFormData({ ...formData, name: e.detail.value })}
                  placeholder={t('alerts.field.name_placeholder')}
                />
              </FormField>

              <FormField label={t('alerts.field.type')}>
                <Select
                  selectedOption={
                    formData.alert_type
                      ? alertTypeOptions.find(o => o.value === formData.alert_type)
                      : null
                  }
                  onChange={e => setFormData({ ...formData, alert_type: e.detail.selectedOption.value })}
                  options={alertTypeOptions}
                />
              </FormField>

              <FormField label={t('alerts.field.threshold')}>
                <Input
                  type="number"
                  value={(formData.threshold || 0).toString()}
                  onChange={e => setFormData({ ...formData, threshold: parseFloat(e.detail.value) })}
                  placeholder="80.0"
                />
              </FormField>

              <FormField label={t('alerts.field.description')}>
                <Input
                  value={formData.description || ''}
                  onChange={e => setFormData({ ...formData, description: e.detail.value })}
                  placeholder={t('alerts.field.description_placeholder')}
                />
              </FormField>

              <FormField label={t('alerts.field.check_interval')}>
                <Input
                  type="number"
                  value={(formData.check_interval_minutes || 5).toString()}
                  onChange={e => setFormData({ ...formData, check_interval_minutes: parseInt(e.detail.value) })}
                />
              </FormField>

              <FormField label={t('alerts.field.enabled')}>
                <Toggle
                  checked={formData.is_enabled !== false}
                  onChange={e => setFormData({ ...formData, is_enabled: e.detail.checked })}
                />
              </FormField>
            </SpaceBetween>
          </Form>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
