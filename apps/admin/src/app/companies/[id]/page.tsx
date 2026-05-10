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
import { useI18n } from '@/app/i18n-provider';

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
  const { t } = useI18n();
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
      <ContentLayout header={<Header variant="h1">{t('pages.company_overview.title')}</Header>}>
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
                {t('pages.company_overview.manage_domains')}
              </Button>
              <Button onClick={() => router.push(`/companies/${companyId}/users`)}>
                {t('pages.company_overview.manage_users')}
              </Button>
              <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/dashboard`)}>
                {t('pages.company_overview.dashboard_btn')} →
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
            header={`${dnsIssues.length} ${t('pages.company_overview.dns_issues_header')}`}
            action={
              <Button onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                {t('pages.company_overview.review_domains')}
              </Button>
            }
          >
            {dnsIssues.map(d => d.name).join(', ')}
          </Alert>
        )}

        <ColumnLayout columns={3}>
          <Container header={<Header variant="h3">{t('pages.company_overview.storage')}</Header>}>
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
                <StatusIndicator type="error">{t('pages.company_overview.over_allocated')}</StatusIndicator>
              )}
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">{t('pages.company_overview.domains')}</Header>}>
            <SpaceBetween size="s">
              <Box fontSize="display-l" fontWeight="bold">{domains.length}</Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {activeDomains.length} {t('pages.company_overview.active')}
                {dnsIssues.length > 0 && ` · ${dnsIssues.length} ${t('pages.company_overview.dns_issues')}`}
              </Box>
              <Button variant="inline-link" onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                {t('pages.company_overview.view_all')} →
              </Button>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">{t('pages.company_overview.info')}</Header>}>
            <KeyValuePairs
              items={[
                { label: t('pages.company_overview.status'), value: <Badge color={company.status === 'active' ? 'green' : 'grey'}>{company.status}</Badge> },
                { label: t('pages.company_overview.created'), value: new Date(company.created_at).toLocaleDateString() },
              ]}
            />
          </Container>
        </ColumnLayout>

        <Table
          loading={loadingDomains}
          loadingText={t('pages.company_overview.loading_domains')}
          columnDefinitions={[
            {
              header: t('pages.company_overview.domain'),
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
              header: t('pages.company_overview.status'),
              cell: (d: Domain) => (
                <Badge color={d.status === 'active' ? 'green' : 'grey'}>{d.status}</Badge>
              ),
              width: '15%',
            },
            {
              header: t('pages.company_overview.dns'),
              cell: (d: Domain) => {
                const s = d.last_dns_check_status;
                return <Badge color={s === 'pass' ? 'green' : s === 'fail' ? 'red' : 'grey'}>{s || t('pages.company_overview.unchecked')}</Badge>;
              },
              width: '15%',
            },
            {
              header: t('pages.company_overview.storage_col'),
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
              header: t('pages.company_overview.added'),
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
                  {t('pages.company_overview.add_domain_btn')}
                </Button>
              }
            >
              {t('pages.company_overview.domains_under')} {company.name}
            </Header>
          }
          empty={
            <Box textAlign="center" padding="l">
              <SpaceBetween size="m" alignItems="center">
                <StatusIndicator type="warning">{t('pages.company_overview.no_domains')}</StatusIndicator>
                <Box color="text-body-secondary">{t('pages.company_overview.no_domains_desc')}</Box>
                <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                  {t('pages.company_overview.add_first_domain')}
                </Button>
              </SpaceBetween>
            </Box>
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
