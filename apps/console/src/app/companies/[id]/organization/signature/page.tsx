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
import { useI18n } from '@/app/i18n-provider';
import { useCompanySignature, useUpdateCompanySignature } from '@/hooks';

interface SignatureConfig {
  html: string;
  text: string;
  enabled: boolean;
}

export default function GlobalSignaturePage() {
  const { t } = useI18n();
  const params = useParams();
  const companyId = params?.id as string;
  const signatureQuery = useCompanySignature(companyId);
  const updateSignature = useUpdateCompanySignature();

  const [config, setConfig] = useState<SignatureConfig>({ html: '', text: '', enabled: false });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'success' | 'error'>('idle');
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(signatureQuery.isLoading);
    if (signatureQuery.data) {
      setConfig(signatureQuery.data as unknown as SignatureConfig);
    }
    if (signatureQuery.isError) {
      setError(t('global_signature.load_error'));
    }
  }, [signatureQuery.data, signatureQuery.isError, signatureQuery.isLoading, t]);

  const handleSave = async () => {
    setSaving(true);
    setSaveStatus('idle');
    try {
      await updateSignature.mutateAsync({ companyId, data: config });
      setSaveStatus('success');
    } catch {
      setSaveStatus('error');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('global_signature.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('global_signature.page_description')}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => signatureQuery.refetch()}>{t('global_signature.discard')}</Button>
              <Button variant="primary" onClick={handleSave} loading={saving}>{t('common.save')}</Button>
            </SpaceBetween>
          }
        >
          {t('global_signature.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {error ? <Alert key="load-error" type="error">{error}</Alert> : null}
        {saveStatus === 'success' ? (
          <Alert key="save-success" type="success" dismissible onDismiss={() => setSaveStatus('idle')}>
            {t('global_signature.save_success')}
          </Alert>
        ) : null}
        {saveStatus === 'error' ? (
          <Alert key="save-error" type="error" dismissible onDismiss={() => setSaveStatus('idle')}>
            {t('global_signature.save_error')}
          </Alert>
        ) : null}

        <Container
          key="settings"
          header={
            <Header
              variant="h2"
              actions={
                <Toggle
                  checked={config.enabled}
                  onChange={e => setConfig(c => ({ ...c, enabled: e.detail.checked }))}
                >
                  {config.enabled ? t('status.enabled') : t('status.disabled')}
                </Toggle>
              }
            >
              {t('global_signature.settings_header')}
            </Header>
          }
        >
          {!config.enabled ? (
            <Alert key="disabled-info" type="info">
              {t('global_signature.disabled_info')}
            </Alert>
          ) : null}
        </Container>

        <Container key="content" header={<Header variant="h2">{t('global_signature.content_header')}</Header>}>
          <Tabs
            tabs={[
              {
                label: t('global_signature.html_tab'),
                id: 'html',
                content: (
                  <SpaceBetween size="s">
                    <Box color="text-body-secondary" fontSize="body-s">
                      {t('global_signature.html_desc')}
                    </Box>
                    <Textarea
                      value={config.html}
                      onChange={e => setConfig(c => ({ ...c, html: e.detail.value }))}
                      placeholder={t('global_signature.html_placeholder')}
                      rows={12}
                    />
                    {config.html ? (
                      <SpaceBetween key="preview" size="xs">
                        <Box fontWeight="bold" fontSize="body-s">{t('global_signature.preview')}</Box>
                        <div
                          style={{ border: '1px solid var(--color-border-divider-default)', borderRadius: 4, padding: 16, background: '#fff', color: '#000' }}
                          dangerouslySetInnerHTML={{ __html: config.html }}
                        />
                      </SpaceBetween>
                    ) : null}
                  </SpaceBetween>
                ),
              },
              {
                label: t('global_signature.text_tab'),
                id: 'text',
                content: (
                  <SpaceBetween size="s">
                    <Box color="text-body-secondary" fontSize="body-s">
                      {t('global_signature.text_desc')}
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
