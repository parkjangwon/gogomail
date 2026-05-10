'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Badge,
  Modal,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface SuppressionEntry {
  id: string;
  email: string;
  reason: string;
  added_at: string;
}

export default function SuppressionListPage() {
  const { t: _unused } = useI18n(); _unused;
  const [entries, setEntries] = useState<SuppressionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newEntry, setNewEntry] = useState({ email: '', reason: '' });

  useEffect(() => {
    fetchSuppressionList();
  }, []);

  const fetchSuppressionList = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/suppression-list?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setEntries(data.entries || []);
      }
    } catch (error) {
      console.error('Failed to fetch suppression list:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleAddEntry = async () => {
    try {
      await fetch('/api/admin/suppression-list', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newEntry),
        credentials: 'include',
      });
      setShowModal(false);
      setNewEntry({ email: '', reason: '' });
      fetchSuppressionList();
    } catch (error) {
      console.error('Failed to add suppression entry:', error);
    }
  };

  const filteredEntries = entries.filter(e =>
    e.email.toLowerCase().includes(filter.toLowerCase()) ||
    e.reason.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Suppression List</Header>}>
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
          description="Manage email suppression list"
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              + Add Email
            </Button>
          }
        >
          Suppression List
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Email',
              cell: (item: SuppressionEntry) => item.email,
              width: '40%',
            },
            {
              header: 'Reason',
              cell: (item: SuppressionEntry) => (
                <Badge color="red">{item.reason}</Badge>
              ),
              width: '35%',
            },
            {
              header: 'Added',
              cell: (item: SuppressionEntry) => new Date(item.added_at).toLocaleString(),
              width: '25%',
            },
          ]}
          items={filteredEntries}
          header={<Header variant="h2" counter={`(${filteredEntries.length})`}>Entries</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>

      <Modal
        onDismiss={() => setShowModal(false)}
        visible={showModal}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowModal(false)}>Cancel</Button>
              <Button variant="primary" onClick={handleAddEntry}>
                Add Email
              </Button>
            </SpaceBetween>
          </Box>
        }
        header="Add to Suppression List"
      >
        <SpaceBetween size="m">
          <FormField label="Email Address">
            <Input
              value={newEntry.email}
              onChange={(e) => setNewEntry({ ...newEntry, email: e.detail.value })}
              placeholder="email@example.com"
            />
          </FormField>
          <FormField label="Reason" description="Why this email is suppressed">
            <Input
              value={newEntry.reason}
              onChange={(e) => setNewEntry({ ...newEntry, reason: e.detail.value })}
              placeholder="e.g., Hard bounce, Complaint"
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
