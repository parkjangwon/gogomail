'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Badge,
  Button,
  Input,
  Alert,
  Box,
  FormField,
} from '@cloudscape-design/components';
import { useState } from 'react';

interface SpamFilterSenderListsProps {
  blockedSenders: string[];
  allowedSenders: string[];
  onRemoveBlockedSender: (index: number) => void;
  onAddBlockedSender: (value: string) => void;
  onRemoveAllowedSender: (index: number) => void;
  onAddAllowedSender: (value: string) => void;
  t: (key: string, defaultValue?: string) => string;
}

export function SpamFilterSenderLists({
  blockedSenders,
  allowedSenders,
  onRemoveBlockedSender,
  onAddBlockedSender,
  onRemoveAllowedSender,
  onAddAllowedSender,
  t,
}: SpamFilterSenderListsProps) {
  const [newBlockedSender, setNewBlockedSender] = useState('');
  const [newAllowedSender, setNewAllowedSender] = useState('');

  return (
    <Container header={<Header variant="h2">{t('pages.spam_filter_page.senders_section')}</Header>}>
      <SpaceBetween size="m">
        <FormField label={t('pages.spam_filter_page.blocked_senders_label')} description={t('pages.spam_filter_page.blocked_senders_desc')}>
          <SpaceBetween size="xs">
            {blockedSenders.length === 0 && (
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_blocked_senders')}</Box>
            )}
            {blockedSenders.map((s, i) => (
              <SpaceBetween key={i} direction="horizontal" size="xs">
                <Badge color="red">{s}</Badge>
                <Button variant="inline-link" onClick={() => onRemoveBlockedSender(i)}>
                  {t('common.delete')}
                </Button>
              </SpaceBetween>
            ))}
            <SpaceBetween direction="horizontal" size="xs">
              <Input
                value={newBlockedSender}
                onChange={e => setNewBlockedSender(e.detail.value)}
                placeholder="spam@example.com or @domain.com"
              />
              <Button onClick={() => {
                const trimmed = newBlockedSender.trim();
                if (!trimmed) return;
                onAddBlockedSender(trimmed);
                setNewBlockedSender('');
              }}>
                {t('common.add')}
              </Button>
            </SpaceBetween>
          </SpaceBetween>
        </FormField>

        <FormField label={t('pages.spam_filter_page.allowed_senders_label')} description={t('pages.spam_filter_page.allowed_senders_desc')}>
          <SpaceBetween size="xs">
            {allowedSenders.length > 0 && (
              <Alert type="info">{t('pages.spam_filter_page.allowed_senders_warning')}</Alert>
            )}
            {allowedSenders.length === 0 && (
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_allowed_senders')}</Box>
            )}
            {allowedSenders.map((s, i) => (
              <SpaceBetween key={i} direction="horizontal" size="xs">
                <Badge color="green">{s}</Badge>
                <Button variant="inline-link" onClick={() => onRemoveAllowedSender(i)}>
                  {t('common.delete')}
                </Button>
              </SpaceBetween>
            ))}
            <SpaceBetween direction="horizontal" size="xs">
              <Input
                value={newAllowedSender}
                onChange={e => setNewAllowedSender(e.detail.value)}
                placeholder="trusted@partner.com or @trusted.com"
              />
              <Button onClick={() => {
                const trimmed = newAllowedSender.trim();
                if (!trimmed) return;
                onAddAllowedSender(trimmed);
                setNewAllowedSender('');
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
