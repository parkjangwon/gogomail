'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Button,
  Modal,
  FormField,
  Select,
  type SelectProps,
  KeyValuePairs,
} from '@cloudscape-design/components';
import { useState } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useAdminBackpressure, useUpdateAdminBackpressure, type BackpressureUpdateRequest } from '@/hooks';

const LEVEL_OPTIONS: SelectProps.Option[] = [
  { value: 'normal', label: 'Normal' },
  { value: 'warning', label: 'Warning' },
  { value: 'critical', label: 'Critical' },
];

const thresholdForLevel = (level: string) => {
  switch (level) {
    case 'normal': return 0;
    case 'warning': return 40;
    case 'critical': return 90;
    default: return 0;
  }
};

const getStatusColor = (status: string) => {
  switch (status) {
    case 'normal':
      return 'green';
    case 'warning':
      return 'severity-high';
    case 'critical':
      return 'red';
    default:
      return 'grey';
  }
};

export default function BackpressurePage() {
  const { t } = useI18n();
  const backpressureQuery = useAdminBackpressure(5_000);
  const updateBackpressure = useUpdateAdminBackpressure();
  const state = backpressureQuery.data?.backpressure ?? null;
  const [showModal, setShowModal] = useState(false);
  const [newLevel, setNewLevel] = useState<SelectProps.Option>(LEVEL_OPTIONS[0]);

  if (backpressureQuery.isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.backpressure.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  if (backpressureQuery.isError && !state) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.backpressure.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <SpaceBetween size="m" alignItems="center">
            <Box color="text-status-error">{t('pages.backpressure.no_data')}</Box>
            <Button iconName="refresh" onClick={() => backpressureQuery.refetch()}>{t('common.retry')}</Button>
          </SpaceBetween>
        </Box>
      </ContentLayout>
    );
  }

  const handleUpdateThreshold = async () => {
    if (!state) return;
    await updateBackpressure.mutateAsync({
      level: (newLevel.value ?? 'normal') as BackpressureUpdateRequest['level'],
      reason: state.reason,
      until: state.until,
    });
    setShowModal(false);
  };

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.backpressure.description')}
          actions={
            <Button variant="primary" onClick={() => {
              setNewLevel(LEVEL_OPTIONS.find(o => o.value === state?.level) ?? LEVEL_OPTIONS[0]);
              setShowModal(true);
            }}>
              {t('pages.backpressure_page.update_threshold')}
            </Button>
          }
        >
          {t('pages.backpressure.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {state && (
          <>
            <Container header={<Header variant="h3">{t('pages.backpressure_page.current_status')}</Header>}>
              <KeyValuePairs
                items={[
                  {
                    label: t('pages.backpressure_page.status'),
                    value: <Badge color={getStatusColor(state.level as string)}>{String(state.level).toUpperCase()}</Badge>,
                  },
                  {
                    label: t('pages.backpressure_page.enabled'),
                    value: (
                      <Badge color={state.level === 'normal' ? 'grey' : state.level === 'critical' ? 'red' : 'severity-high'}>
                        {state.level === 'normal' ? t('pages.backpressure.disabled') : t('pages.backpressure_page.enabled_label')}
                      </Badge>
                    ),
                  },
                ]}
              />
            </Container>

            <Container header={<Header variant="h3">{t('pages.backpressure_page.metrics')}</Header>}>
              <KeyValuePairs
                items={[
                  { label: t('pages.backpressure_page.threshold'), value: `${thresholdForLevel(state.level)}%` },
                  ...(state.reason ? [{ label: t('pages.backpressure_page.reason'), value: state.reason }] : []),
                  ...(state.until ? [{ label: t('pages.backpressure_page.until'), value: new Date(state.until).toLocaleString() }] : []),
                  { label: t('pages.backpressure_page.last_updated'), value: new Date(state.updated_at ?? Date.now()).toLocaleString() },
                ]}
              />
            </Container>
          </>
        )}
      </SpaceBetween>

      <Modal
        onDismiss={() => setShowModal(false)}
        visible={showModal}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={handleUpdateThreshold} loading={updateBackpressure.isPending}>
                {t('pages.backpressure_page.update')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.backpressure_page.modal_header')}
      >
        <FormField label={t('pages.backpressure_page.threshold_label')} description={t('pages.backpressure_page.threshold_desc')}>
          <Select
            selectedOption={newLevel}
            options={LEVEL_OPTIONS}
            onChange={({ detail }) => setNewLevel(detail.selectedOption)}
          />
        </FormField>
      </Modal>
    </ContentLayout>
  );
}
