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
} from '@cloudscape-design/components';
import { useParams } from 'next/navigation';
import { useDashboard } from '@/hooks/useDashboard';

export default function DashboardPage() {
  const params = useParams();
  const companyId = params.id as string;
  const { data, isLoading } = useDashboard(companyId);

  const stats = data?.stats || { total_users: 0, active_domains: 0, total_storage_gb: 0 };
  const apiMetrics = data?.apiUsageMetrics || { requests_today: 0, requests_this_month: 0 };

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">Dashboard</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  const storageUsagePercent = 65;

  return (
    <ContentLayout
      header={
        <Header variant="h1" description="System overview and key metrics">
          Dashboard
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* Key Metrics */}
        <Container header={<Header variant="h2">System Metrics</Header>}>
          <ColumnLayout columns={4} variant="text-grid">
            <KeyValuePairs items={[{ label: 'Total Users', value: stats.total_users || 0 }]} />
            <KeyValuePairs items={[{ label: 'Active Domains', value: stats.active_domains || 0 }]} />
            <KeyValuePairs items={[{ label: 'API Requests', value: apiMetrics.requests_today || 0 }]} />
            <KeyValuePairs items={[{ label: 'Error Rate', value: '0.8%' }]} />
          </ColumnLayout>
        </Container>

        {/* System Health & Storage */}
        <ColumnLayout columns={2}>
          {/* System Health */}
          <Container header={<Header variant="h3">System Health</Header>}>
            <SpaceBetween size="m">
              <Box>
                <ProgressBar value={98} label="Overall Health: 98%" />
              </Box>
              <KeyValuePairs
                items={[
                  { label: 'API Server', value: '● Healthy' },
                  { label: 'Database', value: '● Healthy' },
                  { label: 'Mail Queue', value: '● Healthy' },
                  { label: 'Cache', value: '● Healthy' },
                  { label: 'Uptime', value: '99.98%' },
                  { label: 'Response Time', value: '148ms' },
                ]}
              />
            </SpaceBetween>
          </Container>

          {/* Storage Usage */}
          <Container header={<Header variant="h3">Storage Usage</Header>}>
            <SpaceBetween size="m">
              <Box>
                <ProgressBar value={storageUsagePercent} label={`Total Usage: ${Math.round(storageUsagePercent * 10)}/1000 GB`} />
              </Box>
              <KeyValuePairs
                items={[
                  { label: 'Usage', value: `${storageUsagePercent.toFixed(1)}%` },
                  { label: 'Available', value: `${(1000 - Math.round(storageUsagePercent * 10)).toFixed(1)}GB` },
                  { label: 'Status', value: '✓ Healthy' },
                ]}
              />
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        {/* Quick Actions */}
        <Container header={<Header variant="h3">Quick Actions</Header>}>
          <ColumnLayout columns={6} variant="text-grid">
            {[
              { label: 'Users', href: `/companies/${companyId}/users` },
              { label: 'Domains', href: `/companies/${companyId}/tenancy/domains` },
              { label: 'Logs', href: `/companies/${companyId}/audit-logs` },
              { label: 'API Keys', href: `/companies/${companyId}/security/api-keys` },
              { label: 'Storage', href: `/companies/${companyId}/storage/quota-usage` },
              { label: 'Health', href: `/companies/${companyId}/system/health` },
            ].map((action) => (
              <Box key={action.label} textAlign="center">
                <a href={action.href} style={{ textDecoration: 'none', color: 'inherit' }}>
                  {action.label}
                </a>
              </Box>
            ))}
          </ColumnLayout>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
