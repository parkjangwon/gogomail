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
import { useParams } from 'next/navigation';

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
  const params = useParams();
  const cid = params?.id as string;

  const [companies, setCompanies] = useState<Company[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [currentPage, setCurrentPage] = useState(1);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newCompany, setNewCompany] = useState({ name: '', quota_gb: '' });
  const [creating, setCreating] = useState(false);
  const [createdCompany, setCreatedCompany] = useState<Company | null>(null);
  const [showPostCreateGuide, setShowPostCreateGuide] = useState(false);

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
              {t('pages.companies.create_company')}
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
            header={`"${createdCompany.name}"`}
            action={
              <Button
                variant="primary"
                onClick={() => {
                  setShowPostCreateGuide(false);
                  router.push(`/companies/${cid}/tenancy/domains`);
                }}
              >
                {t('pages.companies.add_domain_now')}
              </Button>
            }
          >
            {t('pages.companies.created_next_step')}
          </Alert>
        )}

        <Table
          columnDefinitions={[
            {
              header: t('pages.companies.company_name'),
              cell: (c: Company) => (
                <Button variant="inline-link" onClick={() => router.push(`/companies/${c.id}`)}>
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
              header: t('pages.companies.storage_quota'),
              cell: (c: Company) => {
                const limit = c.quota_limit ?? 0;
                const used = c.quota_used ?? 0;
                const pct = getQuotaPercent(used, limit);
                return limit > 0 ? (
                  <ProgressBar
                    value={pct}
                    status={c.over_allocated ? 'error' : pct > 80 ? 'in-progress' : 'success'}
                    resultText={`${pct}%`}
                    additionalInfo={`${(used / 1073741824).toFixed(1)} / ${(limit / 1073741824).toFixed(1)} GB`}
                  />
                ) : <Box color="text-body-secondary">{t('pages.companies.unlimited')}</Box>;
              },
              width: '30%',
            },
            {
              header: t('pages.companies.created'),
              cell: (c: Company) => new Date(c.created_at).toLocaleDateString(),
              width: '15%',
            },
            {
              header: t('pages.companies.actions'),
              cell: (c: Company) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" onClick={() => handleViewCompany(c)}>
                    {t('pages.companies.view')}
                  </Button>
                  <Button
                    variant="inline-link"
                    onClick={() => router.push(`/companies/${c.id}/tenancy/domains`)}
                  >
                    {t('pages.companies.add_domain')}
                  </Button>
                </SpaceBetween>
              ),
              width: '20%',
            },
          ]}
          items={paginatedCompanies}
          header={
            <Header variant="h2" counter={`(${filteredCompanies.length})`}>
              {t('pages.companies.company_list')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('pages.companies.search_placeholder')}
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
                <StatusIndicator type="info">{t('pages.companies.no_companies')}</StatusIndicator>
                <Button variant="primary" onClick={() => setShowCreateModal(true)}>
                  {t('pages.companies.create_first')}
                </Button>
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
              <Button onClick={() => setShowCreateModal(false)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={handleCreateCompany}
                loading={creating}
                disabled={!newCompany.name.trim()}
              >
                {t('pages.companies.create_company_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.companies.create_modal_title')}
      >
        <SpaceBetween size="m">
          <FormField
            label={t('pages.companies.company_name')}
            constraintText={t('pages.companies.name_constraint')}
          >
            <Input
              value={newCompany.name}
              onChange={(e) => setNewCompany({ ...newCompany, name: e.detail.value })}
              placeholder={t('pages.companies.name_placeholder')}
              autoFocus
            />
          </FormField>
          <FormField
            label={t('pages.companies.quota_label')}
            description={t('pages.companies.quota_desc')}
          >
            <Input
              type="number"
              value={newCompany.quota_gb}
              onChange={(e) => setNewCompany({ ...newCompany, quota_gb: e.detail.value })}
              placeholder={t('pages.companies.quota_placeholder')}
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
                    router.push(`/companies/${selectedCompany.id}/tenancy/domains`);
                  }}
                >
                  {t('pages.companies.add_domain')}
                </Button>
                <Button variant="primary" onClick={() => setShowDetailModal(false)}>
                  {t('pages.companies.close')}
                </Button>
              </SpaceBetween>
            </Box>
          }
        >
          <Tabs
            tabs={[
              {
                label: t('pages.companies.overview_tab'),
                id: 'overview',
                content: (
                  <SpaceBetween size="m">
                    <ColumnLayout columns={2}>
                      <Container header={<Header variant="h3">{t('pages.companies.company_info')}</Header>}>
                        <KeyValuePairs
                          items={[
                            { label: t('pages.companies.company_id_label'), value: <Box fontSize="body-s" color="text-body-secondary">{selectedCompany.id}</Box> },
                            { label: t('pages.companies.status'), value: <Badge color={selectedCompany.status === 'active' ? 'green' : 'grey'}>{selectedCompany.status}</Badge> },
                            { label: t('pages.companies.created'), value: new Date(selectedCompany.created_at).toLocaleString() },
                          ]}
                        />
                      </Container>
                      <Container header={<Header variant="h3">{t('pages.companies.storage')}</Header>}>
                        <KeyValuePairs
                          items={[
                            { label: t('pages.companies.used'), value: `${((selectedCompany.quota_used ?? 0) / 1073741824).toFixed(2)} GB` },
                            { label: t('pages.companies.limit'), value: (selectedCompany.quota_limit ?? 0) > 0 ? `${(selectedCompany.quota_limit / 1073741824).toFixed(2)} GB` : t('pages.companies.unlimited') },
                            { label: t('pages.companies.remaining'), value: (selectedCompany.quota_limit ?? 0) > 0 ? `${((selectedCompany.quota_remaining ?? 0) / 1073741824).toFixed(2)} GB` : '—' },
                            {
                              label: t('pages.companies.utilization'),
                              value: (selectedCompany.quota_limit ?? 0) > 0
                                ? <ProgressBar value={getQuotaPercent(selectedCompany.quota_used, selectedCompany.quota_limit)} resultText={`${getQuotaPercent(selectedCompany.quota_used, selectedCompany.quota_limit)}%`} />
                                : '—',
                            },
                          ]}
                        />
                      </Container>
                    </ColumnLayout>
                  </SpaceBetween>
                ),
              },
              {
                label: `${t('pages.companies.domains_tab')} (${companyDomains.length})`,
                id: 'domains',
                content: loadingDomains ? (
                  <Box textAlign="center" padding="l"><Spinner /></Box>
                ) : companyDomains.length === 0 ? (
                  <Box textAlign="center" padding="l">
                    <SpaceBetween size="m" alignItems="center">
                      <StatusIndicator type="warning">{t('pages.companies.no_domains')}</StatusIndicator>
                      <Box color="text-body-secondary">{t('pages.companies.no_domains_desc')}</Box>
                      <Button
                        variant="primary"
                        onClick={() => {
                          setShowDetailModal(false);
                          router.push(`/companies/${selectedCompany.id}/tenancy/domains`);
                        }}
                      >
                        {t('pages.companies.add_domain')}
                      </Button>
                    </SpaceBetween>
                  </Box>
                ) : (
                  <Table
                    columnDefinitions={[
                      {
                        header: t('pages.companies.domain'),
                        cell: (d: DomainSummary) => (
                          <Button variant="inline-link" onClick={() => {
                            setShowDetailModal(false);
                            router.push(`/companies/${selectedCompany.id}/domains/${d.id}`);
                          }}>
                            {d.name}
                          </Button>
                        ),
                        width: '40%',
                      },
                      {
                        header: t('pages.companies.status'),
                        cell: (d: DomainSummary) => (
                          <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>
                        ),
                        width: '20%',
                      },
                      {
                        header: t('pages.companies.dns'),
                        cell: (d: DomainSummary) => (
                          <Badge color={d.last_dns_check_status === 'pass' ? 'green' : d.last_dns_check_status === 'fail' ? 'red' : 'grey'}>
                            {d.last_dns_check_status || 'Unchecked'}
                          </Badge>
                        ),
                        width: '20%',
                      },
                      {
                        header: t('pages.companies.added'),
                        cell: (d: DomainSummary) => new Date(d.created_at).toLocaleDateString(),
                        width: '20%',
                      },
                    ]}
                    items={companyDomains}
                    header={<Header variant="h3">{selectedCompany.name}</Header>}
                  />
                ),
              },
            ]}
          />
        </Modal>
      )}
    </ContentLayout>
  );
}
