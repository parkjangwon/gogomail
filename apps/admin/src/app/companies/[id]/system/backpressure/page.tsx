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

interface BackpressureState {
  enabled: boolean;
  threshold: number;
  current_level: number;
  status: 'normal' | 'warning' | 'critical';
  last_updated: string;
}

export default function BackpressurePage() {
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
      <ContentLayout header={<Header variant="h1">Backpressure Control</Header>}>
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
          description="Monitor and control system backpressure"
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              Update Threshold
            </Button>
          }
        >
          Backpressure Control
        </Header>
      }
    >
      <SpaceBetween size="l">
        {state && (
          <>
            <Container header={<Header variant="h3">Current Status</Header>}>
              <KeyValuePairs
                items={[
                  { label: 'Status', value: <Badge color={getStatusColor(state.status)}>{state.status.toUpperCase()}</Badge> },
                  { label: 'Enabled', value: <Badge color={state.enabled ? 'green' : 'grey'}>{state.enabled ? 'Enabled' : 'Disabled'}</Badge> },
                ]}
              />
            </Container>

            <Container header={<Header variant="h3">Metrics</Header>}>
              <KeyValuePairs
                items={[
                  { label: 'Current Level', value: `${state.current_level}%` },
                  { label: 'Threshold', value: `${state.threshold}%` },
                  { label: 'Last Updated', value: new Date(state.last_updated).toLocaleString() },
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
              <Button onClick={() => setShowModal(false)}>Cancel</Button>
              <Button variant="primary" onClick={handleUpdateThreshold}>
                Update
              </Button>
            </SpaceBetween>
          </Box>
        }
        header="Update Backpressure Threshold"
      >
        <FormField label="Threshold (%)" description="Value between 0 and 100">
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
