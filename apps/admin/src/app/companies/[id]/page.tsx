'use client';

import {
  ContentLayout,
  Header,
  ColumnLayout,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  Badge,
  Button,
  ProgressBar,
  KeyValuePairs,
  Table,
  StatusIndicator,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useCompany } from '@/contexts/CompanyContext';

interface Domain {
  id: string;
  name: string;
  status: string;
  last_dns_check_status: string;
  quota_used: number;
  quota_limit: number;
  created_at: string;
}

export default function CompanyOverviewPage() {
  const params = useParams();
  const router = useRouter();
  const companyId = params?.id as string;
  const { companies, currentCompany } = useCompany();

  const [domains, setDomains] = useState<Domain[]>([]);
  const [loadingDomains, setLoadingDomains] = useState(true);

  const company =
    companies.find(c => c.id === companyId) ??
    (companyId === 'default' ? currentCompany : null);

  useEffect(() => {
    if (!company) return;
    setLoadingDomains(true);
    fetch(`/api/admin/domains?company_id=${company.id}&limit=100`, { credentials: 'include' })
      .then(r => r.ok ? r.json() : { domains: [] })
      .then(d => setDomains(d.domains || []))
      .catch(() => {})
      .finally(() => setLoadingDomains(false));
  }, [company]);

  if (!company) {
    return (
      <ContentLayout header={<Header variant="h1">Company Overview</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const quotaPct = company.quota_limit > 0
    ? Math.round((company.quota_used / company.quota_limit) * 100)
    : 0;
  const dnsIssues = domains.filter(d => d.last_dns_check_status !== 'pass' && d.last_dns_check_status !== '');
  const activeDomains = domains.filter(d => d.status === 'active');

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={`ID: ${company.id}`}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                Manage Domains
              </Button>
              <Button onClick={() => router.push(`/companies/${companyId}/users`)}>
                Manage Users
              </Button>
              <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/dashboard`)}>
                Dashboard →
              </Button>
            </SpaceBetween>
          }
        >
          <SpaceBetween direction="horizontal" size="xs">
            <span>{company.name}</span>
            <Badge color={company.status === 'active' ? 'green' : 'severity-high'}>{company.status}</Badge>
          </SpaceBetween>
        </Header>
      }
    >
      <SpaceBetween size="l">
        {dnsIssues.length > 0 && (
          <Alert
            type="warning"
            header={`${dnsIssues.length} domain(s) have DNS issues`}
            action={
              <Button onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                Review Domains
              </Button>
            }
          >
            {dnsIssues.map(d => d.name).join(', ')}
          </Alert>
        )}

        <ColumnLayout columns={3}>
          <Container header={<Header variant="h3">Storage</Header>}>
            <SpaceBetween size="s">
              <ProgressBar
                value={quotaPct}
                status={company.over_allocated ? 'error' : quotaPct > 80 ? 'in-progress' : 'success'}
                resultText={`${quotaPct}%`}
                additionalInfo={
                  company.quota_limit > 0
                    ? `${(company.quota_used / 1073741824).toFixed(1)} / ${(company.quota_limit / 1073741824).toFixed(1)} GB`
                    : `${(company.quota_used / 1073741824).toFixed(1)} GB (unlimited)`
                }
              />
              {company.over_allocated && (
                <StatusIndicator type="error">Over allocated</StatusIndicator>
              )}
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">Domains</Header>}>
            <SpaceBetween size="s">
              <Box fontSize="display-l" fontWeight="bold">{domains.length}</Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {activeDomains.length} active
                {dnsIssues.length > 0 && ` · ${dnsIssues.length} DNS issues`}
              </Box>
              <Button variant="inline-link" onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                View all →
              </Button>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">Info</Header>}>
            <KeyValuePairs
              items={[
                { label: 'Status', value: <Badge color={company.status === 'active' ? 'green' : 'grey'}>{company.status}</Badge> },
                { label: 'Created', value: new Date(company.created_at).toLocaleDateString() },
              ]}
            />
          </Container>
        </ColumnLayout>

        <Table
          loading={loadingDomains}
          loadingText="Loading domains..."
          columnDefinitions={[
            {
              header: 'Domain',
              cell: (d: Domain) => (
                <Button
                  variant="inline-link"
                  onClick={() => router.push(`/companies/${companyId}/domains/${d.id}`)}
                >
                  {d.name}
                </Button>
              ),
              width: '30%',
            },
            {
              header: 'Status',
              cell: (d: Domain) => (
                <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>
              ),
              width: '15%',
            },
            {
              header: 'DNS',
              cell: (d: Domain) => {
                const s = d.last_dns_check_status;
                return <Badge color={s === 'pass' ? 'green' : s === 'fail' ? 'red' : 'grey'}>{s || 'Unchecked'}</Badge>;
              },
              width: '15%',
            },
            {
              header: 'Storage',
              cell: (d: Domain) => {
                const limit = d.quota_limit ?? 0;
                const used = d.quota_used ?? 0;
                return limit > 0
                  ? `${(used / 1073741824).toFixed(1)} / ${(limit / 1073741824).toFixed(1)} GB`
                  : `${(used / 1073741824).toFixed(1)} GB`;
              },
              width: '25%',
            },
            {
              header: 'Added',
              cell: (d: Domain) => new Date(d.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={domains}
          header={
            <Header
              variant="h2"
              counter={`(${domains.length})`}
              actions={
                <Button onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                  + Add Domain
                </Button>
              }
            >
              Domains under {company.name}
            </Header>
          }
          empty={
            <Box textAlign="center" padding="l">
              <SpaceBetween size="m" alignItems="center">
                <StatusIndicator type="warning">No domains configured</StatusIndicator>
                <Box color="text-body-secondary">Add a domain to enable mail for this company.</Box>
                <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                  + Add First Domain
                </Button>
              </SpaceBetween>
            </Box>
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
