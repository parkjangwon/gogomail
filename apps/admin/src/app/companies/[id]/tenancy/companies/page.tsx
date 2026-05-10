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
  Pagination,
  Modal,
  FormField,
  Input,
  Badge,
  StatusIndicator,
  ColumnLayout,
  Container,
  KeyValuePairs,
  Alert,
  ProgressBar,
  Tabs,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

interface Company {
  id: string;
  name: string;
  status: string;
  quota_used: number;
  quota_limit: number;
  quota_remaining: number;
  allocated_domain_quota: number;
  allocatable_domain_quota: number;
  over_allocated: boolean;
  created_at: string;
}

interface DomainSummary {
  id: string;
  name: string;
  status: string;
  last_dns_check_status: string;
  created_at: string;
}

export default function CompaniesPage() {
  const { t } = useI18n();
  const router = useRouter();

  const [companies, setCompanies] = useState<Company[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [currentPage, setCurrentPage] = useState(1);

  // Create modal
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newCompany, setNewCompany] = useState({ name: '', quota_gb: '' });
  const [creating, setCreating] = useState(false);
  const [createdCompany, setCreatedCompany] = useState<Company | null>(null);
  const [showPostCreateGuide, setShowPostCreateGuide] = useState(false);

  // Detail modal
  const [selectedCompany, setSelectedCompany] = useState<Company | null>(null);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [companyDomains, setCompanyDomains] = useState<DomainSummary[]>([]);
  const [loadingDomains, setLoadingDomains] = useState(false);

  const itemsPerPage = 20;

  useEffect(() => { fetchCompanies(); }, []);

  const fetchCompanies = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/companies?limit=200', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setCompanies(data.companies || []);
      }
    } catch (error) {
      console.error('Failed to fetch companies:', error);
    } finally {
      setLoading(false);
    }
  };

  const fetchCompanyDomains = async (companyId: string) => {
    setLoadingDomains(true);
    try {
      const res = await fetch(`/api/admin/domains?company_id=${companyId}&limit=100`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setCompanyDomains(data.domains || []);
      }
    } catch (error) {
      console.error('Failed to fetch domains:', error);
    } finally {
      setLoadingDomains(false);
    }
  };

  const handleViewCompany = (company: Company) => {
    setSelectedCompany(company);
    setShowDetailModal(true);
    fetchCompanyDomains(company.id);
  };

  const handleCreateCompany = async () => {
    if (!newCompany.name.trim()) return;
    setCreating(true);
    try {
      const res = await fetch('/api/admin/companies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newCompany.name,
          quota_limit: newCompany.quota_gb ? parseInt(newCompany.quota_gb) * 1073741824 : 0,
        }),
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setCreatedCompany(data.company);
        setShowCreateModal(false);
        setNewCompany({ name: '', quota_gb: '' });
        setShowPostCreateGuide(true);
        fetchCompanies();
      }
    } catch (error) {
      console.error('Failed to create company:', error);
    } finally {
      setCreating(false);
    }
  };

  const filteredCompanies = companies.filter(c =>
    c.name.toLowerCase().includes(filter.toLowerCase())
  );

  const paginatedCompanies = filteredCompanies.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  );

  const getQuotaPercent = (used: number, limit: number) =>
    limit > 0 ? Math.round((used / limit) * 100) : 0;

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.companies.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.companies.description')}
          counter={`(${companies.length})`}
          actions={
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              + Create Company
            </Button>
          }
        >
          {t('pages.companies.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {showPostCreateGuide && createdCompany && (
          <Alert
            type="success"
            dismissible
            onDismiss={() => setShowPostCreateGuide(false)}
            header={`"${createdCompany.name}" was created successfully`}
            action={
              <Button
                variant="primary"
                onClick={() => {
                  setShowPostCreateGuide(false);
                  router.push(`/companies/default/tenancy/domains`);
                }}
              >
                Add Domain Now
              </Button>
            }
          >
            Next step: add a domain so users can send and receive mail under this company.
          </Alert>
        )}

        <Table
          columnDefinitions={[
            {
              header: 'Company Name',
              cell: (c: Company) => (
                <Button variant="inline-link" onClick={() => handleViewCompany(c)}>
                  {c.name}
                </Button>
              ),
              width: '25%',
            },
            {
              header: t('pages.companies.status'),
              cell: (c: Company) => (
                <Badge color={c.status === 'active' ? 'green' : c.status === 'suspended' ? 'severity-high' : 'grey'}>
                  {c.status}
                </Badge>
              ),
              width: '10%',
            },
            {
              header: 'Storage Quota',
              cell: (c: Company) => {
                const pct = getQuotaPercent(c.quota_used, c.quota_limit);
                return c.quota_limit > 0 ? (
                  <Box>
                    <ProgressBar
                      value={pct}
                      status={c.over_allocated ? 'error' : pct > 80 ? 'in-progress' : 'success'}
                      resultText={`${pct}%`}
                      additionalInfo={`${(c.quota_used / 1073741824).toFixed(1)} / ${(c.quota_limit / 1073741824).toFixed(1)} GB`}
                    />
                  </Box>
                ) : <Box color="text-body-secondary">Unlimited</Box>;
              },
              width: '28%',
            },
            {
              header: t('pages.companies.created'),
              cell: (c: Company) => new Date(c.created_at).toLocaleDateString(),
              width: '15%',
            },
            {
              header: 'Actions',
              cell: (c: Company) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" onClick={() => handleViewCompany(c)}>View</Button>
                  <Button
                    variant="inline-link"
                    onClick={() => router.push(`/companies/default/tenancy/domains`)}
                  >
                    Add Domain
                  </Button>
                </SpaceBetween>
              ),
              width: '22%',
            },
          ]}
          items={paginatedCompanies}
          header={<Header variant="h2" counter={`(${filteredCompanies.length})`}>Company List</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder="Search by company name"
              onChange={(e) => { setFilter(e.detail.filteringText); setCurrentPage(1); }}
            />
          }
          pagination={
            <Pagination
              currentPageIndex={currentPage}
              pagesCount={Math.max(1, Math.ceil(filteredCompanies.length / itemsPerPage))}
              onChange={(e) => setCurrentPage(e.detail.currentPageIndex)}
            />
          }
          empty={
            <Box textAlign="center" padding="l">
              <SpaceBetween size="m" alignItems="center">
                <StatusIndicator type="info">No companies yet</StatusIndicator>
                <Button variant="primary" onClick={() => setShowCreateModal(true)}>Create your first company</Button>
              </SpaceBetween>
            </Box>
          }
        />
      </SpaceBetween>

      {/* Create Company Modal */}
      <Modal
        onDismiss={() => setShowCreateModal(false)}
        visible={showCreateModal}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowCreateModal(false)}>Cancel</Button>
              <Button
                variant="primary"
                onClick={handleCreateCompany}
                loading={creating}
                disabled={!newCompany.name.trim()}
              >
                Create Company
              </Button>
            </SpaceBetween>
          </Box>
        }
        header="Create New Company"
      >
        <SpaceBetween size="m">
          <FormField label="Company Name" constraintText="Must be unique">
            <Input
              value={newCompany.name}
              onChange={(e) => setNewCompany({ ...newCompany, name: e.detail.value })}
              placeholder="e.g., ACME Corp"
              autoFocus
            />
          </FormField>
          <FormField
            label="Storage Quota (GB)"
            description="Leave empty for unlimited"
          >
            <Input
              type="number"
              value={newCompany.quota_gb}
              onChange={(e) => setNewCompany({ ...newCompany, quota_gb: e.detail.value })}
              placeholder="e.g., 500"
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      {/* Company Detail Modal */}
      {selectedCompany && (
        <Modal
          onDismiss={() => setShowDetailModal(false)}
          visible={showDetailModal}
          size="large"
          header={
            <SpaceBetween direction="horizontal" size="s">
              <Box fontWeight="bold" fontSize="heading-m">{selectedCompany.name}</Box>
              <Badge color={selectedCompany.status === 'active' ? 'green' : 'grey'}>{selectedCompany.status}</Badge>
            </SpaceBetween>
          }
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button
                  onClick={() => {
                    setShowDetailModal(false);
                    router.push(`/companies/default/tenancy/domains`);
                  }}
                >
                  Add Domain
                </Button>
                <Button variant="primary" onClick={() => setShowDetailModal(false)}>Close</Button>
              </SpaceBetween>
            </Box>
          }
        >
          <Tabs
            tabs={[
              {
                label: 'Overview',
                id: 'overview',
                content: (
                  <SpaceBetween size="m">
                    <ColumnLayout columns={2}>
                      <Container header={<Header variant="h3">Company Info</Header>}>
                        <KeyValuePairs
                          items={[
                            { label: 'Company ID', value: <Box fontSize="body-s" color="text-body-secondary">{selectedCompany.id}</Box> },
                            { label: 'Status', value: <Badge color={selectedCompany.status === 'active' ? 'green' : 'grey'}>{selectedCompany.status}</Badge> },
                            { label: 'Created', value: new Date(selectedCompany.created_at).toLocaleString() },
                          ]}
                        />
                      </Container>
                      <Container header={<Header variant="h3">Storage</Header>}>
                        <KeyValuePairs
                          items={[
                            { label: 'Used', value: `${(selectedCompany.quota_used / 1073741824).toFixed(2)} GB` },
                            { label: 'Limit', value: selectedCompany.quota_limit > 0 ? `${(selectedCompany.quota_limit / 1073741824).toFixed(2)} GB` : 'Unlimited' },
                            { label: 'Remaining', value: selectedCompany.quota_limit > 0 ? `${(selectedCompany.quota_remaining / 1073741824).toFixed(2)} GB` : '—' },
                            {
                              label: 'Utilization',
                              value: selectedCompany.quota_limit > 0
                                ? <ProgressBar value={getQuotaPercent(selectedCompany.quota_used, selectedCompany.quota_limit)} resultText={`${getQuotaPercent(selectedCompany.quota_used, selectedCompany.quota_limit)}%`} />
                                : '—'
                            },
                          ]}
                        />
                      </Container>
                    </ColumnLayout>
                  </SpaceBetween>
                ),
              },
              {
                label: `Domains (${companyDomains.length})`,
                id: 'domains',
                content: loadingDomains ? (
                  <Box textAlign="center" padding="l"><Spinner /></Box>
                ) : (
                  <SpaceBetween size="m">
                    {companyDomains.length === 0 ? (
                      <Box textAlign="center" padding="l">
                        <SpaceBetween size="m" alignItems="center">
                          <StatusIndicator type="warning">No domains configured</StatusIndicator>
                          <Box color="text-body-secondary">Add a domain to enable mail services for this company.</Box>
                          <Button
                            variant="primary"
                            onClick={() => {
                              setShowDetailModal(false);
                              router.push(`/companies/default/tenancy/domains`);
                            }}
                          >
                            + Add Domain
                          </Button>
                        </SpaceBetween>
                      </Box>
                    ) : (
                      <Table
                        columnDefinitions={[
                          {
                            header: 'Domain',
                            cell: (d: DomainSummary) => d.name,
                            width: '40%',
                          },
                          {
                            header: 'Status',
                            cell: (d: DomainSummary) => (
                              <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>
                            ),
                            width: '20%',
                          },
                          {
                            header: 'DNS',
                            cell: (d: DomainSummary) => (
                              <Badge color={d.last_dns_check_status === 'pass' ? 'green' : d.last_dns_check_status === 'fail' ? 'red' : 'grey'}>
                                {d.last_dns_check_status || 'Unchecked'}
                              </Badge>
                            ),
                            width: '20%',
                          },
                          {
                            header: 'Added',
                            cell: (d: DomainSummary) => new Date(d.created_at).toLocaleDateString(),
                            width: '20%',
                          },
                        ]}
                        items={companyDomains}
                        header={<Header variant="h3">Domains under {selectedCompany.name}</Header>}
                      />
                    )}
                  </SpaceBetween>
                ),
              },
            ]}
          />
        </Modal>
      )}
    </ContentLayout>
  );
}
