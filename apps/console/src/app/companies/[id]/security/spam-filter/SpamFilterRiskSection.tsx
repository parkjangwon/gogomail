'use client';

import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Box,
  Alert,
  ColumnLayout,
  Badge,
} from '@cloudscape-design/components';

interface SpamFilterRiskSectionProps {
  riskItems: string[];
  postureLabel: string;
  changedFields: string[];
  t: (key: string, defaultValue?: string) => string;
}

export function SpamFilterRiskSection({ riskItems, postureLabel, changedFields, t }: SpamFilterRiskSectionProps) {
  return (
    <Container
      header={
        <Header
          variant="h2"
          actions={<Badge color={riskItems.length >= 3 ? 'red' : riskItems.length > 0 ? 'severity-medium' : 'green'}>{postureLabel}</Badge>}
        >
          {t('pages.spam_filter_page.risk_section')}
        </Header>
      }
    >
      <SpaceBetween size="s">
        {riskItems.length === 0 ? (
          <Alert type="success">{t('pages.spam_filter_page.risk_clear')}</Alert>
        ) : (
          <Alert type={riskItems.length >= 3 ? 'error' : 'warning'}>
            {t('pages.spam_filter_page.risk_intro')}
          </Alert>
        )}
        {riskItems.length > 0 && (
          <ColumnLayout columns={2} minColumnWidth={240}>
            {riskItems.map(item => (
              <Box key={item}>
                <Badge color="severity-medium">{t('pages.spam_filter_page.review_required')}</Badge> {item}
              </Box>
            ))}
          </ColumnLayout>
        )}
        {changedFields.length > 0 && (
          <FormField label={t('pages.spam_filter_page.changed_fields_label')}>
            <SpaceBetween direction="horizontal" size="xs">
              {changedFields.map(field => <Badge key={field} color="blue">{field}</Badge>)}
            </SpaceBetween>
          </FormField>
        )}
      </SpaceBetween>
    </Container>
  );
}
