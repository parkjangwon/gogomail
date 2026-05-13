'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Badge,
  Modal,
  FormField,
  Input,
  Select,
  Checkbox,
} from '@cloudscape-design/components';
import type { SelectProps } from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface AlertRule {
  id: string;
  name: string;
  alert_type: string;
  description: string;
  threshold: number;
  check_interval_minutes: number;
  is_enabled: boolean;
  created_at: string;
}

export default function AlertRulesPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const [rules, setRules] = useState<AlertRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newRule, setNewRule] = useState({
    name: '',
    alert_type: 'bounce_rate',
    description: '',
    threshold: '0.05',
    check_interval_minutes: '60',
    is_enabled: true,
  });

  const [deletingId, setDeletingId] = useState<string | null>(null);

  const alertTypeOptions: SelectProps.Option[] = [
    { label: t('pages.alerts_page.type_bounce_rate'), value: 'bounce_rate' },
    { label: t('pages.alerts_page.type_delivery_failure'), value: 'delivery_failure' },
    { label: t('pages.alerts_page.type_quota_usage'), value: 'quota_usage' },
    { label: t('pages.alerts_page.type_spam_score'), value: 'spam_score' },
    { label: t('pages.alerts_page.type_queue_size'), value: 'queue_size' },
  ];

  useEffect(() => {
    fetchAlertRules();
  }, [companyId]);

  const fetchAlertRules = async () => {
    if (!companyId) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/alert-rules`, {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setRules(data.rules || []);
      }
    } catch (error) {
      console.error('Failed to fetch alert rules:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newRule.name.trim()) return;
    setCreating(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/alert-rules`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newRule.name,
          alert_type: newRule.alert_type,
          description: newRule.description,
          threshold: parseFloat(newRule.threshold) || 0,
          check_interval_minutes: parseInt(newRule.check_interval_minutes) || 60,
          is_enabled: newRule.is_enabled,
          created_by: 'admin',
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowCreateModal(false);
        setNewRule({
          name: '',
          alert_type: 'bounce_rate',
          description: '',
          threshold: '0.05',
          check_interval_minutes: '60',
          is_enabled: true,
        });
        fetchAlertRules();
      }
    } catch (error) {
      console.error('Failed to create alert rule:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (rule: AlertRule) => {
    setDeletingId(rule.id);
    try {
      await fetch(`/api/admin/alert-rules/${rule.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      fetchAlertRules();
    } catch (error) {
      console.error('Failed to delete alert rule:', error);
    } finally {
      setDeletingId(null);
    }
  };

  const selectedAlertType = alertTypeOptions.find(o => o.value === newRule.alert_type) ?? alertTypeOptions[0];

  const filteredRules = rules.filter(r =>
    r.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.alerts_page.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.alerts_page.description')}
          actions={
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              {t('pages.alerts_page.create_alert_btn')}
            </Button>
          }
        >
          {t('pages.alerts_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.alerts_page.name'),
              cell: (item: AlertRule) => item.name,
              width: '20%',
            },
            {
              header: t('pages.alerts_page.alert_type'),
              cell: (item: AlertRule) => (
                <Badge color="blue">{item.alert_type}</Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.alerts_page.threshold'),
              cell: (item: AlertRule) => item.threshold,
              width: '10%',
            },
            {
              header: t('pages.alerts_page.interval'),
              cell: (item: AlertRule) => `${item.check_interval_minutes} min`,
              width: '12%',
            },
            {
              header: t('pages.alerts_page.enabled'),
              cell: (item: AlertRule) => (
                <Badge color={item.is_enabled ? 'green' : 'grey'}>
                  {item.is_enabled ? t('pages.alerts_page.enabled_label') : t('pages.alerts_page.disabled_label')}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: t('pages.alerts_page.created'),
              cell: (item: AlertRule) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
            {
              header: t('pages.alerts_page.actions'),
              cell: (item: AlertRule) => (
                <Button
                  variant="inline-link"
                  onClick={() => handleDelete(item)}
                  loading={deletingId === item.id}
                >
                  {t('common.delete')}
                </Button>
              ),
              width: '16%',
            },
          ]}
          items={filteredRules}
          header={
            <Header variant="h2" counter={`(${filteredRules.length})`}>
              {t('pages.alerts_page.rules')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
          empty={
            <Box textAlign="center" padding="l">
              {t('pages.alerts_page.no_rules')}
            </Box>
          }
        />
      </SpaceBetween>

      <Modal
        onDismiss={() => setShowCreateModal(false)}
        visible={showCreateModal}
        size="medium"
        header={t('pages.alerts_page.create_modal_title')}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowCreateModal(false)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={handleCreate}
                loading={creating}
                disabled={!newRule.name.trim()}
              >
                {t('pages.alerts_page.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label={t('pages.alerts_page.name_label')}>
            <Input
              value={newRule.name}
              onChange={(e) => setNewRule({ ...newRule, name: e.detail.value })}
              placeholder={t('pages.alerts_page.name_placeholder')}
            />
          </FormField>
          <FormField label={t('pages.alerts_page.alert_type_label')}>
            <Select
              selectedOption={selectedAlertType}
              options={alertTypeOptions}
              onChange={(e: { detail: SelectProps.ChangeDetail }) =>
                setNewRule({ ...newRule, alert_type: e.detail.selectedOption.value ?? 'bounce_rate' })
              }
              expandToViewport
            />
          </FormField>
          <FormField label={t('pages.alerts_page.description_label')}>
            <Input
              value={newRule.description}
              onChange={(e) => setNewRule({ ...newRule, description: e.detail.value })}
              placeholder={t('pages.alerts_page.description_placeholder')}
            />
          </FormField>
          <FormField label={t('pages.alerts_page.threshold_label')}>
            <Input
              type="number"
              value={newRule.threshold}
              onChange={(e) => setNewRule({ ...newRule, threshold: e.detail.value })}
              placeholder="0.05"
            />
          </FormField>
          <FormField label={t('pages.alerts_page.interval_label')}>
            <Input
              type="number"
              value={newRule.check_interval_minutes}
              onChange={(e) => setNewRule({ ...newRule, check_interval_minutes: e.detail.value })}
              placeholder="60"
            />
          </FormField>
          <FormField label={t('pages.alerts_page.enabled_label')}>
            <Checkbox
              checked={newRule.is_enabled}
              onChange={(e) => setNewRule({ ...newRule, is_enabled: e.detail.checked })}
            >
              {t('pages.alerts_page.enabled_checkbox_label')}
            </Checkbox>
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
