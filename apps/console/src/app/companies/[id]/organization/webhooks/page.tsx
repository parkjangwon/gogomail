'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Multiselect,
  MultiselectProps,
  Toggle,
  Box,
  Spinner,
  Flashbar,
  FlashbarProps,
  Table,
  Modal,
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface Webhook {
  id: string;
  name: string;
  url: string;
  secret: string;
  events: string[];
  enabled: boolean;
  created_at: string;
  last_triggered_at?: string;
}

const ALL_EVENTS = [
  'user.created', 'user.deleted', 'user.updated',
  'mail.received', 'mail.sent', 'mail.bounced',
  'domain.verified', 'domain.unverified',
  'alert.triggered',
];

const eventOptions: MultiselectProps.Option[] = ALL_EVENTS.map(e => ({ label: e, value: e }));

export default function WebhooksPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;

  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [loading, setLoading] = useState(true);
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [showModal, setShowModal] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);

  const [form, setForm] = useState({ name: '', url: '', events: [] as string[], enabled: true });

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/webhooks`);
      const data = await res.json();
      setWebhooks(data.webhooks ?? []);
    } catch {
      setFlash([{ type: 'error', content: 'Failed to load webhooks', dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setLoading(false);
    }
  }, [cid]);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    setSaving(true);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/webhooks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      });
      if (!res.ok) throw new Error(await res.text());
      setFlash([{ type: 'success', content: 'Webhook created', dismissible: true, onDismiss: () => setFlash([]) }]);
      setShowModal(false);
      setForm({ name: '', url: '', events: [], enabled: true });
      load();
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this webhook?')) return;
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/webhooks/${id}`, { method: 'DELETE' });
      if (!res.ok) throw new Error(await res.text());
      setFlash([{ type: 'success', content: 'Webhook deleted', dismissible: true, onDismiss: () => setFlash([]) }]);
      load();
    } catch {
      setFlash([{ type: 'error', content: 'Delete failed', dismissible: true, onDismiss: () => setFlash([]) }]);
    }
  };

  const handleTest = async (id: string) => {
    setTesting(id);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/webhooks/${id}/test`, { method: 'POST' });
      const data = await res.json();
      setFlash([{
        type: data.success ? 'success' : 'error',
        content: data.message ?? (data.success ? 'Test delivered' : 'Test failed'),
        dismissible: true,
        onDismiss: () => setFlash([]),
      }]);
    } catch {
      setFlash([{ type: 'error', content: 'Test request failed', dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setTesting(null);
    }
  };

  if (loading) return <Box padding="xl"><Spinner /></Box>;

  return (
    <ContentLayout header={<Header variant="h1" description="Receive real-time event notifications via HTTP POST">{t('nav.webhooks')}</Header>}>
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}
        <Container
          header={
            <Header
              variant="h2"
              actions={<Button variant="primary" onClick={() => setShowModal(true)}>Add Webhook</Button>}
            >
              Endpoints ({webhooks.length})
            </Header>
          }
        >
          <Table
            items={webhooks}
            columnDefinitions={[
              { id: 'name', header: 'Name', cell: (i) => i.name },
              { id: 'url', header: 'URL', cell: (i) => <Box variant="code">{i.url}</Box> },
              { id: 'events', header: 'Events', cell: (i) => <SpaceBetween size="xs" direction="horizontal">{i.events.map(e => <Badge key={e} color="blue">{e}</Badge>)}</SpaceBetween> },
              { id: 'status', header: 'Status', cell: (i) => <Badge color={i.enabled ? 'green' : 'grey'}>{i.enabled ? 'Active' : 'Disabled'}</Badge> },
              {
                id: 'actions', header: 'Actions',
                cell: (i) => (
                  <SpaceBetween size="xs" direction="horizontal">
                    <Button variant="inline-link" loading={testing === i.id} onClick={() => handleTest(i.id)}>Test</Button>
                    <Button variant="inline-link" onClick={() => handleDelete(i.id)}>Delete</Button>
                  </SpaceBetween>
                ),
              },
            ]}
            empty={<Box textAlign="center" color="inherit">No webhooks configured</Box>}
          />
        </Container>

        <Modal
          visible={showModal}
          onDismiss={() => setShowModal(false)}
          header="Add Webhook"
          footer={
            <Box float="right">
              <SpaceBetween size="xs" direction="horizontal">
                <Button variant="link" onClick={() => setShowModal(false)}>Cancel</Button>
                <Button variant="primary" loading={saving} onClick={handleCreate}>Create</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <FormField label={t('pages.webhooks_page.field_name')} constraintText={t('pages.webhooks_page.field_name_hint')}>
              <Input value={form.name} onChange={({ detail }) => setForm(f => ({ ...f, name: detail.value }))} />
            </FormField>
            <FormField label={t('pages.webhooks_page.field_url')} constraintText={t('pages.webhooks_page.field_url_hint')}>
              <Input value={form.url} placeholder="https://example.com/webhook" onChange={({ detail }) => setForm(f => ({ ...f, url: detail.value }))} />
            </FormField>
            <FormField label={t('pages.webhooks_page.field_events')} constraintText={t('pages.webhooks_page.field_events_hint')}>
              <Multiselect
                selectedOptions={form.events.map(e => ({ label: e, value: e }))}
                options={eventOptions}
                onChange={({ detail }) => setForm(f => ({ ...f, events: detail.selectedOptions.map(o => o.value ?? '') }))}
                placeholder={t('pages.webhooks_page.events_placeholder')}
              />
            </FormField>
            <Toggle checked={form.enabled} onChange={({ detail }) => setForm(f => ({ ...f, enabled: detail.checked }))}>
              Enable immediately
            </Toggle>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
