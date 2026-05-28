'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Badge,
  Button,
  Input,
  FormField,
} from '@cloudscape-design/components';
import { useState } from 'react';

interface SpamFilterAttachmentSettingsProps {
  maxAttachmentMb: number;
  onMaxChange: (mb: number) => void;
  blockedExtensions: string[];
  onRemoveExtension: (index: number) => void;
  onAddExtension: (value: string) => void;
  t: (key: string, defaultValue?: string) => string;
}

export function SpamFilterAttachmentSettings({
  maxAttachmentMb,
  onMaxChange,
  blockedExtensions,
  onRemoveExtension,
  onAddExtension,
  t,
}: SpamFilterAttachmentSettingsProps) {
  const [newBlockedExt, setNewBlockedExt] = useState('');

  return (
    <Container header={<Header variant="h2">{t('pages.spam_filter_page.attachments_section')}</Header>}>
      <SpaceBetween size="m">
        <FormField
          label={t('pages.spam_filter_page.max_attachment_label')}
          constraintText={t('pages.spam_filter_page.max_attachment_hint')}
        >
          <Input
            type="number"
            value={String(maxAttachmentMb)}
            onChange={e => onMaxChange(parseInt(e.detail.value) || 0)}
          />
        </FormField>

        <FormField label={t('pages.spam_filter_page.blocked_ext_label')} description={t('pages.spam_filter_page.blocked_ext_desc')}>
          <SpaceBetween size="xs">
            <SpaceBetween direction="horizontal" size="xs">
              {blockedExtensions.map((ext, i) => (
                <SpaceBetween key={i} direction="horizontal" size="xs">
                  <Badge color="red">{ext}</Badge>
                  <Button variant="inline-link" onClick={() => onRemoveExtension(i)}>
                    {t('common.delete')}
                  </Button>
                </SpaceBetween>
              ))}
            </SpaceBetween>
            <SpaceBetween direction="horizontal" size="xs">
              <Input
                value={newBlockedExt}
                onChange={e => setNewBlockedExt(e.detail.value)}
                placeholder=".exe"
              />
              <Button onClick={() => {
                const trimmed = newBlockedExt.trim();
                if (!trimmed) return;
                onAddExtension(trimmed);
                setNewBlockedExt('');
              }}>
                {t('common.add')}
              </Button>
            </SpaceBetween>
          </SpaceBetween>
        </FormField>
      </SpaceBetween>
    </Container>
  );
}
