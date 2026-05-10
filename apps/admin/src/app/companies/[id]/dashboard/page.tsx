'use client';

import { useParams } from 'next/navigation';
import {
  ContentLayout,
  Header,
  Container,
  ColumnLayout,
  Box,
  KeyValuePairs,
  SpaceBetween,
  Spinner,
  Alert,
  StatusIndicator,
} from '@cloudscape-design/components';
import { useDashboard } from '@/hooks/useDashboard';
import { parseErrorResponse } from '@/lib/error-handler';

export default function DashboardPage() {
  const params = useParams();
  const companyId = params.id as string;
  const { data, isLoading, error } = useDashboard(companyId);

  if (isLoading) {
    return (
      <ContentLayout header={<Header variant="h1">Dashboard</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  if (error) {
    return (
      <ContentLayout header={<Header variant="h1">Dashboard</Header>}>
        <Alert type="error">
          {parseErrorResponse(error).getErrorMessage()}
        </Alert>
      </ContentLayout>
    );
  }

  const stats = data?.stats || { total_users: 0, active_domains: 0, total_storage_gb: 0 };
  const metrics = data?.activityMetrics || [];
  const apiMetrics = data?.apiUsageMetrics || { requests_today: 0, requests_this_month: 0 };
  const securityEvents = data?.securityEvents || [];

  return (
    <ContentLayout
      header={
        <Header variant="h1" description="System overview and key metrics">
          Dashboard
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* Key Statistics Cards */}
        <ColumnLayout columns={4} variant="text-grid">
          <Box>
            <Box variant="h3" color="text-body-secondary">
              <small>Total Users</small>
            </Box>
            <div style={{ fontSize: '28px', fontWeight: 'bold', marginTop: '4px' }}>
              {stats.total_users || 0}
            </div>
          </Box>
          <Box>
            <Box variant="h3" color="text-body-secondary">
              <small>Active Domains</small>
            </Box>
            <div style={{ fontSize: '28px', fontWeight: 'bold', marginTop: '4px' }}>
              {stats.active_domains || 0}
            </div>
          </Box>
          <Box>
            <Box variant="h3" color="text-body-secondary">
              <small>API Requests (24h)</small>
            </Box>
            <div style={{ fontSize: '28px', fontWeight: 'bold', marginTop: '4px' }}>
              {apiMetrics.requests_today || 0}
            </div>
          </Box>
          <Box>
            <Box variant="h3" color="text-body-secondary">
              <small>System Status</small>
            </Box>
            <div style={{ marginTop: '4px' }}>
              <StatusIndicator type="success">Operational</StatusIndicator>
            </div>
          </Box>
        </ColumnLayout>

        {/* Activity Summary */}
        <Container header={<Header variant="h2">Recent Activity</Header>}>
          {metrics.length > 0 ? (
            <KeyValuePairs
              columns={3}
              items={metrics.slice(-3).map((m) => ({
                label: new Date(m.date).toLocaleDateString(),
                value: `${m.user_logins} logins, ${m.api_calls} API calls`,
              }))}
            />
          ) : (
            <Box color="text-body-secondary" textAlign="center" padding="m">
              No activity data
            </Box>
          )}
        </Container>

        {/* Security Events */}
        {securityEvents.length > 0 && (
          <Container header={<Header variant="h2">Latest Security Events</Header>}>
            <SpaceBetween size="m">
              {securityEvents.slice(0, 3).map((event) => (
                <div key={event.id} style={{ paddingBottom: '12px', borderBottom: '1px solid #e1e4e8' }}>
                  <Box variant="h3">{event.event_type}</Box>
                  <KeyValuePairs
                    items={[
                      { label: 'Severity', value: <StatusIndicator type={event.severity === 'high' ? 'error' : 'warning'} /> },
                      { label: 'Time', value: new Date(event.timestamp).toLocaleString() },
                      { label: 'Details', value: event.description },
                    ]}
                  />
                </div>
              ))}
            </SpaceBetween>
          </Container>
        )}

        {/* Quick Actions */}
        <Container header={<Header variant="h2">Quick Actions</Header>}>
          <ColumnLayout columns={3} variant="text-grid">
            <div style={{ padding: '12px', borderBottom: '1px solid #e1e4e8' }}>
              <a href={`/companies/${companyId}/users`} style={{ textDecoration: 'none', color: 'inherit' }}>
                <Box variant="h3">👥 Manage Users</Box>
                <small>Create and manage user accounts</small>
              </a>
            </div>
            <div style={{ padding: '12px', borderBottom: '1px solid #e1e4e8' }}>
              <a href={`/companies/${companyId}/audit-logs`} style={{ textDecoration: 'none', color: 'inherit' }}>
                <Box variant="h3">📋 Audit Logs</Box>
                <small>View system activity and changes</small>
              </a>
            </div>
            <div style={{ padding: '12px', borderBottom: '1px solid #e1e4e8' }}>
              <a href={`/companies/${companyId}/domains`} style={{ textDecoration: 'none', color: 'inherit' }}>
                <Box variant="h3">🌐 Domains</Box>
                <small>Manage email domains</small>
              </a>
            </div>
          </ColumnLayout>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
