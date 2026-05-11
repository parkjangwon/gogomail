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
  ColumnLayout,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback, useMemo } from 'react';
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

// Substitute {{variable}} placeholders with sample values
function interpolate(tmpl: string, samples: Record<string, string>): string {
  return tmpl.replace(/\{\{(\w+)\}\}/g, (_, key) => samples[key] ?? `[${key}]`);
}

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
  const [sampleVars, setSampleVars] = useState<Record<string, string>>({});

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
    // Seed sample values with variable names
    const seeds: Record<string, string> = {};
    (tmpl.variables ?? []).forEach(v => { seeds[v] = `sample_${v}`; });
    setSampleVars(seeds);
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

  const previewHtml = useMemo(() => {
    const html = editForm.body_html ?? '';
    const subject = editForm.subject ?? '';
    const renderedSubject = interpolate(subject, sampleVars);
    const renderedBody = interpolate(html, sampleVars);
    return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 0; background: #f4f4f4; }
  .wrapper { max-width: 600px; margin: 20px auto; background: #fff; border-radius: 6px; overflow: hidden; box-shadow: 0 1px 4px rgba(0,0,0,.1); }
  .subject-bar { background: #232f3e; color: #fff; padding: 12px 20px; font-size: 14px; font-weight: 600; }
  .body { padding: 24px 20px; }
</style>
</head>
<body>
  <div class="wrapper">
    <div class="subject-bar">Subject: ${renderedSubject}</div>
    <div class="body">${renderedBody || '<p style="color:#aaa;font-style:italic">No HTML content yet.</p>'}</div>
  </div>
</body>
</html>`;
  }, [editForm.body_html, editForm.subject, sampleVars]);

  const previewText = useMemo(
    () => interpolate(editForm.body_text ?? '', sampleVars),
    [editForm.body_text, sampleVars]
  );

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
                cell: (i) => <Button variant="inline-link" onClick={() => openEdit(i)}>Edit & Preview</Button>,
              },
            ]}
            empty={<Box textAlign="center" color="inherit">No templates available</Box>}
          />
        </Container>

        {selected && (
          <Modal
            size="max"
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
            <ColumnLayout columns={2} variant="default">
              {/* ── Left: Editor ── */}
              <SpaceBetween size="m">
                <FormField label={t('pages.notification_templates_page.subject')}>
                  <Input
                    value={editForm.subject ?? ''}
                    onChange={({ detail }) => setEditForm(f => ({ ...f, subject: detail.value }))}
                  />
                </FormField>
                <FormField label={t('pages.notification_templates_page.locale')}>
                  <Select
                    selectedOption={LOCALE_OPTIONS.find(o => o.value === editForm.locale) ?? LOCALE_OPTIONS[0]}
                    options={LOCALE_OPTIONS}
                    onChange={({ detail }) => setEditForm(f => ({ ...f, locale: detail.selectedOption.value }))}
                  />
                </FormField>

                {/* Sample variable values */}
                {(selected.variables ?? []).length > 0 && (
                  <Container header={<Header variant="h3" description="Used for preview only">Sample Values</Header>}>
                    <SpaceBetween size="xs">
                      {(selected.variables ?? []).map(v => (
                        <FormField key={v} label={`{{${v}}}`}>
                          <Input
                            value={sampleVars[v] ?? ''}
                            onChange={({ detail }) => setSampleVars(s => ({ ...s, [v]: detail.value }))}
                          />
                        </FormField>
                      ))}
                    </SpaceBetween>
                  </Container>
                )}

                <Tabs
                  tabs={[
                    {
                      label: 'HTML',
                      id: 'html',
                      content: (
                        <FormField label={t('pages.notification_templates_page.html_body')}>
                          <Textarea
                            rows={14}
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
                        <FormField label={t('pages.notification_templates_page.plain_body')}>
                          <Textarea
                            rows={14}
                            value={editForm.body_text ?? ''}
                            onChange={({ detail }) => setEditForm(f => ({ ...f, body_text: detail.value }))}
                          />
                        </FormField>
                      ),
                    },
                  ]}
                />
              </SpaceBetween>

              {/* ── Right: Preview ── */}
              <SpaceBetween size="m">
                <Header variant="h3" description="Live preview with sample values">Email Preview</Header>
                <Tabs
                  tabs={[
                    {
                      label: 'HTML',
                      id: 'html-preview',
                      content: (
                        <Box>
                          <iframe
                            srcDoc={previewHtml}
                            title="Email HTML Preview"
                            style={{
                              width: '100%',
                              height: '480px',
                              border: '1px solid #d1d5db',
                              borderRadius: '4px',
                              background: '#f4f4f4',
                            }}
                            sandbox="allow-same-origin"
                          />
                        </Box>
                      ),
                    },
                    {
                      label: 'Plain Text',
                      id: 'text-preview',
                      content: (
                        <Box>
                          <div style={{
                            whiteSpace: 'pre-wrap',
                            fontFamily: 'monospace',
                            fontSize: '13px',
                            background: '#f8f8f8',
                            border: '1px solid #d1d5db',
                            borderRadius: '4px',
                            padding: '16px',
                            minHeight: '200px',
                            color: previewText ? '#000' : '#aaa',
                          }}>
                            {previewText || 'No plain text content yet.'}
                          </div>
                        </Box>
                      ),
                    },
                  ]}
                />
              </SpaceBetween>
            </ColumnLayout>
          </Modal>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
