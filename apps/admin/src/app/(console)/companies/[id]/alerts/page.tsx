'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import {
  Button,
  Container,
  Header,
  Tabs,
  TabsProps,
  Modal,
  Box,
  FormField,
  Input,
  Select,
  Checkbox,
  Textarea,
  Alert,
  SpaceBetween,
  Spinner,
  Table,
} from '@cloudscape-design/components';
import { useAlertRules, useAlertChannels, useAlertEvents, useCreateAlertRule, useCreateAlertChannel, useDeleteAlertRule } from '@/hooks/useAlerts';
import { AlertRule, AlertChannel, AlertEvent } from '@/hooks/useAlerts';

export default function AlertsPage() {
  const params = useParams();
  const companyId = params.id as string;
  const [activeTab, setActiveTab] = useState<string>('rules');
  const [showRuleModal, setShowRuleModal] = useState(false);
  const [showChannelModal, setShowChannelModal] = useState(false);

  // Alert Rules
  const rulesQuery = useAlertRules(companyId);
  const createRuleMutation = useCreateAlertRule();
  const deleteRuleMutation = useDeleteAlertRule();

  // Alert Channels
  const channelsQuery = useAlertChannels(companyId);
  const createChannelMutation = useCreateAlertChannel();

  // Alert Events
  const eventsQuery = useAlertEvents(companyId);

  // Form state for rules
  const [ruleForm, setRuleForm] = useState({
    alert_type: 'storage' as const,
    name: '',
    description: '',
    threshold: 80,
    check_interval_minutes: 5,
    is_enabled: true,
    created_by: 'admin',
  });

  // Form state for channels
  const [channelForm, setChannelForm] = useState({
    channel_type: 'email' as 'email' | 'webhook' | 'dashboard',
    name: '',
    config: { recipients: [] as string[], url: '', auth_header: '' },
    is_enabled: true,
    created_by: 'admin',
  });

  const handleCreateRule = async () => {
    try {
      await createRuleMutation.mutateAsync({
        companyId,
        data: ruleForm,
      });
      setShowRuleModal(false);
      setRuleForm({
        alert_type: 'storage',
        name: '',
        description: '',
        threshold: 80,
        check_interval_minutes: 5,
        is_enabled: true,
        created_by: 'admin',
      });
    } catch (error) {
      console.error('Failed to create rule:', error);
    }
  };

  const handleCreateChannel = async () => {
    try {
      await createChannelMutation.mutateAsync({
        companyId,
        data: channelForm,
      });
      setShowChannelModal(false);
      setChannelForm({
        channel_type: 'email',
        name: '',
        config: { recipients: [], url: '', auth_header: '' },
        is_enabled: true,
        created_by: 'admin',
      });
    } catch (error) {
      console.error('Failed to create channel:', error);
    }
  };

  const handleDeleteRule = async (ruleId: string) => {
    if (confirm('Are you sure you want to delete this alert rule?')) {
      try {
        await deleteRuleMutation.mutateAsync({ ruleId, companyId });
      } catch (error) {
        console.error('Failed to delete rule:', error);
      }
    }
  };

  const ruleColumns = [
    { header: 'Name', cell: (item: AlertRule) => item.name },
    { header: 'Type', cell: (item: AlertRule) => item.alert_type },
    { header: 'Threshold', cell: (item: AlertRule) => item.threshold },
    { header: 'Interval (min)', cell: (item: AlertRule) => item.check_interval_minutes },
    {
      header: 'Status',
      cell: (item: AlertRule) => (item.is_enabled ? 'Enabled' : 'Disabled'),
    },
    {
      header: 'Actions',
      cell: (item: AlertRule) => (
        <Button
          onClick={() => handleDeleteRule(item.id)}
          loading={deleteRuleMutation.isPending}
        >
          Delete
        </Button>
      ),
    },
  ];

  const channelColumns = [
    { header: 'Name', cell: (item: AlertChannel) => item.name },
    { header: 'Type', cell: (item: AlertChannel) => item.channel_type },
    {
      header: 'Status',
      cell: (item: AlertChannel) => (item.is_enabled ? 'Enabled' : 'Disabled'),
    },
  ];

  const eventColumns = [
    { header: 'Rule ID', cell: (item: AlertEvent) => item.alert_rule_id },
    { header: 'Current Value', cell: (item: AlertEvent) => item.current_value },
    { header: 'Threshold', cell: (item: AlertEvent) => item.threshold },
    { header: 'Triggered', cell: (item: AlertEvent) => new Date(item.triggered_at).toLocaleString() },
    {
      header: 'Status',
      cell: (item: AlertEvent) => (item.resolved_at ? 'Resolved' : 'Triggered'),
    },
  ];

  const tabs: TabsProps.Tab[] = [
    {
      label: 'Alert Rules',
      id: 'rules',
      content: (
        <SpaceBetween size="m">
          <Box float="right">
            <Button
              variant="primary"
              onClick={() => setShowRuleModal(true)}
            >
              Create Alert Rule
            </Button>
          </Box>
          {rulesQuery.isPending ? (
            <Spinner />
          ) : rulesQuery.data && rulesQuery.data.length > 0 ? (
            <Table
              columnDefinitions={ruleColumns}
              items={rulesQuery.data}
              header={<Header>Alert Rules</Header>}
            />
          ) : (
            <Alert>No alert rules configured</Alert>
          )}
        </SpaceBetween>
      ),
    },
    {
      label: 'Alert Channels',
      id: 'channels',
      content: (
        <SpaceBetween size="m">
          <Box float="right">
            <Button
              variant="primary"
              onClick={() => setShowChannelModal(true)}
            >
              Create Channel
            </Button>
          </Box>
          {channelsQuery.isPending ? (
            <Spinner />
          ) : channelsQuery.data && channelsQuery.data.length > 0 ? (
            <Table
              columnDefinitions={channelColumns}
              items={channelsQuery.data}
              header={<Header>Alert Channels</Header>}
            />
          ) : (
            <Alert>No alert channels configured</Alert>
          )}
        </SpaceBetween>
      ),
    },
    {
      label: 'Alert Events',
      id: 'events',
      content: (
        <SpaceBetween size="m">
          {eventsQuery.isPending ? (
            <Spinner />
          ) : eventsQuery.data && eventsQuery.data.length > 0 ? (
            <Table
              columnDefinitions={eventColumns}
              items={eventsQuery.data}
              header={<Header>Alert Events</Header>}
            />
          ) : (
            <Alert>No alert events</Alert>
          )}
        </SpaceBetween>
      ),
    },
  ];

  return (
    <Container header={<Header>Alert Configuration</Header>}>
      <Tabs activeTabId={activeTab} onChange={(e) => setActiveTab(e.detail.activeTabId)} tabs={tabs} />

      {/* Create Alert Rule Modal */}
      <Modal
        onDismiss={() => setShowRuleModal(false)}
        visible={showRuleModal}
        header="Create Alert Rule"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="link" onClick={() => setShowRuleModal(false)}>
                Cancel
              </Button>
              <Button
                variant="primary"
                onClick={handleCreateRule}
                loading={createRuleMutation.isPending}
              >
                Create
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label="Alert Type">
            <Select
              selectedOption={{
                label: ruleForm.alert_type,
                value: ruleForm.alert_type,
              }}
              options={[
                { label: 'Storage', value: 'storage' },
                { label: 'Login Failures', value: 'login_failures' },
                { label: 'API Errors', value: 'api_errors' },
              ]}
              onChange={(e) =>
                setRuleForm({
                  ...ruleForm,
                  alert_type: e.detail.selectedOption.value as any,
                })
              }
            />
          </FormField>
          <FormField label="Rule Name">
            <Input
              value={ruleForm.name}
              onChange={(e) =>
                setRuleForm({ ...ruleForm, name: e.detail.value })
              }
            />
          </FormField>
          <FormField label="Description">
            <Textarea
              value={ruleForm.description}
              onChange={(e) =>
                setRuleForm({ ...ruleForm, description: e.detail.value })
              }
            />
          </FormField>
          <FormField label="Threshold">
            <Input
              type="number"
              value={String(ruleForm.threshold)}
              onChange={(e) =>
                setRuleForm({
                  ...ruleForm,
                  threshold: parseFloat(e.detail.value),
                })
              }
            />
          </FormField>
          <FormField label="Check Interval (minutes)">
            <Input
              type="number"
              value={String(ruleForm.check_interval_minutes)}
              onChange={(e) =>
                setRuleForm({
                  ...ruleForm,
                  check_interval_minutes: parseInt(e.detail.value),
                })
              }
            />
          </FormField>
          <Checkbox
            checked={ruleForm.is_enabled}
            onChange={(e) =>
              setRuleForm({ ...ruleForm, is_enabled: e.detail.checked })
            }
          >
            Enabled
          </Checkbox>
        </SpaceBetween>
      </Modal>

      {/* Create Alert Channel Modal */}
      <Modal
        onDismiss={() => setShowChannelModal(false)}
        visible={showChannelModal}
        header="Create Alert Channel"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="link" onClick={() => setShowChannelModal(false)}>
                Cancel
              </Button>
              <Button
                variant="primary"
                onClick={handleCreateChannel}
                loading={createChannelMutation.isPending}
              >
                Create
              </Button>
            </SpaceBetween>
          </Box>
        }
      >
        <SpaceBetween size="m">
          <FormField label="Channel Type">
            <Select
              selectedOption={{
                label: channelForm.channel_type,
                value: channelForm.channel_type,
              }}
              options={[
                { label: 'Email', value: 'email' },
                { label: 'Webhook', value: 'webhook' },
                { label: 'Dashboard', value: 'dashboard' },
              ]}
              onChange={(e) =>
                setChannelForm({
                  ...channelForm,
                  channel_type: e.detail.selectedOption.value as any,
                })
              }
            />
          </FormField>
          <FormField label="Channel Name">
            <Input
              value={channelForm.name}
              onChange={(e) =>
                setChannelForm({ ...channelForm, name: e.detail.value })
              }
            />
          </FormField>
          {channelForm.channel_type === 'email' && (
            <FormField label="Recipients (comma-separated)">
              <Textarea
                value={channelForm.config.recipients?.join(', ') || ''}
                onChange={(e) =>
                  setChannelForm({
                    ...channelForm,
                    config: {
                      ...channelForm.config,
                      recipients: e.detail.value
                        .split(',')
                        .map((r) => r.trim()),
                    },
                  })
                }
              />
            </FormField>
          )}
          {channelForm.channel_type === 'webhook' && (
            <FormField label="Webhook URL">
              <Input
                value={(channelForm.config as any).url || ''}
                onChange={(e) =>
                  setChannelForm({
                    ...channelForm,
                    config: { ...channelForm.config, url: e.detail.value } as any,
                  })
                }
              />
            </FormField>
          )}
          <Checkbox
            checked={channelForm.is_enabled}
            onChange={(e) =>
              setChannelForm({
                ...channelForm,
                is_enabled: e.detail.checked,
              })
            }
          >
            Enabled
          </Checkbox>
        </SpaceBetween>
      </Modal>
    </Container>
  );
}
