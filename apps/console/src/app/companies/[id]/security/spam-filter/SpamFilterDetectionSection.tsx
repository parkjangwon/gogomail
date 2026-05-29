'use client';

import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Toggle,
  RadioGroup,
  ColumnLayout,
} from '@cloudscape-design/components';
import { SpamFilterPolicy } from './spamFilterTypes';

interface SpamFilterDetectionSectionProps {
  policy: SpamFilterPolicy;
  onUpdatePolicy: (updater: (p: SpamFilterPolicy) => SpamFilterPolicy) => void;
  t: (key: string, defaultValue?: string) => string;
}

export function SpamFilterDetectionSection({ policy, onUpdatePolicy, t }: SpamFilterDetectionSectionProps) {
  return (
    <Container header={<Header variant="h2">{t('pages.spam_filter_page.detection_section')}</Header>}>
      <SpaceBetween size="m">
        <ColumnLayout columns={2}>
          <FormField
            label={t('pages.spam_filter_page.threshold_label')}
            constraintText={t('pages.spam_filter_page.threshold_hint')}
          >
            <Input
              type="number"
              value={String(policy.spam_threshold)}
              onChange={e => {
                const v = parseInt(e.detail.value) || 1;
                onUpdatePolicy(p => ({ ...p, spam_threshold: Math.max(1, Math.min(10, v)) }));
              }}
            />
          </FormField>
          <FormField label={t('pages.spam_filter_page.virus_scan_label')} description={t('pages.spam_filter_page.virus_scan_desc')}>
            <Toggle
              checked={policy.virus_scan_enabled}
              onChange={e => onUpdatePolicy(p => ({ ...p, virus_scan_enabled: e.detail.checked }))}
            >
              {policy.virus_scan_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
            </Toggle>
          </FormField>
          <FormField label={t('pages.spam_filter_page.strict_auth_label')} description={t('pages.spam_filter_page.strict_auth_desc')}>
            <Toggle
              checked={policy.strict_auth_enabled}
              onChange={e => onUpdatePolicy(p => ({ ...p, strict_auth_enabled: e.detail.checked }))}
            >
              {policy.strict_auth_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
            </Toggle>
          </FormField>
          <FormField
            label={t('pages.spam_filter_page.bulk_limit_label')}
            constraintText={t('pages.spam_filter_page.bulk_limit_hint')}
          >
            <Input
              type="number"
              value={String(policy.bulk_recipient_limit)}
              onChange={e => {
                const v = parseInt(e.detail.value) || 1;
                onUpdatePolicy(p => ({ ...p, bulk_recipient_limit: Math.max(1, Math.min(500, v)) }));
              }}
            />
          </FormField>
        </ColumnLayout>

        <FormField label={t('pages.spam_filter_page.action_label')} description={t('pages.spam_filter_page.action_desc')}>
          <RadioGroup
            value={policy.quarantine_enabled ? 'quarantine' : 'reject'}
            onChange={e => onUpdatePolicy(p => ({ ...p, quarantine_enabled: e.detail.value === 'quarantine' }))}
            items={[
              { value: 'quarantine', label: t('pages.spam_filter_page.action_quarantine') },
              { value: 'reject', label: t('pages.spam_filter_page.action_reject') },
            ]}
          />
        </FormField>
      </SpaceBetween>
    </Container>
  );
}
