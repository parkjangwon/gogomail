'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Badge,
  Modal,
  FormField,
  Input,
  TextFilter,
  Tabs,
  Container,
  KeyValuePairs,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface Domain {
  id: string;
  name: string;
  status: string;
  dns_verified: boolean;
  quota_gb: number;
  user_count: number;
  created_at: string;
  dkim_status?: string;
  spf_status?: string;
  dmarc_status?: string;
}

export default function DomainsPage() {
  const { t } = useI18n();
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [selectedDomain, setSelectedDomain] = useState<Domain | null>(null);
  const [showDetailsModal, setShowDetailsModal] = useState(false);
  const [newDomain, setNewDomain] = useState({ name: '', quota_gb: 100 });

  useEffect(() => {
    fetchDomains();
  }, []);

  const fetchDomains = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/domains?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setDomains(data.domains || []);
      }
    } catch (error) {
      console.error('Failed to fetch domains:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateDomain = async () => {
    try {
      const res = await fetch('/api/admin/domains', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newDomain),
        credentials: 'include',
      });
      if (res.ok) {
        setShowModal(false);
        setNewDomain({ name: '', quota_gb: 100 });
        fetchDomains();
      }
    } catch (error) {
      console.error('Failed to create domain:', error);
    }
  };

  const handleVerifyDNS = async (domainId: string) => {
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/verify-dns`, {
        method: 'POST',
        credentials: 'include',
      });
      if (res.ok) {
        fetchDomains();
      }
    } catch (error) {
      console.error('Failed to verify DNS:', error);
    }
  };

  const filteredDomains = domains.filter(d =>
    d.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.domains.title')}</Header>}>
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
          description="Manage email domains and DNS configuration"
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              {t('pages.domains.create_domain')}
            </Button>
          }
        >
          Domains
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Domain',
              cell: (domain: Domain) => (
                <Button
                  variant="inline-link"
                  onClick={() => {
                    setSelectedDomain(domain);
                    setShowDetailsModal(true);
                  }}
                >
                  {domain.name}
                </Button>
              ),
              width: '25%',
            },
            {
              header: t('pages.domains.status'),
              cell: (domain: Domain) => (
                <Badge color={domain.status === 'active' ? 'green' : 'grey'}>
                  {domain.status}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: 'DNS Verified',
              cell: (domain: Domain) => (
                <Badge color={domain.dns_verified ? 'green' : 'red'}>
                  {domain.dns_verified ? 'Yes' : 'No'}
                </Badge>
              ),
              width: '12%',
            },
            {
              header: 'Users',
              cell: (domain: Domain) => domain.user_count,
              width: '10%',
            },
            {
              header: 'Quota (GB)',
              cell: (domain: Domain) => domain.quota_gb,
              width: '12%',
            },
            {
              header: t('pages.domains.created'),
              cell: (domain: Domain) => new Date(domain.created_at).toLocaleDateString(),
              width: '15%',
            },
            {
              header: 'Actions',
              cell: (domain: Domain) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button
                    variant="inline-link"
                    onClick={() => handleVerifyDNS(domain.id)}
                    disabled={domain.dns_verified}
                  >
                    Verify DNS
                  </Button>
                  <Button variant="inline-link" disabled>Edit</Button>
                </SpaceBetween>
              ),
              width: '14%',
            },
          ]}
          items={filteredDomains}
          header={<Header variant="h2" counter={`(${filteredDomains.length})`}>Domain List</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />

        {/* Create Domain Modal */}
        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowModal(false)}>Cancel</Button>
                <Button variant="primary" onClick={handleCreateDomain}>
                  Create Domain
                </Button>
              </SpaceBetween>
            </Box>
          }
          header="Add New Domain"
        >
          <SpaceBetween size="m">
            <FormField label="Domain Name">
              <Input
                value={newDomain.name}
                onChange={(e) => setNewDomain({ ...newDomain, name: e.detail.value })}
                placeholder="example.com"
              />
            </FormField>
            <FormField label="Quota (GB)">
              <Input
                type="number"
                value={newDomain.quota_gb.toString()}
                onChange={(e) => setNewDomain({ ...newDomain, quota_gb: parseInt(e.detail.value) })}
              />
            </FormField>
          </SpaceBetween>
        </Modal>

        {/* Domain Details Modal */}
        {selectedDomain && (
          <Modal
            onDismiss={() => setShowDetailsModal(false)}
            visible={showDetailsModal}
            size="large"
            header={`Domain Details: ${selectedDomain.name}`}
          >
            <Tabs
              tabs={[
                {
                  label: 'General',
                  id: 'general',
                  content: (
                    <Container header={<Header variant="h3">General Information</Header>}>
                      <KeyValuePairs
                        items={[
                          { label: 'Domain Name', value: selectedDomain.name },
                          { label: 'Status', value: selectedDomain.status },
                          { label: 'Users', value: selectedDomain.user_count },
                          { label: 'Quota (GB)', value: selectedDomain.quota_gb },
                          { label: 'Created', value: new Date(selectedDomain.created_at).toLocaleString() },
                        ]}
                      />
                    </Container>
                  ),
                },
                {
                  label: 'DNS Records',
                  id: 'dns',
                  content: (
                    <Container header={<Header variant="h3">DNS Records Status</Header>}>
                      <KeyValuePairs
                        items={[
                          { label: 'DNS Verified', value: selectedDomain.dns_verified ? 'Yes' : 'No' },
                          { label: 'SPF Status', value: selectedDomain.spf_status || 'Not checked' },
                          { label: 'DKIM Status', value: selectedDomain.dkim_status || 'Not checked' },
                          { label: 'DMARC Status', value: selectedDomain.dmarc_status || 'Not checked' },
                        ]}
                      />
                    </Container>
                  ),
                },
              ]}
            />
          </Modal>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
