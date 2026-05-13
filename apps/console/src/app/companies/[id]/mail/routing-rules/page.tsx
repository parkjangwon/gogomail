'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Toggle,
  Select,
  Box,
  Spinner,
  Flashbar,
  FlashbarProps,
  Modal,
  StatusIndicator,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface RoutingRule {
  id: string;
  name: string;
  enabled: boolean;
  priority: number;
  match_from: string;
  match_to: string;
  match_subject: string;
  action: string;
  action_value: string;
}

const emptyRule = (): RoutingRule => ({
  id: crypto.randomUUID(),
  name: '',
  enabled: true,
  priority: 10,
  match_from: '',
  match_to: '',
  match_subject: '',
  action: 'forward',
  action_value: '',
});

const ACTION_OPTIONS = [
  { labelKey: 'action_forward', value: 'forward' },
  { label: 'BCC', value: 'bcc' },
  { labelKey: 'action_reject', value: 'reject' },
  { labelKey: 'action_tag', value: 'tag' },
];

export default function RoutingRulesPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const [rules, setRules] = useState<RoutingRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);

  // Modal state
  const [modalVisible, setModalVisible] = useState(false);
  const [editingRule, setEditingRule] = useState<RoutingRule>(emptyRule());
  const [isEdit, setIsEdit] = useState(false);

  // Delete confirmation
  const [deleteModalVisible, setDeleteModalVisible] = useState(false);
  const [ruleToDelete, setRuleToDelete] = useState<RoutingRule | null>(null);

  const fetchRules = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${cid}/routing-rules`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setRules(data.rules ?? []);
      }
    } finally {
      setLoading(false);
    }
  }, [cid]);

  useEffect(() => { fetchRules(); }, [fetchRules]);

  const handleSaveAll = async () => {
    setSaving(true);
    try {
      const res = await fetch(`/api/admin/companies/${cid}/routing-rules`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ rules }),
        credentials: 'include',
      });
      if (res.ok) {
        setNotifications([{ type: 'success', content: t('pages.routing_rules_page.save_success'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-ok' }]);
      } else {
        setNotifications([{ type: 'error', content: t('pages.routing_rules_page.save_error'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-err' }]);
      }
    } finally {
      setSaving(false);
    }
  };

  const openAdd = () => {
    setEditingRule(emptyRule());
    setIsEdit(false);
    setModalVisible(true);
  };

  const openEdit = (rule: RoutingRule) => {
    setEditingRule({ ...rule });
    setIsEdit(true);
    setModalVisible(true);
  };

  const handleModalSave = () => {
    if (!editingRule.name.trim()) return;
    if (isEdit) {
      setRules(rs => rs.map(r => r.id === editingRule.id ? editingRule : r));
    } else {
      setRules(rs => [...rs, editingRule]);
    }
    setModalVisible(false);
  };

  const confirmDelete = (rule: RoutingRule) => {
    setRuleToDelete(rule);
    setDeleteModalVisible(true);
  };

  const handleDelete = () => {
    if (ruleToDelete) {
      setRules(rs => rs.filter(r => r.id !== ruleToDelete.id));
    }
    setDeleteModalVisible(false);
    setRuleToDelete(null);
  };

  const toggleEnabled = (rule: RoutingRule) => {
    setRules(rs => rs.map(r => r.id === rule.id ? { ...r, enabled: !r.enabled } : r));
  };

  const actionOptions = ACTION_OPTIONS.map(o => ({
    label: 'labelKey' in o ? t(`pages.routing_rules_page.${o.labelKey}`) : o.label,
    value: o.value,
  }));
  const selectedActionOption = actionOptions.find(o => o.value === editingRule.action) ?? actionOptions[0];

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.routing_rules_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.routing_rules_page.description')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={openAdd} iconName="add-plus">
                {t('pages.routing_rules_page.add_rule')}
              </Button>
              <Button variant="primary" onClick={handleSaveAll} loading={saving}>
                {t('pages.routing_rules_page.save_all')}
              </Button>
            </SpaceBetween>
          }
        >
          {t('pages.routing_rules_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {notifications.length > 0 && <Flashbar items={notifications} />}

        <Container>
          <DataTable
            items={[...rules].sort((a, b) => a.priority - b.priority)}
            empty={
              <Box textAlign="center" color="inherit" padding="xl">
                <SpaceBetween size="s">
                  <Box variant="strong">{t('pages.routing_rules_page.empty_state')}</Box>
                  <Button onClick={openAdd}>{t('pages.routing_rules_page.add_rule')}</Button>
                </SpaceBetween>
              </Box>
            }
            columnDefinitions={[
              {
                header: t('pages.routing_rules_page.col_priority'),
                cell: (item: RoutingRule) => item.priority,
                width: '8%',
              },
              {
                header: t('pages.routing_rules_page.col_name'),
                cell: (item: RoutingRule) => item.name,
                width: '18%',
              },
              {
                header: t('pages.routing_rules_page.col_status'),
                cell: (item: RoutingRule) => (
                  <Toggle
                    checked={item.enabled}
                    onChange={() => toggleEnabled(item)}
                  >
                    {item.enabled
                      ? <StatusIndicator type="success">{t('pages.routing_rules_page.enabled')}</StatusIndicator>
                      : <StatusIndicator type="stopped">{t('pages.routing_rules_page.disabled')}</StatusIndicator>
                    }
                  </Toggle>
                ),
                width: '14%',
              },
              {
                header: t('pages.routing_rules_page.col_match_from'),
                cell: (item: RoutingRule) => item.match_from || <Box color="text-body-secondary">—</Box>,
                width: '14%',
              },
              {
                header: t('pages.routing_rules_page.col_match_to'),
                cell: (item: RoutingRule) => item.match_to || <Box color="text-body-secondary">—</Box>,
                width: '14%',
              },
              {
                header: t('pages.routing_rules_page.col_action'),
                cell: (item: RoutingRule) => item.action,
                width: '10%',
              },
              {
                header: t('pages.routing_rules_page.col_action_value'),
                cell: (item: RoutingRule) => item.action_value || <Box color="text-body-secondary">—</Box>,
                width: '12%',
              },
              {
                header: t('pages.routing_rules_page.col_actions'),
                cell: (item: RoutingRule) => (
                  <SpaceBetween direction="horizontal" size="xs">
                    <Button variant="inline-link" onClick={() => openEdit(item)}>
                      {t('common.edit')}
                    </Button>
                    <Button variant="inline-link" onClick={() => confirmDelete(item)}>
                      {t('common.delete')}
                    </Button>
                  </SpaceBetween>
                ),
                width: '10%',
              },
            ]}
          />
        </Container>
      </SpaceBetween>

      {/* Add/Edit Modal */}
      <Modal
        visible={modalVisible}
        onDismiss={() => setModalVisible(false)}
        header={isEdit ? t('pages.routing_rules_page.modal_edit_title') : t('pages.routing_rules_page.modal_add_title')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="link" onClick={() => setModalVisible(false)}>
                {t('pages.routing_rules_page.cancel')}
              </Button>
              <Button variant="primary" onClick={handleModalSave} disabled={!editingRule.name.trim()}>
                {t('pages.routing_rules_page.save_rule')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.routing_rules_page.field_name')}>
            <Input
              value={editingRule.name}
              onChange={e => setEditingRule(r => ({ ...r, name: e.detail.value }))}
              placeholder={t('pages.routing_rules_page.block_placeholder')}
            />
          </FormField>

          <FormField label={t('pages.routing_rules_page.field_priority')} constraintText={t('pages.routing_rules_page.field_priority_hint')}>
            <Input
              type="number"
              value={String(editingRule.priority)}
              onChange={e => setEditingRule(r => ({ ...r, priority: parseInt(e.detail.value) || 0 }))}
            />
          </FormField>

          <FormField label={t('pages.routing_rules_page.field_enabled')}>
            <Toggle
              checked={editingRule.enabled}
              onChange={e => setEditingRule(r => ({ ...r, enabled: e.detail.checked }))}
            >
              {editingRule.enabled ? t('pages.routing_rules_page.enabled') : t('pages.routing_rules_page.disabled')}
            </Toggle>
          </FormField>

          <FormField label={t('pages.routing_rules_page.field_match_from')} constraintText={t('pages.routing_rules_page.field_match_from_hint')}>
            <Input
              value={editingRule.match_from}
              onChange={e => setEditingRule(r => ({ ...r, match_from: e.detail.value }))}
              placeholder="*@spam.com"
            />
          </FormField>

          <FormField label={t('pages.routing_rules_page.field_match_to')} constraintText={t('pages.routing_rules_page.field_match_to_hint')}>
            <Input
              value={editingRule.match_to}
              onChange={e => setEditingRule(r => ({ ...r, match_to: e.detail.value }))}
              placeholder="user@example.com"
            />
          </FormField>

          <FormField label={t('pages.routing_rules_page.field_match_subject')} constraintText={t('pages.routing_rules_page.field_match_subject_hint')}>
            <Input
              value={editingRule.match_subject}
              onChange={e => setEditingRule(r => ({ ...r, match_subject: e.detail.value }))}
              placeholder={t('pages.routing_rules_page.tag_placeholder')}
            />
          </FormField>

          <FormField label={t('pages.routing_rules_page.field_action')}>
            <Select
              selectedOption={selectedActionOption}
              onChange={e => setEditingRule(r => ({ ...r, action: e.detail.selectedOption.value ?? 'forward' }))}
              options={actionOptions}
            />
          </FormField>

          <FormField label={t('pages.routing_rules_page.field_action_value')} constraintText={t('pages.routing_rules_page.field_action_value_hint')}>
            <Input
              value={editingRule.action_value}
              onChange={e => setEditingRule(r => ({ ...r, action_value: e.detail.value }))}
              placeholder={editingRule.action === 'tag' ? 'spam-tag' : editingRule.action === 'reject' ? 'Rejected by policy' : 'archive@example.com'}
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        visible={deleteModalVisible}
        onDismiss={() => setDeleteModalVisible(false)}
        header={t('common.delete')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="link" onClick={() => setDeleteModalVisible(false)}>
                {t('pages.routing_rules_page.cancel')}
              </Button>
              <Button variant="primary" onClick={handleDelete}>
                {t('common.delete')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <Alert type="warning">
          {t('pages.routing_rules_page.delete_confirm')}
          {ruleToDelete && <Box variant="strong"> &quot;{ruleToDelete.name}&quot;</Box>}
        </Alert>
      </Modal>
    </ContentLayout>
  );
}
