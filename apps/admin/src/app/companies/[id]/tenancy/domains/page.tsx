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
  Select,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useParams, useRouter } from 'next/navigation';

interface Domain {
  id: string;
  company_id: string;
  company_name: string;
  name: string;
  name_ace: string;
  status: string;
  last_dns_check_status: string;
  quota_used: number;
  quota_limit: number;
  quota_remaining: number;
  allocated_user_quota: number;
  allocatable_user_quota: number;
  created_at: string;
}

interface Company {
  id: string;
  name: string;
}

export default function DomainsPage() {
  const { t } = useI18n();
  const params = useParams();
  const router = useRouter();
  const companyId = params?.id as string;

  const [domains, setDomains] = useState<Domain[]>([]);
  const [companies, setCompanies] = useState<Company[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [filterCompany, setFilterCompany] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [newDomain, setNewDomain] = useState({ name: '', company_id: companyId === 'default' ? '' : companyId, quota_gb: '100' });
  const [creating, setCreating] = useState(false);
  const [verifying, setVerifying] = useState<string | null>(null);

  useEffect(() => {
    fetchDomains();
    fetchCompanies();
  }, []);

  const fetchDomains = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/domains?limit=200', { credentials: 'include' });
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

  const fetchCompanies = async () => {
    try {
      const res = await fetch('/api/admin/companies?limit=200', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setCompanies(data.companies || []);
      }
    } catch (error) {
      console.error('Failed to fetch companies:', error);
    }
  };

  const handleCreateDomain = async () => {
    if (!newDomain.name.trim() || !newDomain.company_id) return;
    setCreating(true);
    try {
      const res = await fetch('/api/admin/domains', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newDomain.name,
          company_id: newDomain.company_id,
          quota_gb: parseInt(newDomain.quota_gb),
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowModal(false);
        setNewDomain({ name: '', company_id: '', quota_gb: '100' });
        fetchDomains();
      }
    } catch (error) {
      console.error('Failed to create domain:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleVerifyDNS = async (domainId: string) => {
    setVerifying(domainId);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/verify-dns`, {
        method: 'POST',
        credentials: 'include',
      });
      if (res.ok) fetchDomains();
    } catch (error) {
      console.error('Failed to verify DNS:', error);
    } finally {
      setVerifying(null);
    }
  };

  const getDNSBadge = (status: string) => {
    switch (status) {
      case 'pass': return <Badge color="green">Pass</Badge>;
      case 'fail': return <Badge color="red">Fail</Badge>;
      case 'partial': return <Badge color="severity-high">Partial</Badge>;
      default: return <Badge color="grey">{t('pages.tenancy_domains.unchecked') || 'Unchecked'}</Badge>;
    }
  };

  const filteredDomains = domains.filter(d => {
    const matchesName = d.name.toLowerCase().includes(filter.toLowerCase());
    const matchesCompany = !filterCompany || d.company_id === filterCompany;
    return matchesName && matchesCompany;
  });

  const companyOptions = [
    { label: t('pages.tenancy_domains.all_companies'), value: '' },
    ...companies.map(c => ({ label: c.name, value: c.id })),
  ];

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.domains.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.tenancy_domains.description')}
          counter={`(${domains.length})`}
          actions={
            <Button variant="primary" onClick={() => setShowModal(true)}>
              {t('pages.domains.create_domain')}
            </Button>
          }
        >
          {t('pages.domains.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: t('pages.tenancy_domains.domain'),
              cell: (d: Domain) => (
                <Button variant="inline-link" onClick={() => router.push(`/companies/${d.company_id}/domains/${d.id}`)}>
                  {d.name}
                </Button>
              ),
              width: '22%',
            },
            {
              header: t('pages.tenancy_domains.company'),
              cell: (d: Domain) => (
                <Box>
                  <Box fontWeight="bold" fontSize="body-s">{d.company_name || '—'}</Box>
                  <Box color="text-body-secondary" fontSize="body-s">{d.company_id}</Box>
                </Box>
              ),
              width: '20%',
            },
            {
              header: t('pages.domains.status'),
              cell: (d: Domain) => (
                <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>
              ),
              width: '10%',
            },
            {
              header: t('pages.tenancy_domains.dns'),
              cell: (d: Domain) => getDNSBadge(d.last_dns_check_status),
              width: '10%',
            },
            {
              header: t('pages.tenancy_domains.quota_used'),
              cell: (d: Domain) => {
                const limit = d.quota_limit ?? 0;
                const used = d.quota_used ?? 0;
                const pct = limit > 0 ? Math.round((used / limit) * 100) : 0;
                return (
                  <Box>
                    <Box>
                      {limit > 0
                        ? `${(used / 1073741824).toFixed(1)} / ${(limit / 1073741824).toFixed(1)} GB`
                        : `${(used / 1073741824).toFixed(1)} GB (${t('pages.tenancy_domains.unlimited')})`}
                    </Box>
                    {limit > 0 && (
                      <Box color={pct > 80 ? 'text-status-error' : 'text-body-secondary'} fontSize="body-s">{pct}%</Box>
                    )}
                  </Box>
                );
              },
              width: '18%',
            },
            {
              header: t('pages.domains.created'),
              cell: (d: Domain) => new Date(d.created_at).toLocaleDateString(),
              width: '10%',
            },
            {
              header: t('pages.tenancy_domains.actions'),
              cell: (d: Domain) => (
                <Button
                  variant="inline-link"
                  onClick={() => handleVerifyDNS(d.id)}
                  loading={verifying === d.id}
                  disabled={d.last_dns_check_status === 'pass'}
                >
                  {t('pages.tenancy_domains.verify_dns')}
                </Button>
              ),
              width: '10%',
            },
          ]}
          items={filteredDomains}
          header={<Header variant="h2" counter={`(${filteredDomains.length})`}>{t('pages.tenancy_domains.domain_list')}</Header>}
          filter={
            <SpaceBetween direction="horizontal" size="xs">
              <TextFilter
                filteringText={filter}
                filteringPlaceholder={t('pages.tenancy_domains.search_by_domain')}
                onChange={(e) => setFilter(e.detail.filteringText)}
              />
              <Select
                selectedOption={companyOptions.find(o => o.value === filterCompany) ?? companyOptions[0]}
                options={companyOptions}
                onChange={(e) => setFilterCompany(e.detail.selectedOption.value ?? '')}
                placeholder={t('pages.tenancy_domains.filter_by_company')}
              />
            </SpaceBetween>
          }
          empty={
            <Box textAlign="center" padding="l">
              <StatusIndicator type="info">{t('pages.tenancy_domains.no_domains')}</StatusIndicator>
            </Box>
          }
        />

        {/* Create Domain Modal */}
        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          size="medium"
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
                <Button
                  variant="primary"
                  onClick={handleCreateDomain}
                  loading={creating}
                  disabled={!newDomain.name.trim() || !newDomain.company_id}
                >
                  {t('pages.tenancy_domains.add_domain_btn')}
                </Button>
              </SpaceBetween>
            </Box>
          }
          header={t('pages.tenancy_domains.add_new_domain')}
        >
          <SpaceBetween size="m">
            <FormField
              label={t('pages.tenancy_domains.company')}
              description={t('pages.tenancy_domains.filter_by_company')}
            >
              <Select
                selectedOption={
                  companies.find(c => c.id === newDomain.company_id)
                    ? { label: companies.find(c => c.id === newDomain.company_id)!.name, value: newDomain.company_id }
                    : null
                }
                options={companies.map(c => ({ label: c.name, value: c.id }))}
                onChange={(e) => setNewDomain({ ...newDomain, company_id: e.detail.selectedOption.value ?? '' })}
                placeholder={t('pages.tenancy_domains.select_company')}
                empty={t('pages.tenancy_domains.no_companies')}
              />
            </FormField>
            <FormField label={t('pages.domains.domain_name')}>
              <Input
                value={newDomain.name}
                onChange={(e) => setNewDomain({ ...newDomain, name: e.detail.value })}
                placeholder="example.com"
              />
            </FormField>
            <FormField label={t('pages.tenancy_domains.storage_quota_gb')}>
              <Input
                type="number"
                value={newDomain.quota_gb}
                onChange={(e) => setNewDomain({ ...newDomain, quota_gb: e.detail.value })}
              />
            </FormField>
          </SpaceBetween>
        </Modal>

      </SpaceBetween>
    </ContentLayout>
  );
}
