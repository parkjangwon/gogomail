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
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

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
  const [companies, setCompanies] = useState<Company[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
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

  const filteredCompanies = companies.filter(c =>
    c.name.toLowerCase().includes(filter.toLowerCase())
  );

  const paginatedCompanies = filteredCompanies.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Companies</Header>}>
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
            <Button variant="primary" disabled>
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
              header: 'Status',
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
              header: 'Created',
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
    </ContentLayout>
  );
}
