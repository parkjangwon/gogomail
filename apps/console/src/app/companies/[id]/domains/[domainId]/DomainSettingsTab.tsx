'use client';
import { DataTable } from '@/components/DataTable';
import {
  Header,
  Box,
  SpaceBetween,
  Button,
  Modal,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { DomainSetting } from './domainDetailTypes';

interface Props {
  settings: DomainSetting[];
  domainName: string;
  showAddSetting: boolean;
  onShowAddSetting: (v: boolean) => void;
  newSetting: { key: string; value: string };
  onNewSettingChange: (s: { key: string; value: string }) => void;
  savingSetting: boolean;
  onAddSetting: () => void;
  t: (key: string, fallback?: string) => string;
}

export function DomainSettingsTab({
  settings,
  domainName,
  showAddSetting,
  onShowAddSetting,
  newSetting,
  onNewSettingChange,
  savingSetting,
  onAddSetting,
  t,
}: Props) {
  return (
    <SpaceBetween size="l">
      <DataTable
        columnDefinitions={[
          { header: t('pages.domain_detail.setting_key'), cell: (s: DomainSetting) => <Box fontWeight="bold">{s.Key}</Box>, width: '35%' },
          { header: t('pages.domain_detail.setting_value'), cell: (s: DomainSetting) => typeof s.Value === 'object' ? JSON.stringify(s.Value) : String(s.Value ?? ''), width: '45%' },
          { header: t('pages.domain_detail.updated'), cell: (s: DomainSetting) => s.UpdatedAt ? new Date(s.UpdatedAt).toLocaleDateString() : '—', width: '20%' },
        ]}
        items={settings}
        header={
          <Header variant="h2" counter={`(${settings.length})`} actions={<Button variant="primary" onClick={() => onShowAddSetting(true)}>{t('pages.domain_detail.add_setting_btn')}</Button>}>
            {t('pages.domain_detail.domain_settings_title')}
          </Header>
        }
        empty={<Box textAlign="center" padding="l"><Box color="text-body-secondary">{t('pages.domain_detail.no_custom_settings')}</Box></Box>}
      />

      <Modal
        visible={showAddSetting}
        onDismiss={() => onShowAddSetting(false)}
        header={`${t('pages.domain_detail.add_setting_modal_header')} — ${domainName}`}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => onShowAddSetting(false)}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={onAddSetting} loading={savingSetting} disabled={!newSetting.key.trim()}>
                {t('pages.domain_detail.save_setting')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.domain_detail.key_label')} constraintText={t('pages.domain_detail.key_constraint')}>
            <Input value={newSetting.key} onChange={(e) => onNewSettingChange({ ...newSetting, key: e.detail.value })} placeholder="setting_key" autoFocus />
          </FormField>
          <FormField label={t('pages.domain_detail.value_label')}>
            <Input value={newSetting.value} onChange={(e) => onNewSettingChange({ ...newSetting, value: e.detail.value })} placeholder="value" />
          </FormField>
        </SpaceBetween>
      </Modal>
    </SpaceBetween>
  );
}
