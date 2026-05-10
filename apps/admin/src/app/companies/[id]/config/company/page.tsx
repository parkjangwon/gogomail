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
  Modal,
  FormField,
  Input,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

interface ConfigEntry {
  id: string;
  key: string;
  value: string;
  last_updated: string;
}

export default function CompanyConfigPage() {
  const [configs, setConfigs] = useState<ConfigEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newConfig, setNewConfig] = useState({ key: '', value: '' });

  useEffect(() => {
    fetchCompanyConfig();
  }, []);

  const fetchCompanyConfig = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/config/company?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setConfigs(data.config || []);
      }
    } catch (error) {
      console.error('Failed to fetch company config:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateConfig = async () => {
    try {
      await fetch('/api/admin/config/company', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newConfig),
        credentials: 'include',
      });
      setShowModal(false);
      setNewConfig({ key: '', value: '' });
      fetchCompanyConfig();
    } catch (error) {
      console.error('Failed to create config:', error);
    }
  };

  const filteredConfigs = configs.filter(c =>
    c.key.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Company Config</Header>}>
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
          description="Manage company-level configuration"
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              + Add Config
            </Button>
          }
        >
          Company Config
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Key',
              cell: (item: ConfigEntry) => item.key,
              width: '30%',
            },
            {
              header: 'Value',
              cell: (item: ConfigEntry) => item.value,
              width: '50%',
            },
            {
              header: 'Last Updated',
              cell: (item: ConfigEntry) => new Date(item.last_updated).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredConfigs}
          header={<Header variant="h2" counter={`(${filteredConfigs.length})`}>Config</Header>}
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
              <Button variant="primary" onClick={handleCreateConfig}>
                Add Config
              </Button>
            </SpaceBetween>
          </Box>
        }
        header="Add Configuration"
      >
        <SpaceBetween size="m">
          <FormField label="Key">
            <Input
              value={newConfig.key}
              onChange={(e) => setNewConfig({ ...newConfig, key: e.detail.value })}
              placeholder="e.g., max_users"
            />
          </FormField>
          <FormField label="Value">
            <Input
              value={newConfig.value}
              onChange={(e) => setNewConfig({ ...newConfig, value: e.detail.value })}
              placeholder="Configuration value"
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
