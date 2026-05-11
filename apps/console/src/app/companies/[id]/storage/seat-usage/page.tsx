'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  ColumnLayout,
  ProgressBar,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';

interface SeatUsage {
  total_users: number;
  active_users: number;
  suspended_users: number;
  domain_count: number;
  storage_used: number;
  storage_limit: number;
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

export default function SeatUsagePage() {
  const params = useParams();
  const companyId = params?.id as string;
  const [data, setData] = useState<SeatUsage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!companyId) return;
    fetch(`/api/admin/companies/${companyId}/seat-usage`, { credentials: 'include' })
      .then(r => r.ok ? r.json() : Promise.reject(r.statusText))
      .then(setData)
      .catch(e => setError(String(e)))
      .finally(() => setLoading(false));
  }, [companyId]);

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Seat &amp; License Usage</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (error || !data) {
    return (
      <ContentLayout header={<Header variant="h1">Seat &amp; License Usage</Header>}>
        <Box color="text-status-error">{error || 'Failed to load usage data'}</Box>
      </ContentLayout>
    );
  }

  const storagePercent = data.storage_limit > 0
    ? Math.min(100, Math.round((data.storage_used / data.storage_limit) * 100))
    : 0;

  const storageStatus: 'error' | 'success' | 'in-progress' = storagePercent >= 90 ? 'error' : storagePercent >= 75 ? 'in-progress' : 'success';

  return (
    <ContentLayout
      header={
        <Header variant="h1" description="Track user seats and storage consumption across your organization.">
          Seat &amp; License Usage
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* User seats */}
        <ColumnLayout columns={3} variant="text-grid" minColumnWidth={160}>
          <Container header={<Header variant="h3">Total Users</Header>}>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{data.total_users}</Box>
              <Box color="text-body-secondary" fontSize="body-s">across {data.domain_count} domain{data.domain_count !== 1 ? 's' : ''}</Box>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">Active</Header>}>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold" color="text-status-success">
                {data.active_users}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {data.total_users > 0 ? `${Math.round((data.active_users / data.total_users) * 100)}%` : '—'} of total
              </Box>
            </SpaceBetween>
          </Container>

          <Container header={<Header variant="h3">Suspended</Header>}>
            <SpaceBetween size="xxs">
              <Box
                fontSize="display-l"
                fontWeight="bold"
                color={data.suspended_users > 0 ? 'text-status-warning' : undefined}
              >
                {data.suspended_users}
              </Box>
              <Box color="text-body-secondary" fontSize="body-s">
                {data.suspended_users > 0 ? 'Review and clean up suspended accounts' : 'No suspended accounts'}
              </Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        {/* Storage */}
        <Container header={<Header variant="h2">Storage Allocation</Header>}>
          <SpaceBetween size="m">
            <ColumnLayout columns={3} variant="text-grid">
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">Used</Box>
                <Box fontWeight="bold">{formatBytes(data.storage_used)}</Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">Allocated</Box>
                <Box fontWeight="bold">{data.storage_limit > 0 ? formatBytes(data.storage_limit) : 'Unlimited'}</Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">Available</Box>
                <Box fontWeight="bold">
                  {data.storage_limit > 0 ? formatBytes(Math.max(0, data.storage_limit - data.storage_used)) : '—'}
                </Box>
              </SpaceBetween>
            </ColumnLayout>

            {data.storage_limit > 0 && (
              <ProgressBar
                value={storagePercent}
                label={`Storage usage (${storagePercent}%)`}
                status={storageStatus}
                additionalInfo={`${formatBytes(data.storage_used)} of ${formatBytes(data.storage_limit)}`}
              />
            )}
          </SpaceBetween>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
