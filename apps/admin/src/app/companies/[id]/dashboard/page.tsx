'use client';

import {
  ContentLayout,
  Header,
  Box,
  Spinner,
  SpaceBetween,
  Container,
  ColumnLayout,
  ProgressBar,
  KeyValuePairs,
  Button,
  StatusIndicator,
  Badge,
} from '@cloudscape-design/components';
import { useParams, useRouter } from 'next/navigation';
import { useDashboard } from '@/hooks/useDashboard';
import { useCompany } from '@/contexts/CompanyContext';

export default function DashboardPage() {
  const params = useParams();
  const router = useRouter();
  const companyId = params.id as string;
  const { currentCompany } = useCompany();
  const { data, isLoading } = useDashboard(companyId);

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">Dashboard</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  const stats = data?.stats ?? {
    total_users: 0,
    active_domains: 0,
    total_storage_used: 0,
    total_storage_limit: 0,
    over_allocated: false,
  };

  const storageUsed = stats.total_storage_used;
  const storageLimit = stats.total_storage_limit;
  const storageUsedGb = (storageUsed / 1073741824).toFixed(1);
  const storageLimitGb = storageLimit > 0 ? (storageLimit / 1073741824).toFixed(1) : null;
  const storagePct = storageLimit > 0 ? Math.round((storageUsed / storageLimit) * 100) : 0;

  const company = currentCompany;

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={company ? `${company.name} — ${company.status}` : 'System overview'}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => router.push(`/companies/${companyId}/tenancy/domains`)}>
                Domains
              </Button>
              <Button onClick={() => router.push(`/companies/${companyId}/users`)}>
                Users
              </Button>
              <Button variant="primary" onClick={() => router.push(`/companies/${companyId}`)}>
                Company Overview
              </Button>
            </SpaceBetween>
          }
        >
          Dashboard
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* KPI row */}
        <ColumnLayout columns={4} variant="text-grid">
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{stats.total_users}</Box>
              <Box color="text-body-secondary" fontSize="body-s">Total Users</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">{stats.active_domains}</Box>
              <Box color="text-body-secondary" fontSize="body-s">Active Domains</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">
                {storageLimitGb ? `${storageUsedGb} GB` : `${storageUsedGb} GB`}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {storageLimitGb ? `of ${storageLimitGb} GB used` : 'Storage Used (unlimited)'}
              </Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xs">
              <Box fontSize="display-l" fontWeight="bold">
                <Badge color={company?.status === 'active' ? 'green' : 'grey'}>
                  {company?.status ?? '—'}
                </Badge>
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">Company Status</Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        <ColumnLayout columns={2}>
          {/* Storage */}
          <Container header={<Header variant="h3">Storage Quota</Header>}>
            <SpaceBetween size="m">
              {storageLimit > 0 ? (
                <>
                  <ProgressBar
                    value={storagePct}
                    status={stats.over_allocated ? 'error' : storagePct > 80 ? 'in-progress' : 'success'}
                    resultText={`${storagePct}%`}
                    additionalInfo={`${storageUsedGb} / ${storageLimitGb} GB`}
                  />
                  {stats.over_allocated && (
                    <StatusIndicator type="error">Over allocated — reduce usage or increase quota</StatusIndicator>
                  )}
                </>
              ) : (
                <SpaceBetween size="s">
                  <StatusIndicator type="success">Unlimited storage</StatusIndicator>
                  <Box color="text-body-secondary" fontSize="body-s">{storageUsedGb} GB used</Box>
                </SpaceBetween>
              )}
              <KeyValuePairs
                items={[
                  { label: 'Used', value: `${storageUsedGb} GB` },
                  { label: 'Limit', value: storageLimitGb ? `${storageLimitGb} GB` : 'Unlimited' },
                ]}
              />
            </SpaceBetween>
          </Container>

          {/* Quick Links */}
          <Container header={<Header variant="h3">Quick Actions</Header>}>
            <SpaceBetween size="s">
              {[
                { label: 'Manage Users', path: '/users' },
                { label: 'Manage Domains', path: '/tenancy/domains' },
                { label: 'Audit Logs', path: '/audit-logs' },
                { label: 'DKIM Keys', path: '/security/dkim-keys' },
                { label: 'Quota Usage', path: '/storage/quota-usage' },
                { label: 'API Health', path: '/system/health' },
              ].map(({ label, path }) => (
                <Button
                  key={label}
                  variant="inline-link"
                  onClick={() => router.push(`/companies/${companyId}${path}`)}
                >
                  {label} →
                </Button>
              ))}
            </SpaceBetween>
          </Container>
        </ColumnLayout>
      </SpaceBetween>
    </ContentLayout>
  );
}
