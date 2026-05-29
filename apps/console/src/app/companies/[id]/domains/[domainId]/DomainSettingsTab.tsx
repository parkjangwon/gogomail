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
  Textarea,
  Alert,
  Badge,
} from '@cloudscape-design/components';
import { useState } from 'react';
import { DomainSetting } from './domainDetailTypes';

function isJsonLike(raw: string): boolean {
  const t = raw.trim();
  if (!t) return false;
  try {
    const parsed = JSON.parse(t);
    return typeof parsed !== 'string';
  } catch {
    return false;
  }
}

function ValueCell({ value }: { value: unknown }) {
  const [expanded, setExpanded] = useState(false);
  if (typeof value === 'object' && value !== null) {
    const pretty = JSON.stringify(value, null, 2);
    const oneline = JSON.stringify(value);
    const truncated = oneline.length > 80 ? oneline.slice(0, 80) + '…' : oneline;
    return (
      <SpaceBetween size="xs">
        <Box color="text-body-secondary">
          {expanded ? (
            <pre style={{ margin: 0, fontFamily: 'monospace', fontSize: '12px', whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
              {pretty}
            </pre>
          ) : (
            <code style={{ fontFamily: 'monospace', fontSize: '12px' }}>{truncated}</code>
          )}
        </Box>
        {oneline.length > 80 && (
          <Button variant="inline-link" onClick={() => setExpanded((v) => !v)}>
            {expanded ? '접기' : '펼치기'}
          </Button>
        )}
      </SpaceBetween>
    );
  }
  return <span>{String(value ?? '—')}</span>;
}

interface Props {
  settings: DomainSetting[];
  domainName: string;
  // Add modal
  showAddSetting: boolean;
  onShowAddSetting: (v: boolean) => void;
  newSetting: { key: string; value: string };
  onNewSettingChange: (s: { key: string; value: string }) => void;
  savingSetting: boolean;
  settingError: string;
  onSetSettingError: (e: string) => void;
  onAddSetting: () => void;
  // Edit modal
  editingSetting: DomainSetting | null;
  editSettingValue: string;
  onOpenEditSetting: (s: DomainSetting) => void;
  onEditSettingValueChange: (v: string) => void;
  onCloseEditSetting: () => void;
  onSaveEditSetting: () => void;
  // Delete
  deletingSettingKey: string | null;
  onDeleteSetting: (s: DomainSetting) => void;
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
  settingError,
  onSetSettingError,
  onAddSetting,
  editingSetting,
  editSettingValue,
  onOpenEditSetting,
  onEditSettingValueChange,
  onCloseEditSetting,
  onSaveEditSetting,
  deletingSettingKey,
  onDeleteSetting,
  t,
}: Props) {
  const [confirmDeleteKey, setConfirmDeleteKey] = useState<string | null>(null);
  const pendingDelete = settings.find((s) => s.Key === confirmDeleteKey) ?? null;

  const addValueIsJson = isJsonLike(newSetting.value);
  const editValueIsJson = isJsonLike(editSettingValue);

  const valueHint = (isJson: boolean) =>
    isJson
      ? t('pages.domain_detail.value_hint_json', 'Detected as JSON — will be stored as a structured value')
      : t('pages.domain_detail.value_hint_string', 'Stored as a string. Enter valid JSON syntax for objects, arrays, numbers, or booleans.');

  return (
    <SpaceBetween size="l">
      <DataTable
        columnDefinitions={[
          {
            header: t('pages.domain_detail.setting_key'),
            cell: (s: DomainSetting) => (
              <SpaceBetween direction="horizontal" size="xs">
                <Box fontWeight="bold">{s.Key}</Box>
                {s.Locked && <Badge color="grey">{t('common.locked', 'Locked')}</Badge>}
              </SpaceBetween>
            ),
            width: '28%',
          },
          {
            header: t('pages.domain_detail.setting_value'),
            cell: (s: DomainSetting) => <ValueCell value={s.Value} />,
            width: '42%',
          },
          {
            header: t('pages.domain_detail.updated'),
            cell: (s: DomainSetting) => s.UpdatedAt ? new Date(s.UpdatedAt).toLocaleDateString() : '—',
            width: '15%',
          },
          {
            header: t('common.actions', 'Actions'),
            cell: (s: DomainSetting) => (
              <SpaceBetween direction="horizontal" size="xs">
                <Button
                  variant="inline-link"
                  disabled={s.Locked}
                  onClick={() => { onCloseEditSetting(); onSetSettingError(''); onOpenEditSetting(s); }}
                >
                  {t('common.edit', 'Edit')}
                </Button>
                <Button
                  variant="inline-link"
                  disabled={s.Locked || deletingSettingKey === s.Key}
                  loading={deletingSettingKey === s.Key}
                  onClick={() => setConfirmDeleteKey(s.Key)}
                >
                  {t('common.delete', 'Delete')}
                </Button>
              </SpaceBetween>
            ),
            width: '15%',
          },
        ]}
        items={settings}
        header={
          <Header
            variant="h2"
            counter={`(${settings.length})`}
            actions={
              <Button variant="primary" onClick={() => { onSetSettingError(''); onShowAddSetting(true); }}>
                {t('pages.domain_detail.add_setting_btn')}
              </Button>
            }
          >
            {t('pages.domain_detail.domain_settings_title')}
          </Header>
        }
        empty={
          <Box textAlign="center" padding="l">
            <Box color="text-body-secondary">{t('pages.domain_detail.no_custom_settings')}</Box>
          </Box>
        }
      />

      {/* Add Setting Modal */}
      <Modal
        visible={showAddSetting}
        onDismiss={() => { onShowAddSetting(false); onSetSettingError(''); }}
        header={`${t('pages.domain_detail.add_setting_modal_header')} — ${domainName}`}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => { onShowAddSetting(false); onSetSettingError(''); }}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={onAddSetting}
                loading={savingSetting}
                disabled={!newSetting.key.trim()}
              >
                {t('pages.domain_detail.save_setting')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField
            label={t('pages.domain_detail.key_label')}
            constraintText={t('pages.domain_detail.key_constraint')}
          >
            <Input
              value={newSetting.key}
              onChange={(e) => onNewSettingChange({ ...newSetting, key: e.detail.value })}
              placeholder="mail.max_size_mb"
              autoFocus
            />
          </FormField>
          <FormField
            label={t('pages.domain_detail.value_label')}
            constraintText={valueHint(addValueIsJson)}
          >
            <Textarea
              value={newSetting.value}
              onChange={(e) => onNewSettingChange({ ...newSetting, value: e.detail.value })}
              placeholder={'예) true  |  42  |  "hello"  |  {"key": "value"}'}
              rows={3}
            />
          </FormField>
          {settingError && <Alert type="error">{settingError}</Alert>}
        </SpaceBetween>
      </Modal>

      {/* Edit Setting Modal */}
      <Modal
        visible={!!editingSetting}
        onDismiss={() => { onCloseEditSetting(); onSetSettingError(''); }}
        header={`${t('common.edit', 'Edit')} — ${editingSetting?.Key ?? ''}`}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => { onCloseEditSetting(); onSetSettingError(''); }}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={onSaveEditSetting} loading={savingSetting}>
                {t('common.save')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.domain_detail.key_label')}>
            <Input value={editingSetting?.Key ?? ''} disabled />
          </FormField>
          <FormField
            label={t('pages.domain_detail.value_label')}
            constraintText={valueHint(editValueIsJson)}
          >
            <Textarea
              value={editSettingValue}
              onChange={(e) => onEditSettingValueChange(e.detail.value)}
              rows={6}
              autoFocus
            />
          </FormField>
          {settingError && <Alert type="error">{settingError}</Alert>}
        </SpaceBetween>
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        visible={!!confirmDeleteKey}
        onDismiss={() => setConfirmDeleteKey(null)}
        header={t('common.delete', 'Delete Setting')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setConfirmDeleteKey(null)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={() => {
                  if (pendingDelete) { onDeleteSetting(pendingDelete); setConfirmDeleteKey(null); }
                }}
              >
                {t('common.delete', 'Delete')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <Box>
          {t('pages.domain_detail.setting_key')}: <strong>{confirmDeleteKey}</strong>
          {' — '}
          {t('pages.domain_detail.delete_setting_confirm', 'This setting will be permanently removed.')}
        </Box>
      </Modal>
    </SpaceBetween>
  );
}
