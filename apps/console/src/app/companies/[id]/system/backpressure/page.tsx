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
  Input,
  KeyValuePairs,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface BackpressureState {
  enabled: boolean;
  threshold: number;
  current_level: number;
  status: 'normal' | 'warning' | 'critical';
  last_updated: string;
}

export default function BackpressurePage() {
  const { t } = useI18n();
  const [state, setState] = useState<BackpressureState | null>(null);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [newThreshold, setNewThreshold] = useState('');

  useEffect(() => {
    fetchBackpressureState();
    const interval = setInterval(fetchBackpressureState, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchBackpressureState = async () => {
    try {
      const res = await fetch('/api/admin/backpressure', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setState(data);
        setNewThreshold(data.threshold.toString());
      }
    } catch (error) {
      console.error('Failed to fetch backpressure state:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateThreshold = async () => {
    try {
      await fetch('/api/admin/backpressure', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ threshold: parseInt(newThreshold) }),
        credentials: 'include',
      });
      setShowModal(false);
      fetchBackpressureState();
    } catch (error) {
      console.error('Failed to update backpressure:', error);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'normal': return 'green';
      case 'warning': return 'severity-high';
      case 'critical': return 'red';
      default: return 'grey';
    }
  };

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.backpressure.title')}</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.backpressure.description')}
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
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
                  { label: t('pages.backpressure_page.status'), value: <Badge color={getStatusColor(state.status)}>{state.status.toUpperCase()}</Badge> },
                  { label: t('pages.backpressure_page.enabled'), value: <Badge color={state.enabled ? 'green' : 'grey'}>{state.enabled ? t('pages.backpressure_page.enabled_label') : t('pages.backpressure.disabled')}</Badge> },
                ]}
              />
            </Container>

            <Container header={<Header variant="h3">{t('pages.backpressure_page.metrics')}</Header>}>
              <KeyValuePairs
                items={[
                  { label: t('pages.backpressure_page.current_level'), value: `${state.current_level}%` },
                  { label: t('pages.backpressure_page.threshold'), value: `${state.threshold}%` },
                  { label: t('pages.backpressure_page.last_updated'), value: new Date(state.last_updated).toLocaleString() },
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
              <Button variant="primary" onClick={handleUpdateThreshold}>
                {t('pages.backpressure_page.update')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.backpressure_page.modal_header')}
      >
        <FormField label={t('pages.backpressure_page.threshold_label')} description={t('pages.backpressure_page.threshold_desc')}>
          <Input
            type="number"
            value={newThreshold}
            onChange={(e) => setNewThreshold(e.detail.value)}
          />
        </FormField>
      </Modal>
    </ContentLayout>
  );
}
