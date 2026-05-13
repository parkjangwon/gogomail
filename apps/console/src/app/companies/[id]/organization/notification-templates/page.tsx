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
  Box,
  Spinner,
  Flashbar,
  FlashbarProps,
  Table,
  Modal,
  Badge,
  ColumnLayout,
  Toggle,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback, useMemo } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface NotifTemplate {
  id: string;
  name: string;
  subject: string;
  body: string;
  enabled: boolean;
}

// Substitute {{variable}} placeholders with sample values
function interpolate(tmpl: string, samples: Record<string, string>): string {
  return tmpl.replace(/\{\{\.?([\w]+)\}\}/g, (_, key) => samples[key] ?? `[${key}]`);
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

  const load = useCallback(async () => {
    if (!cid) return;
    setLoading(true);
    try {
      const res = await fetch(`/admin/v1/companies/${cid}/notification-templates`);
      const data = await res.json();
      setTemplates(data.templates ?? []);
    } catch {
      setFlash([{ type: 'error', content: t('notif_templates_page.failed_load'), dismissible: true, onDismiss: () => setFlash([]) }]);
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
      setFlash([{ type: 'success', content: t('notif_templates_page.saved'), dismissible: true, onDismiss: () => setFlash([]) }]);
      setSelected(null);
      load();
    } catch (e: unknown) {
      setFlash([{ type: 'error', content: String(e), dismissible: true, onDismiss: () => setFlash([]) }]);
    } finally {
      setSaving(false);
    }
  };

  const previewHtml = useMemo(() => {
    const html = editForm.body ?? '';
    const subject = editForm.subject ?? '';
    const samples = {
      CompanyName: 'GogoMail',
      ResetURL: 'https://mail.example.com/reset',
      UserName: 'user@example.com',
      UsagePercent: '85',
    };
    const renderedSubject = interpolate(subject, samples);
    const renderedBody = interpolate(html, samples);
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
    <div class="subject-bar">${t('notif_templates_page.preview_subject')}: ${renderedSubject}</div>
    <div class="body">${renderedBody || `<p style="color:#aaa;font-style:italic">${t('notif_templates_page.no_html_content')}</p>`}</div>
  </div>
</body>
</html>`;
  }, [editForm.body, editForm.subject, t]);

  if (loading) return <Box padding="xl"><Spinner /></Box>;

  return (
    <ContentLayout header={<Header variant="h1" description={t('notif_templates_page.description')}>{t('nav.notif_templates')}</Header>}>
      <SpaceBetween size="m">
        {flash.length > 0 && <Flashbar items={flash} />}

        <Container header={<Header variant="h2">{t('notif_templates_page.templates')} ({templates.length})</Header>}>
          <Table
            items={templates}
            columnDefinitions={[
              { id: 'name', header: t('notif_templates_page.template'), cell: (i) => i.name },
              { id: 'id', header: 'ID', cell: (i) => <Box variant="code">{i.id}</Box> },
              { id: 'status', header: t('notif_templates_page.status'), cell: (i) => <Badge color={i.enabled ? 'green' : 'grey'}>{i.enabled ? t('status.enabled') : t('status.disabled')}</Badge> },
              {
                id: 'actions', header: '',
                cell: (i) => <Button variant="inline-link" onClick={() => openEdit(i)}>{t('notif_templates_page.edit_preview')}</Button>,
              },
            ]}
            empty={<Box textAlign="center" color="inherit">{t('notif_templates_page.empty')}</Box>}
          />
        </Container>

        {selected && (
          <Modal
            size="max"
            visible={!!selected}
            onDismiss={() => setSelected(null)}
            header={`${t('notif_templates_page.edit')}: ${selected.name}`}
            footer={
              <Box float="right">
                <SpaceBetween size="xs" direction="horizontal">
                  <Button variant="link" onClick={() => setSelected(null)}>{t('common.cancel')}</Button>
                  <Button variant="primary" loading={saving} onClick={handleSave}>{t('common.save')}</Button>
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
                <FormField label={t('pages.notification_templates_page.html_body')}>
                  <Textarea
                    rows={16}
                    value={editForm.body ?? ''}
                    onChange={({ detail }) => setEditForm(f => ({ ...f, body: detail.value }))}
                  />
                </FormField>
                <Toggle checked={editForm.enabled ?? true} onChange={({ detail }) => setEditForm(f => ({ ...f, enabled: detail.checked }))}>
                  {t('status.enabled')}
                </Toggle>
              </SpaceBetween>

              {/* ── Right: Preview ── */}
              <SpaceBetween size="m">
                <Header variant="h3" description={t('notif_templates_page.preview_desc')}>{t('notif_templates_page.preview_title')}</Header>
                <Box>
                  <iframe
                    srcDoc={previewHtml}
                    title={t('notif_templates_page.preview_iframe_title')}
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
              </SpaceBetween>
            </ColumnLayout>
          </Modal>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
