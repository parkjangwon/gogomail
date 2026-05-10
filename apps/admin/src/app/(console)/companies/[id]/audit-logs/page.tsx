'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Button,
  Table,
  Pagination,
  Box,
  Spinner,
  Alert,
  Grid,
} from '@cloudscape-design/components';
import { useAuditLogs, useExportAuditLogs } from '@/hooks/useAuditLogs';

export default function AuditLogsPage() {
  const params = useParams();
  const companyId = params.id as string;

  const [filters, setFilters] = useState({
    start_date: '',
    end_date: '',
    action: '',
    resource_type: '',
    admin_user_id: '',
  });

  const [pagination, setPagination] = useState({
    currentPageIndex: 0,
    pageSize: 50,
  });

  const logsQuery = useAuditLogs(companyId, {
    ...filters,
    limit: pagination.pageSize,
    offset: pagination.currentPageIndex * pagination.pageSize,
  });

  const exportMutation = useExportAuditLogs();

  const logs = logsQuery.data?.logs || [];
  const totalItems = logsQuery.data?.total || 0;

  const handleExport = async (format: 'csv' | 'json') => {
    try {
      await exportMutation.mutateAsync({
        companyId,
        format,
        filters,
      });
    } catch (error) {
      console.error('Export failed:', error);
    }
  };

  const columns = [
    {
      header: 'Timestamp',
      cell: (item: any) => new Date(item.timestamp).toLocaleString(),
      width: 150,
    },
    { header: 'Action', cell: (item: any) => item.action },
    { header: 'Resource Type', cell: (item: any) => item.resource_type },
    { header: 'Resource ID', cell: (item: any) => item.resource_id || '-' },
    { header: 'Admin User', cell: (item: any) => item.admin_user_id },
    { header: 'IP Address', cell: (item: any) => item.ip_address || '-' },
  ];

  const pageCount = Math.ceil(totalItems / pagination.pageSize);

  return (
    <Container header={<Header>Audit Logs</Header>}>
      <SpaceBetween size="l">
        {/* Filters */}
        <SpaceBetween size="m">
          <Header variant="h3">Filters</Header>
          <Grid gridDefinition={[{ colspan: 4 }, { colspan: 4 }, { colspan: 4 }]}>
            <FormField label="Start Date">
              <Input
                value={filters.start_date}
                onChange={(e) =>
                  setFilters({ ...filters, start_date: e.detail.value })
                }
                placeholder="YYYY-MM-DD"
              />
            </FormField>
            <FormField label="End Date">
              <Input
                value={filters.end_date}
                onChange={(e) =>
                  setFilters({ ...filters, end_date: e.detail.value })
                }
                placeholder="YYYY-MM-DD"
              />
            </FormField>
            <FormField label="Action">
              <Input
                placeholder="e.g., CREATE, UPDATE, DELETE"
                value={filters.action}
                onChange={(e) =>
                  setFilters({ ...filters, action: e.detail.value })
                }
              />
            </FormField>
            <FormField label="Resource Type">
              <Input
                placeholder="e.g., user, domain, config"
                value={filters.resource_type}
                onChange={(e) =>
                  setFilters({ ...filters, resource_type: e.detail.value })
                }
              />
            </FormField>
            <FormField label="Admin User ID">
              <Input
                value={filters.admin_user_id}
                onChange={(e) =>
                  setFilters({ ...filters, admin_user_id: e.detail.value })
                }
              />
            </FormField>
            <Box />
          </Grid>
          <Button
            onClick={() =>
              setFilters({
                start_date: '',
                end_date: '',
                action: '',
                resource_type: '',
                admin_user_id: '',
              })
            }
          >
            Clear Filters
          </Button>
        </SpaceBetween>

        {/* Export Buttons */}
        <SpaceBetween direction="horizontal" size="xs">
          <Button
            onClick={() => handleExport('csv')}
            loading={exportMutation.isPending}
          >
            Export as CSV
          </Button>
          <Button
            onClick={() => handleExport('json')}
            loading={exportMutation.isPending}
          >
            Export as JSON
          </Button>
        </SpaceBetween>

        {/* Logs Table */}
        {logsQuery.isPending ? (
          <Spinner />
        ) : logs.length > 0 ? (
          <SpaceBetween size="m">
            <Table columnDefinitions={columns} items={logs} variant="full-page" />
            <Pagination
              currentPageIndex={pagination.currentPageIndex}
              pagesCount={pageCount}
              onNextPageClick={() =>
                setPagination({
                  ...pagination,
                  currentPageIndex: pagination.currentPageIndex + 1,
                })
              }
              onPreviousPageClick={() =>
                setPagination({
                  ...pagination,
                  currentPageIndex: pagination.currentPageIndex - 1,
                })
              }
            />
          </SpaceBetween>
        ) : (
          <Alert>No audit logs found</Alert>
        )}
      </SpaceBetween>
    </Container>
  );
}
