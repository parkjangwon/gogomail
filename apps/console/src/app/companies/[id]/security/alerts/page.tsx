'use client';

import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import {
  Badge,
  Box,
  Button,
  ContentLayout,
  Header,
  SpaceBetween,
  Spinner,
  TextFilter,
  Container,
} from '@cloudscape-design/components';
import type { SelectProps } from '@cloudscape-design/components';
import {
  CreateAlertChannelChannel_type,
  CreateAlertRuleAlert_type,
} from '@gogomail/api-types';
import { DataTable } from '@/components/DataTable';
import { useI18n } from '@/app/i18n-provider';
import {
  type AlertChannel,
  type AlertChannelCreateRequest,
  type AlertChannelUpdateRequest,
  type AlertEvent,
  type AlertRule,
  type AlertRuleCreateRequest,
  useAlertChannels,
  useAlertEvents,
  useAlertRules,
  useCreateAlertChannel,
  useCreateAlertRule,
  useDeleteAlertChannel,
  useDeleteAlertRule,
  useUpdateAlertChannel,
  useUpdateAlertRule,
} from '@/hooks/useAlerts';
import { RuleModal, type RuleForm } from './RuleModal';
import { ChannelModal, type ChannelForm } from './ChannelModal';

const defaultRuleForm: RuleForm = {
  name: '',
  alert_type: CreateAlertRuleAlert_type.storage,
  description: '',
  threshold: '0.8',
  check_interval_minutes: '60',
  is_enabled: true,
};

const defaultChannelForm: ChannelForm = {
  name: '',
  channel_type: CreateAlertChannelChannel_type.email,
  recipients_text: '',
  url: '',
  auth_header: '',
  is_enabled: true,
};

function toRecipients(text: string) {
  return text
    .split(',')
    .map(value => value.trim())
    .filter(Boolean);
}

export default function AlertRulesPage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;

  const { data: rules = [], isLoading: loadingRules } = useAlertRules(companyId);
  const { data: channels = [], isLoading: loadingChannels } = useAlertChannels(companyId);
  const { data: events = [], isLoading: loadingEvents } = useAlertEvents(companyId);
  const createRule = useCreateAlertRule();
  const updateRule = useUpdateAlertRule();
  const deleteRule = useDeleteAlertRule();
  const createChannel = useCreateAlertChannel();
  const updateChannel = useUpdateAlertChannel();
  const deleteChannel = useDeleteAlertChannel();

  const alertTypeOptions: SelectProps.Option[] = useMemo(
    () => [
      { label: t('pages.alerts_page.type_storage'), value: 'storage' },
      { label: t('pages.alerts_page.type_login_failures'), value: 'login_failures' },
      { label: t('pages.alerts_page.type_api_errors'), value: 'api_errors' },
    ],
    [t]
  );

  const channelTypeOptions: SelectProps.Option[] = useMemo(
    () => [
      { label: t('pages.alerts_page.channel_email'), value: 'email' },
      { label: t('pages.alerts_page.channel_webhook'), value: 'webhook' },
      { label: t('pages.alerts_page.channel_dashboard'), value: 'dashboard' },
    ],
    [t]
  );

  const [ruleFilter, setRuleFilter] = useState('');
  const [channelFilter, setChannelFilter] = useState('');
  const [showRuleModal, setShowRuleModal] = useState(false);
  const [showChannelModal, setShowChannelModal] = useState(false);
  const [ruleForm, setRuleForm] = useState<RuleForm>(defaultRuleForm);
  const [channelForm, setChannelForm] = useState<ChannelForm>(defaultChannelForm);
  const [deletingRuleId, setDeletingRuleId] = useState<string | null>(null);
  const [deletingChannelId, setDeletingChannelId] = useState<string | null>(null);

  const filteredRules = useMemo(
    () => rules.filter(rule => rule.name.toLowerCase().includes(ruleFilter.toLowerCase())),
    [ruleFilter, rules]
  );

  const filteredChannels = useMemo(
    () => channels.filter(channel => channel.name.toLowerCase().includes(channelFilter.toLowerCase())),
    [channelFilter, channels]
  );

  const openEvents = events.filter(event => !event.resolved_at);
  const recentEvents = [...events].sort((a, b) => new Date(b.triggered_at).getTime() - new Date(a.triggered_at).getTime()).slice(0, 10);
  const loading = loadingRules || loadingChannels || loadingEvents;

  const openRuleCreate = () => {
    setRuleForm(defaultRuleForm);
    setShowRuleModal(true);
  };

  const openRuleEdit = (rule: AlertRule) => {
    setRuleForm({
      id: rule.id,
      name: rule.name,
      alert_type: rule.alert_type as CreateAlertRuleAlert_type,
      description: rule.description ?? '',
      threshold: String(rule.threshold),
      check_interval_minutes: String(rule.check_interval_minutes),
      is_enabled: rule.is_enabled,
    });
    setShowRuleModal(true);
  };

  const openChannelCreate = () => {
    setChannelForm(defaultChannelForm);
    setShowChannelModal(true);
  };

  const openChannelEdit = (channel: AlertChannel) => {
    setChannelForm({
      id: channel.id,
      name: channel.name,
      channel_type: channel.channel_type as CreateAlertChannelChannel_type,
      recipients_text: channel.config.recipients?.join(', ') ?? '',
      url: channel.config.url ?? '',
      auth_header: channel.config.auth_header ?? '',
      is_enabled: channel.is_enabled,
    });
    setShowChannelModal(true);
  };

  const saveRule = async () => {
    if (!companyId || !ruleForm.name.trim()) return;
    const parsedThreshold = Number(ruleForm.threshold);
    if (ruleForm.threshold.trim() && !Number.isFinite(parsedThreshold)) {
      return;
    }
    const payload: AlertRuleCreateRequest = {
      name: ruleForm.name.trim(),
      alert_type: ruleForm.alert_type,
      description: ruleForm.description.trim() || undefined,
      threshold: Number.isFinite(parsedThreshold) ? parsedThreshold : 0,
      check_interval_minutes: Number(ruleForm.check_interval_minutes) || 60,
      is_enabled: ruleForm.is_enabled,
    };

    try {
      if (ruleForm.id) {
        await updateRule.mutateAsync({
          companyId,
          ruleId: ruleForm.id,
          data: {
            name: payload.name,
            description: payload.description,
            threshold: payload.threshold,
            check_interval_minutes: payload.check_interval_minutes,
            is_enabled: payload.is_enabled,
          },
        });
      } else {
        await createRule.mutateAsync({
          companyId,
          data: payload,
        });
      }
      setShowRuleModal(false);
    } catch {
      // mutation error is surfaced by the query hook's error state
    }
  };

  const saveChannel = async () => {
    if (!companyId || !channelForm.name.trim()) return;

    try {
      if (channelForm.id) {
        await updateChannel.mutateAsync({
          companyId,
          channelId: channelForm.id,
          data: {
            name: channelForm.name.trim(),
            is_enabled: channelForm.is_enabled,
          } as AlertChannelUpdateRequest,
        });
      } else {
        const config: AlertChannelCreateRequest['config'] = {};
        if (channelForm.channel_type === 'email') {
          config.recipients = toRecipients(channelForm.recipients_text);
        }
        if (channelForm.channel_type === 'webhook') {
          if (channelForm.url.trim()) config.url = channelForm.url.trim();
          if (channelForm.auth_header.trim()) config.auth_header = channelForm.auth_header.trim();
        }

        await createChannel.mutateAsync({
          companyId,
          data: {
            name: channelForm.name.trim(),
            channel_type: channelForm.channel_type,
            config,
            is_enabled: channelForm.is_enabled,
          } as AlertChannelCreateRequest,
        });
      }
      setShowChannelModal(false);
    } catch {
      // mutation error is surfaced by the query hook's error state
    }
  };

  const removeRule = async (rule: AlertRule) => {
    if (!companyId || !window.confirm(t('pages.alerts_page.confirm_delete_rule').replace('{name}', rule.name))) return;
    setDeletingRuleId(rule.id);
    try {
      await deleteRule.mutateAsync({ companyId, ruleId: rule.id });
    } finally {
      setDeletingRuleId(null);
    }
  };

  const removeChannel = async (channel: AlertChannel) => {
    if (!companyId || !window.confirm(t('pages.alerts_page.confirm_delete_channel').replace('{name}', channel.name))) return;
    setDeletingChannelId(channel.id);
    try {
      await deleteChannel.mutateAsync({ companyId, channelId: channel.id });
    } finally {
      setDeletingChannelId(null);
    }
  };

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
          description={t('pages.alerts_page.page_description')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={openChannelCreate}>{t('pages.alerts_page.new_channel')}</Button>
              <Button variant="primary" onClick={openRuleCreate}>
                {t('pages.alerts_page.create_alert_btn')}
              </Button>
            </SpaceBetween>
          }
        >
          {t('pages.alerts_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Container header={<Header variant="h3">{t('pages.alerts_page.summary')}</Header>}>
          <SpaceBetween direction="horizontal" size="l">
            <Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.alerts_page.enabled_rules')}</Box>
              <Box fontSize="heading-m">{rules.filter(rule => rule.is_enabled).length}</Box>
            </Box>
            <Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.alerts_page.channels')}</Box>
              <Box fontSize="heading-m">{channels.length}</Box>
            </Box>
            <Box>
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.alerts_page.open_events')}</Box>
              <Box fontSize="heading-m">{openEvents.length}</Box>
            </Box>
          </SpaceBetween>
        </Container>

        <DataTable
          columnDefinitions={[
            {
              header: t('pages.alerts_page.name'),
              cell: (item: AlertRule) => item.name,
              width: '18%',
            },
            {
              header: t('pages.alerts_page.alert_type'),
              cell: (item: AlertRule) => <Badge color="blue">{item.alert_type}</Badge>,
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
              width: '10%',
            },
            {
              header: t('pages.alerts_page.enabled'),
              cell: (item: AlertRule) => (
                <Badge color={item.is_enabled ? 'green' : 'grey'}>
                  {item.is_enabled ? t('pages.alerts_page.enabled_label') : t('pages.alerts_page.disabled_label')}
                </Badge>
              ),
              width: '10%',
            },
            {
              header: t('pages.alerts_page.created'),
              cell: (item: AlertRule) => new Date(item.created_at).toLocaleString(),
              width: '15%',
            },
            {
              header: t('common.actions'),
              cell: (item: AlertRule) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" onClick={() => openRuleEdit(item)}>
                    {t('common.edit')}
                  </Button>
                  <Button
                    variant="inline-link"
                    onClick={() => removeRule(item)}
                    loading={deletingRuleId === item.id}
                  >
                    {t('common.delete')}
                  </Button>
                </SpaceBetween>
              ),
              width: '20%',
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
              filteringText={ruleFilter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setRuleFilter(e.detail.filteringText)}
            />
          }
          empty={<Box textAlign="center" padding="l">{t('pages.alerts_page.no_rules')}</Box>}
        />

        <DataTable
          columnDefinitions={[
            {
              header: t('pages.alerts_page.name'),
              cell: (item: AlertChannel) => item.name,
              width: '20%',
            },
            {
              header: t('pages.alerts_page.channel_type'),
              cell: (item: AlertChannel) => <Badge color="blue">{item.channel_type}</Badge>,
              width: '12%',
            },
            {
              header: t('pages.alerts_page.config'),
              cell: (item: AlertChannel) => {
                if (item.channel_type === 'email') return item.config.recipients?.join(', ') || '—';
                if (item.channel_type === 'webhook') return item.config.url || '—';
                return t('pages.alerts_page.dashboard_only');
              },
              width: '28%',
            },
            {
              header: t('pages.alerts_page.status'),
              cell: (item: AlertChannel) => (
                <Badge color={item.is_enabled ? 'green' : 'grey'}>
                  {item.is_enabled ? t('common.enabled') : t('common.disabled')}
                </Badge>
              ),
              width: '10%',
            },
            {
              header: t('common.actions'),
              cell: (item: AlertChannel) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" onClick={() => openChannelEdit(item)}>
                    {t('common.edit')}
                  </Button>
                  <Button
                    variant="inline-link"
                    onClick={() => removeChannel(item)}
                    loading={deletingChannelId === item.id}
                  >
                    {t('common.delete')}
                  </Button>
                </SpaceBetween>
              ),
              width: '20%',
            },
          ]}
          items={filteredChannels}
          header={
            <Header variant="h2" counter={`(${filteredChannels.length})`}>
              {t('pages.alerts_page.channels')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={channelFilter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setChannelFilter(e.detail.filteringText)}
            />
          }
          empty={<Box textAlign="center" padding="l">{t('pages.alerts_page.no_channels')}</Box>}
        />

        <DataTable
          columnDefinitions={[
            {
              header: t('pages.alerts_page.rule'),
              cell: (item: AlertEvent) => rules.find(r => r.id === item.alert_rule_id)?.name ?? item.alert_rule_id,
              width: '18%',
            },
            {
              header: t('pages.alerts_page.current'),
              cell: (item: AlertEvent) => item.current_value,
              width: '10%',
            },
            {
              header: t('pages.alerts_page.threshold'),
              cell: (item: AlertEvent) => item.threshold,
              width: '10%',
            },
            {
              header: t('pages.alerts_page.message'),
              cell: (item: AlertEvent) => item.message || '—',
              width: '34%',
            },
            {
              header: t('pages.alerts_page.status'),
              cell: (item: AlertEvent) => (
                <Badge color={item.resolved_at ? 'green' : 'red'}>
                  {item.resolved_at ? t('pages.alerts_page.resolved') : t('pages.alerts_page.open')}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: t('pages.alerts_page.triggered'),
              cell: (item: AlertEvent) => new Date(item.triggered_at).toLocaleString(),
              width: '16%',
            },
          ]}
          items={recentEvents}
          header={
            <Header variant="h2" counter={`(${recentEvents.length})`}>
              {t('pages.alerts_page.recent_events')}
            </Header>
          }
          empty={<Box textAlign="center" padding="l">{t('pages.alerts_page.no_recent_events')}</Box>}
        />
      </SpaceBetween>

      <RuleModal
        visible={showRuleModal}
        form={ruleForm}
        alertTypeOptions={alertTypeOptions}
        isLoading={createRule.isPending || updateRule.isPending}
        onSave={saveRule}
        onClose={() => setShowRuleModal(false)}
        onFormChange={setRuleForm}
        t={t}
      />

      <ChannelModal
        visible={showChannelModal}
        form={channelForm}
        channelTypeOptions={channelTypeOptions}
        isLoading={createChannel.isPending || updateChannel.isPending}
        onSave={saveChannel}
        onClose={() => setShowChannelModal(false)}
        onFormChange={setChannelForm}
        t={t}
      />
    </ContentLayout>
  );
}
