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
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface Company {
  id: string;
  name: string;
  status: string;
  quota_gb: number;
  users_count: number;
  domains_count: number;
  created_at: string;
}

export default function CompaniesPage() {
  const { t } = useI18n();
  const [companies, setCompanies] = useState<Company[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
  const [showModal, setShowModal] = useState(false);
  const [newCompany, setNewCompany] = useState({ name: '', quota_gb: '' });
  const [creating, setCreating] = useState(false);
  const itemsPerPage = 20;

  useEffect(() => {
    fetchCompanies();
  }, []);

  const fetchCompanies = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/companies?limit=100', {
        credentials: 'include'
      });
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

  const handleCreateCompany = async () => {
    if (!newCompany.name.trim()) {
      return;
    }
    setCreating(true);
    try {
      const res = await fetch('/api/admin/companies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newCompany.name,
          quota_limit: parseInt(newCompany.quota_gb) * 1024 * 1024 * 1024,
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowModal(false);
        setNewCompany({ name: '', quota_gb: '' });
        fetchCompanies();
      } else {
        console.error('Failed to create company');
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

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.companies.title')}</Header>}>
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
          description="Manage tenants and organizations"
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              + Create Company
            </Button>
          }
        >
          Companies
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Name',
              cell: (company: Company) => company.name,
              width: '25%',
            },
            {
              header: t('pages.companies.status'),
              cell: (company: Company) => company.status,
              width: '15%',
            },
            {
              header: 'Users',
              cell: (company: Company) => company.users_count,
              width: '10%',
            },
            {
              header: 'Domains',
              cell: (company: Company) => company.domains_count,
              width: '10%',
            },
            {
              header: 'Quota (GB)',
              cell: (company: Company) => company.quota_gb,
              width: '12%',
            },
            {
              header: t('pages.companies.created'),
              cell: (company: Company) => new Date(company.created_at).toLocaleDateString(),
              width: '15%',
            },
            {
              header: 'Actions',
              cell: () => (
                <Button variant="inline-link" disabled>View</Button>
              ),
              width: '13%',
            },
          ]}
          items={paginatedCompanies}
          header={<Header variant="h2" counter={`(${filteredCompanies.length})`}>Company List</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              onChange={(e) => {
                setFilter(e.detail.filteringText);
                setCurrentPage(1);
              }}
            />
          }
          pagination={
            <Pagination
              currentPageIndex={currentPage}
              pagesCount={Math.ceil(filteredCompanies.length / itemsPerPage)}
              onChange={(e) => setCurrentPage(e.detail.currentPageIndex)}
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
              <Button variant="primary" onClick={handleCreateCompany} loading={creating}>
                Create Company
              </Button>
            </SpaceBetween>
          </Box>
        }
        header="Create New Company"
      >
        <SpaceBetween size="m">
          <FormField label="Company Name">
            <Input
              value={newCompany.name}
              onChange={(e) => setNewCompany({ ...newCompany, name: e.detail.value })}
              placeholder="e.g., ACME Corp"
            />
          </FormField>
          <FormField label="Quota (GB)">
            <Input
              type="number"
              value={newCompany.quota_gb}
              onChange={(e) => setNewCompany({ ...newCompany, quota_gb: e.detail.value })}
              placeholder="e.g., 100"
            />
          </FormField>
        </SpaceBetween>
      </Modal>
    </ContentLayout>
  );
}
