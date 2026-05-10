"use client";

import { useState, useCallback } from "react";
import { useAuditLogs } from "@/hooks";
import Table from "@cloudscape-design/components/table";
import Header from "@cloudscape-design/components/header";
import Container from "@cloudscape-design/components/container";
import Button from "@cloudscape-design/components/button";
import Box from "@cloudscape-design/components/box";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Spinner from "@cloudscape-design/components/spinner";
import FormField from "@cloudscape-design/components/form-field";
import Input from "@cloudscape-design/components/input";
import Select, { SelectProps } from "@cloudscape-design/components/select";
import Pagination from "@cloudscape-design/components/pagination";

const DEMO_COMPANY_ID = "demo-company";

const ACTION_OPTIONS: SelectProps.Option[] = [
  { label: "All Actions", value: "" },
  { label: "CREATE", value: "CREATE" },
  { label: "UPDATE", value: "UPDATE" },
  { label: "DELETE", value: "DELETE" },
  { label: "READ", value: "READ" },
  { label: "LOGIN", value: "LOGIN" },
  { label: "LOGOUT", value: "LOGOUT" },
];

export default function AuditLogsPage() {
  const [selectedItems, setSelectedItems] = useState<any[]>([]);
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [actionFilter, setActionFilter] = useState<SelectProps.Option | null>(
    ACTION_OPTIONS[0]
  );
  const [adminUserFilter, setAdminUserFilter] = useState("");
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize] = useState(20);

  const filter = {
    company_id: DEMO_COMPANY_ID,
    action: actionFilter?.value || undefined,
    admin_user_id: adminUserFilter || undefined,
    start_time: startDate || undefined,
    end_time: endDate || undefined,
    limit: pageSize,
    offset: (currentPage - 1) * pageSize,
  };

  const { data: logs = [], isLoading, refetch } = useAuditLogs(filter);

  const handleExport = () => {
    // TODO: Implement CSV/PDF export in P3
  };

  const handleReset = useCallback(() => {
    setStartDate("");
    setEndDate("");
    setActionFilter(ACTION_OPTIONS[0]);
    setAdminUserFilter("");
    setCurrentPage(1);
    refetch();
  }, [refetch]);

  return (
    <Box padding="l">
      <Container
        header={
          <Header
            variant="h1"
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={handleReset}>Clear Filters</Button>
                <Button disabled={selectedItems.length === 0}>
                  Delete Selected
                </Button>
                <Button onClick={handleExport}>Export</Button>
              </SpaceBetween>
            }
          >
            Audit Logs
          </Header>
        }
      >
        <SpaceBetween direction="vertical" size="l">
          <Box padding="s">
            <SpaceBetween direction="vertical" size="m">
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "repeat(auto-fit, minmax(250px, 1fr))",
                  gap: "16px",
                }}
              >
                <FormField label="Start Date">
                  <Input
                    type="text"
                    placeholder="YYYY-MM-DD"
                    value={startDate}
                    onChange={(e) => {
                      setStartDate(e.detail.value);
                      setCurrentPage(1);
                    }}
                  />
                </FormField>

                <FormField label="End Date">
                  <Input
                    type="text"
                    placeholder="YYYY-MM-DD"
                    value={endDate}
                    onChange={(e) => {
                      setEndDate(e.detail.value);
                      setCurrentPage(1);
                    }}
                  />
                </FormField>

                <FormField label="Action">
                  <Select
                    selectedOption={actionFilter}
                    onChange={(e) => {
                      setActionFilter(e.detail.selectedOption);
                      setCurrentPage(1);
                    }}
                    options={ACTION_OPTIONS}
                  />
                </FormField>

                <FormField label="Admin User">
                  <Input
                    value={adminUserFilter}
                    onChange={(e) => {
                      setAdminUserFilter(e.detail.value);
                      setCurrentPage(1);
                    }}
                    placeholder="Filter by admin user ID"
                  />
                </FormField>
              </div>
            </SpaceBetween>
          </Box>

          {isLoading ? (
            <Box textAlign="center" padding="l">
              <Spinner />
            </Box>
          ) : (
            <>
              <Table
                columnDefinitions={[
                  {
                    id: "timestamp",
                    header: "Timestamp",
                    cell: (item: any) =>
                      new Date(item.timestamp).toLocaleString(),
                    sortingField: "timestamp",
                  },
                  {
                    id: "action",
                    header: "Action",
                    cell: (item: any) => item.action,
                    sortingField: "action",
                  },
                  {
                    id: "resource_type",
                    header: "Resource Type",
                    cell: (item: any) => item.resource_type,
                  },
                  {
                    id: "resource_id",
                    header: "Resource ID",
                    cell: (item: any) => item.resource_id || "-",
                  },
                  {
                    id: "admin_user_id",
                    header: "Admin",
                    cell: (item: any) => item.admin_user_id,
                  },
                  {
                    id: "ip_address",
                    header: "IP Address",
                    cell: (item: any) => item.ip_address || "-",
                  },
                ]}
                items={logs}
                selectionType="multi"
                selectedItems={selectedItems}
                onSelectionChange={(e) => setSelectedItems(e.detail.selectedItems)}
                variant="full-page"
                pagination={
                  <Pagination
                    currentPageIndex={currentPage}
                    pagesCount={Math.ceil(100 / pageSize)}
                    onChange={(e) => setCurrentPage(e.detail.currentPageIndex)}
                  />
                }
                empty={
                  <Box textAlign="center" padding="xl" color="text-status-info">
                    No audit logs found
                  </Box>
                }
              />
            </>
          )}
        </SpaceBetween>
      </Container>
    </Box>
  );
}
