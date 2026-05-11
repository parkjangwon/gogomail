'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Textarea,
  Select,
  SelectProps,
  Box,
  Spinner,
  Flashbar,
  FlashbarProps,
  Table,
  Modal,
  Badge,
  Tabs,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface NotifTemplate {
  id: string;
  name: string;
  subject: string;
  body_html: string;
  body_text: string;
  locale: string;
  variables: string[];
}

const LOCALE_OPTIONS: SelectProps.Option[] = [
  { label: 'English', value: 'en' },
  { label: '한국어', value: 'ko' },
  { label: '日本語', value: 'ja' },
  { label: '中文 (简体)', value: 'zh-CN' },
];

export default function NotifTemplatesPage() {
  const { t } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id;

  const [templates, setTemplates] = useState<NotifTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [selected, setSelected] = useState<NotifTemplate | null>(null);
  const [saving, setSaving] = useState(false);
  const [editForm, setEditForm] = useState<Partial<NotifTemplate>>({});

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/notification-templates`);
      const data = await res.json();
      setTemplates(data.templates ?? []);
    } catch {
      setFlash([{ type: 'error', content: 'Failed to load templates', dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setLoading(false);
    }
  }, [cid]);

  useEffect(() => { load(); }, [load]);

  const openEdit = (tmpl: NotifTemplate) => {
    setSelected(tmpl);
    setEditForm({ ...tmpl });
  };

  const handleSave = async () => {
    if (!selected) return;
    setSaving(true);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/notification-templates/${selected.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(editForm),
      });
      if (!res.ok) throw new Error(await res.text());
      setFlash([{ type: 'success', content: 'Template saved', dismissible: true, onDismiss: () => setFlash([]) }]);
      setSelected(null);
      load();
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <Box padding="xl"><Spinner /></Box>;

  return (
    <ContentLayout header={<Header variant="h1" description="Customize transactional email and notification messages">{t('nav.notif_templates')}</Header>}>
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}
        <Container header={<Header variant="h2">Templates ({templates.length})</Header>}>
          <Table
            items={templates}
            columnDefinitions={[
              { id: 'name', header: 'Template', cell: (i) => i.name },
              { id: 'id', header: 'ID', cell: (i) => <Box variant="code">{i.id}</Box> },
              { id: 'locale', header: 'Locale', cell: (i) => <Badge color="blue">{i.locale}</Badge> },
              {
                id: 'variables', header: 'Variables',
                cell: (i) => i.variables?.length
                  ? <Box variant="small">{i.variables.map(v => `{{${v}}}`).join(', ')}</Box>
                  : <Box variant="small" color="text-status-inactive">—</Box>,
              },
              {
                id: 'actions', header: '',
                cell: (i) => <Button variant="inline-link" onClick={() => openEdit(i)}>Edit</Button>,
              },
            ]}
            empty={<Box textAlign="center" color="inherit">No templates available</Box>}
          />
        </Container>

        {selected && (
          <Modal
            size="large"
            visible={!!selected}
            onDismiss={() => setSelected(null)}
            header={`Edit: ${selected.name}`}
            footer={
              <Box float="right">
                <SpaceBetween size="xs" direction="horizontal">
                  <Button variant="link" onClick={() => setSelected(null)}>Cancel</Button>
                  <Button variant="primary" loading={saving} onClick={handleSave}>Save</Button>
                </SpaceBetween>
              </Box>
            }
          >
            <SpaceBetween size="m">
              <FormField label="Subject">
                <Input
                  value={editForm.subject ?? ''}
                  onChange={({ detail }) => setEditForm(f => ({ ...f, subject: detail.value }))}
                />
              </FormField>
              <FormField label="Locale">
                <Select
                  selectedOption={LOCALE_OPTIONS.find(o => o.value === editForm.locale) ?? LOCALE_OPTIONS[0]}
                  options={LOCALE_OPTIONS}
                  onChange={({ detail }) => setEditForm(f => ({ ...f, locale: detail.selectedOption.value }))}
                />
              </FormField>
              <Tabs
                tabs={[
                  {
                    label: 'HTML',
                    id: 'html',
                    content: (
                      <FormField label="HTML Body" constraintText={`Variables: ${(selected.variables ?? []).map(v => `{{${v}}}`).join(', ')}`}>
                        <Textarea
                          rows={12}
                          value={editForm.body_html ?? ''}
                          onChange={({ detail }) => setEditForm(f => ({ ...f, body_html: detail.value }))}
                        />
                      </FormField>
                    ),
                  },
                  {
                    label: 'Plain Text',
                    id: 'text',
                    content: (
                      <FormField label="Plain Text Body">
                        <Textarea
                          rows={12}
                          value={editForm.body_text ?? ''}
                          onChange={({ detail }) => setEditForm(f => ({ ...f, body_text: detail.value }))}
                        />
                      </FormField>
                    ),
                  },
                ]}
              />
            </SpaceBetween>
          </Modal>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
