'use client';

import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Toggle,
  Button,
  Box,
  Badge,
  ColumnLayout,
} from '@cloudscape-design/components';
import { SpamFilterPolicy } from './spamFilterTypes';

interface SpamFilterRblSectionProps {
  policy: SpamFilterPolicy;
  onUpdatePolicy: (updater: (p: SpamFilterPolicy) => SpamFilterPolicy) => void;
  newRBLZone: string;
  onNewRBLZoneChange: (value: string) => void;
  onAddRBLZone: () => void;
  onRemoveRBLZone: (index: number) => void;
  t: (key: string, defaultValue?: string) => string;
}

export function SpamFilterRblSection({
  policy,
  onUpdatePolicy,
  newRBLZone,
  onNewRBLZoneChange,
  onAddRBLZone,
  onRemoveRBLZone,
  t,
}: SpamFilterRblSectionProps) {
  return (
    <Container header={<Header variant="h2">{t('pages.spam_filter_page.rbl_section')}</Header>}>
      <SpaceBetween size="m">
        <ColumnLayout columns={2}>
          <FormField label={t('pages.spam_filter_page.rbl_lookup_label')} description={t('pages.spam_filter_page.rbl_lookup_desc')}>
            <Toggle
              checked={policy.rbl_check_enabled}
              onChange={e => onUpdatePolicy(p => ({ ...p, rbl_check_enabled: e.detail.checked }))}
            >
              {policy.rbl_check_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
            </Toggle>
          </FormField>
          <FormField label={t('pages.spam_filter_page.rbl_reject_label')} description={t('pages.spam_filter_page.rbl_reject_desc')}>
            <Toggle
              checked={policy.rbl_reject_enabled}
              disabled={!policy.rbl_check_enabled}
              onChange={e => onUpdatePolicy(p => ({ ...p, rbl_reject_enabled: e.detail.checked }))}
            >
              {policy.rbl_reject_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
            </Toggle>
          </FormField>
        </ColumnLayout>
        <FormField label={t('pages.spam_filter_page.rbl_zones_label')} description={t('pages.spam_filter_page.rbl_zones_desc')}>
          <SpaceBetween size="xs">
            {policy.rbl_zones.length === 0 && (
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_rbl_zones')}</Box>
            )}
            <SpaceBetween direction="horizontal" size="xs">
              {policy.rbl_zones.map((zone, i) => (
                <SpaceBetween key={zone} direction="horizontal" size="xs">
                  <Badge color="blue">{zone}</Badge>
                  <Button variant="inline-link" onClick={() => onRemoveRBLZone(i)}>
                    {t('common.delete')}
                  </Button>
                </SpaceBetween>
              ))}
            </SpaceBetween>
            <SpaceBetween direction="horizontal" size="xs">
              <Input
                value={newRBLZone}
                onChange={e => onNewRBLZoneChange(e.detail.value)}
                placeholder="zen.example-rbl.test"
              />
              <Button onClick={onAddRBLZone}>
                {t('common.add')}
              </Button>
            </SpaceBetween>
          </SpaceBetween>
        </FormField>
      </SpaceBetween>
    </Container>
  );
}
