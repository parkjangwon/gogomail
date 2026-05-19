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
  Multiselect,
  MultiselectProps,
  Toggle,
  Box,
  Spinner,
  Flashbar,
  FlashbarProps,
  Modal,
  Badge,
} from '@cloudscape-design/components';
import { useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import { useCompanyWebhooks, useCreateCompanyWebhook, useDeleteCompanyWebhook, useTestCompanyWebhook, type CompanyWebhookInput } from '@/hooks';

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
  const webhooksQuery = useCompanyWebhooks(cid);
  const createWebhook = useCreateCompanyWebhook();
  const deleteWebhook = useDeleteCompanyWebhook();
  const testWebhook = useTestCompanyWebhook();
  const webhooks = webhooksQuery.data ?? [];
  const loading = webhooksQuery.isLoading;
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [showModal, setShowModal] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [form, setForm] = useState<CompanyWebhookInput>({ name: '', url: '', events: [], enabled: true });

  const handleCreate = async () => {
    setSaving(true);
    try {
      await createWebhook.mutateAsync({ companyId: cid!, data: form });
      setFlash([{ type: 'success', content: t('webhooks_page.created'), dismissible: true, onDismiss: () => setFlash([]) }]);
      setShowModal(false);
      setForm({ name: '', url: '', events: [], enabled: true });
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: e instanceof Error ? e.message : 'An unexpected error occurred.', dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t('webhooks_page.confirm_delete'))) return;
    try {
      await deleteWebhook.mutateAsync({ companyId: cid!, webhookId: id });
      setFlash([{ type: 'success', content: t('webhooks_page.deleted'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } catch {
      setFlash([{ type: 'error', content: t('webhooks_page.delete_failed'), dismissible: true, onDismiss: () => setFlash([]) }]);
    }
  };

  const handleTest = async (id: string) => {
    setTesting(id);
    try {
      const res = await testWebhook.mutateAsync({ companyId: cid!, webhookId: id });
      setFlash([{
        type: res.status_code && res.status_code < 300 ? 'success' : 'error',
        content: res.status_code && res.status_code < 300 ? t('webhooks_page.test_delivered') : t('webhooks_page.test_failed'),
        dismissible: true,
        onDismiss: () => setFlash([]),
      }]);
    } catch {
      setFlash([{ type: 'error', content: t('webhooks_page.test_request_failed'), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setTesting(null);
    }
  };

  if (loading) return <Box padding="xl"><Spinner /></Box>;

  return (
    <ContentLayout header={<Header variant="h1" description={t('webhooks_page.description')}>{t('nav.webhooks')}</Header>}>
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}
        <Container
          header={
            <Header
              variant="h2"
              actions={<Button variant="primary" onClick={() => setShowModal(true)}>{t('webhooks_page.add_webhook')}</Button>}
            >
              {t('webhooks_page.endpoints')} ({webhooks.length})
            </Header>
          }
        >
          <DataTable
            items={webhooks}
            columnDefinitions={[
              { id: 'name', header: t('webhooks_page.name'), cell: (i) => i.name ?? '—' },
              { id: 'url', header: 'URL', cell: (i) => <Box variant="code">{i.url ?? '—'}</Box> },
              { id: 'events', header: t('webhooks_page.events'), cell: (i) => <SpaceBetween size="xs" direction="horizontal">{(i.events ?? []).map(e => <Badge key={e} color="blue">{e}</Badge>)}</SpaceBetween> },
              { id: 'status', header: t('webhooks_page.status'), cell: (i) => <Badge color={i.enabled ? 'green' : 'grey'}>{(i.enabled ?? false) ? t('status.active') : t('status.disabled')}</Badge> },
              {
                id: 'actions', header: t('common.actions'),
                cell: (i) => (
                  <SpaceBetween size="xs" direction="horizontal">
                    <Button variant="inline-link" loading={testing === i.id} onClick={() => handleTest(i.id ?? '')}>{t('webhooks_page.test')}</Button>
                    <Button variant="inline-link" onClick={() => handleDelete(i.id ?? '')}>{t('common.delete')}</Button>
                  </SpaceBetween>
                ),
              },
            ]}
            empty={<Box textAlign="center" color="inherit">{t('webhooks_page.empty')}</Box>}
          />
        </Container>

        <Modal
          visible={showModal}
          onDismiss={() => setShowModal(false)}
          header={t('webhooks_page.add_webhook')}
          footer={
            <Box float="right">
              <SpaceBetween size="xs" direction="horizontal">
                <Button variant="link" onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
                <Button variant="primary" loading={saving} onClick={handleCreate}>{t('common.create')}</Button>
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
            <Toggle checked={form.enabled ?? true} onChange={({ detail }) => setForm(f => ({ ...f, enabled: detail.checked }))}>
              {t('webhooks_page.enable_immediately')}
            </Toggle>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </ContentLayout>
  );
}
