'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  Button,
  Textarea,
  Toggle,
  Alert,
  Tabs,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';

interface SignatureConfig {
  html: string;
  text: string;
  enabled: boolean;
}

export default function GlobalSignaturePage() {
  const params = useParams();
  const companyId = params?.id as string;

  const [config, setConfig] = useState<SignatureConfig>({ html: '', text: '', enabled: false });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'success' | 'error'>('idle');
  const [error, setError] = useState('');

  const fetchSignature = async () => {
    if (!companyId) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/signature`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setConfig(data.signature || { html: '', text: '', enabled: false });
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSignature();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [companyId]);

  const handleSave = async () => {
    setSaving(true);
    setSaveStatus('idle');
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/signature`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
      if (!res.ok) { setSaveStatus('error'); return; }
      setSaveStatus('success');
    } catch {
      setSaveStatus('error');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Global Email Signature</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description="Append a company-wide signature to all outbound emails."
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={fetchSignature}>Discard</Button>
              <Button variant="primary" onClick={handleSave} loading={saving}>Save</Button>
            </SpaceBetween>
          }
        >
          Global Email Signature
        </Header>
      }
    >
      <SpaceBetween size="l">
        {error && <Alert type="error">{error}</Alert>}
        {saveStatus === 'success' && <Alert type="success" dismissible onDismiss={() => setSaveStatus('idle')}>Signature saved successfully.</Alert>}
        {saveStatus === 'error' && <Alert type="error" dismissible onDismiss={() => setSaveStatus('idle')}>Failed to save signature. Please try again.</Alert>}

        <Container
          header={
            <Header
              variant="h2"
              actions={
                <Toggle
                  checked={config.enabled}
                  onChange={e => setConfig(c => ({ ...c, enabled: e.detail.checked }))}
                >
                  {config.enabled ? 'Enabled' : 'Disabled'}
                </Toggle>
              }
            >
              Signature Settings
            </Header>
          }
        >
          {!config.enabled && (
            <Alert type="info">
              The signature is currently disabled. Enable it to append to all outbound messages.
            </Alert>
          )}
        </Container>

        <Container header={<Header variant="h2">Signature Content</Header>}>
          <Tabs
            tabs={[
              {
                label: 'HTML',
                id: 'html',
                content: (
                  <SpaceBetween size="s">
                    <Box color="text-body-secondary" fontSize="body-s">
                      HTML signature appended to rich-text (HTML) emails. Use inline styles for best compatibility.
                    </Box>
                    <Textarea
                      value={config.html}
                      onChange={e => setConfig(c => ({ ...c, html: e.detail.value }))}
                      placeholder="<p>Best regards,<br/><strong>Acme Corp</strong><br/>support@acme.com</p>"
                      rows={12}
                    />
                    {config.html && (
                      <SpaceBetween size="xs">
                        <Box fontWeight="bold" fontSize="body-s">Preview</Box>
                        <div
                          style={{ border: '1px solid var(--color-border-divider-default)', borderRadius: 4, padding: 16, background: '#fff', color: '#000' }}
                          dangerouslySetInnerHTML={{ __html: config.html }}
                        />
                      </SpaceBetween>
                    )}
                  </SpaceBetween>
                ),
              },
              {
                label: 'Plain Text',
                id: 'text',
                content: (
                  <SpaceBetween size="s">
                    <Box color="text-body-secondary" fontSize="body-s">
                      Plain-text signature appended to text-only emails.
                    </Box>
                    <Textarea
                      value={config.text}
                      onChange={e => setConfig(c => ({ ...c, text: e.detail.value }))}
                      placeholder={"Best regards,\nAcme Corp\nsupport@acme.com"}
                      rows={8}
                    />
                  </SpaceBetween>
                ),
              },
            ]}
          />
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
