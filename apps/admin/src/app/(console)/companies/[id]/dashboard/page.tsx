'use client';

import { useParams } from 'next/navigation';
import {
  Container,
  Header,
  SpaceBetween,
  Box,
  ColumnLayout,
  StatusIndicator,
  Table,
  Spinner,
  Alert,
  SegmentedControl,
} from '@cloudscape-design/components';
import { useState } from 'react';
import { useDashboardStats, useActivityMetrics, useApiUsageMetrics, useSecurityEvents } from '@/hooks/useDashboard';

export default function DashboardPage() {
  const params = useParams();
  const companyId = params.id as string;
  const [dayRange, setDayRange] = useState('7');

  const statsQuery = useDashboardStats(companyId);
  const activityQuery = useActivityMetrics(companyId, parseInt(dayRange));
  const apiUsageQuery = useApiUsageMetrics(companyId);
  const securityQuery = useSecurityEvents(companyId, 10);

  const stats = statsQuery.data;
  const activities = activityQuery.data || [];
  const apiUsages = apiUsageQuery.data || [];
  const securityEvents = securityQuery.data || [];

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'critical': return 'error';
      case 'high': return 'warning';
      case 'medium': return 'warning';
      case 'low': return 'info';
      default: return 'info';
    }
  };

  const activityColumns = [
    {
      header: 'Date',
      cell: (item: any) => new Date(item.date).toLocaleDateString(),
      width: 100,
    },
    { header: 'Logins', cell: (item: any) => item.user_logins },
    { header: 'API Calls', cell: (item: any) => item.api_calls },
    { header: 'Sent', cell: (item: any) => item.mail_sent },
    { header: 'Received', cell: (item: any) => item.mail_received },
  ];

  const apiUsageColumns = [
    { header: 'Endpoint', cell: (item: any) => item.endpoint },
    { header: 'Requests', cell: (item: any) => item.requests_count.toLocaleString() },
    { header: 'Avg Latency (ms)', cell: (item: any) => item.avg_latency_ms.toFixed(2) },
    {
      header: 'Error Rate',
      cell: (item: any) => (
        <StatusIndicator type={item.error_rate > 5 ? 'warning' : 'success'}>
          {item.error_rate.toFixed(2)}%
        </StatusIndicator>
      ),
    },
  ];

  const securityColumns = [
    { header: 'Event Type', cell: (item: any) => item.event_type },
    {
      header: 'Severity',
      cell: (item: any) => (
        <StatusIndicator type={getSeverityColor(item.severity)}>
          {item.severity.toUpperCase()}
        </StatusIndicator>
      ),
    },
    { header: 'Description', cell: (item: any) => item.description },
    {
      header: 'Time',
      cell: (item: any) => new Date(item.timestamp).toLocaleString(),
    },
  ];

  return (
    <Container header={<Header>Dashboard</Header>}>
      <SpaceBetween size="xl">
        {/* Statistics Cards */}
        {statsQuery.isPending ? (
          <Spinner />
        ) : stats ? (
          <ColumnLayout columns={4} variant="text-grid">
            <Box>
              <div style={{ fontSize: '24px', fontWeight: 'bold' }}>
                {stats.total_users}
              </div>
              <div style={{ color: '#666', fontSize: '12px' }}>Total Users</div>
            </Box>
            <Box>
              <div style={{ fontSize: '24px', fontWeight: 'bold' }}>
                {stats.total_domains}
              </div>
              <div style={{ color: '#666', fontSize: '12px' }}>Total Domains</div>
            </Box>
            <Box>
              <div style={{ fontSize: '24px', fontWeight: 'bold' }}>
                {stats.active_sessions}
              </div>
              <div style={{ color: '#666', fontSize: '12px' }}>Active Sessions</div>
            </Box>
            <Box>
              <div style={{ fontSize: '24px', fontWeight: 'bold' }}>
                {stats.pending_invitations}
              </div>
              <div style={{ color: '#666', fontSize: '12px' }}>Pending Invites</div>
            </Box>
          </ColumnLayout>
        ) : null}

        {/* Activity Metrics */}
        <Box>
          <SpaceBetween size="m">
            <Header
              actions={
                <SegmentedControl
                  selectedId={dayRange}
                  onChange={(e) => setDayRange(e.detail.selectedId)}
                  options={[
                    { id: '7', text: 'Last 7 Days' },
                    { id: '30', text: 'Last 30 Days' },
                    { id: '90', text: 'Last 90 Days' },
                  ]}
                />
              }
            >
              Activity Metrics
            </Header>
            {activityQuery.isPending ? (
              <Spinner />
            ) : activities.length > 0 ? (
              <Table
                columnDefinitions={activityColumns}
                items={activities}
                variant="embedded"
              />
            ) : (
              <Alert>No activity data available</Alert>
            )}
          </SpaceBetween>
        </Box>

        {/* API Usage */}
        <Box>
          <SpaceBetween size="m">
            <Header>API Usage (Last 24 Hours)</Header>
            {apiUsageQuery.isPending ? (
              <Spinner />
            ) : apiUsages.length > 0 ? (
              <Table
                columnDefinitions={apiUsageColumns}
                items={apiUsages}
                variant="embedded"
              />
            ) : (
              <Alert>No API usage data available</Alert>
            )}
          </SpaceBetween>
        </Box>

        {/* Security Events */}
        <Box>
          <SpaceBetween size="m">
            <Header>Recent Security Events</Header>
            {securityQuery.isPending ? (
              <Spinner />
            ) : securityEvents.length > 0 ? (
              <Table
                columnDefinitions={securityColumns}
                items={securityEvents}
                variant="embedded"
              />
            ) : (
              <Alert>No security events</Alert>
            )}
          </SpaceBetween>
        </Box>
      </SpaceBetween>
    </Container>
  );
}
