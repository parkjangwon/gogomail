"use client";

import { useState } from "react";
import { useAuditLogs } from "@/hooks";
import Table from "@cloudscape-design/components/table";
import Header from "@cloudscape-design/components/header";
import Container from "@cloudscape-design/components/container";
import Button from "@cloudscape-design/components/button";
import Box from "@cloudscape-design/components/box";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Spinner from "@cloudscape-design/components/spinner";

export default function AuditLogsPage() {
  const { data: logs = [], isLoading } = useAuditLogs();
  const [selectedItems, setSelectedItems] = useState<typeof logs>([]);

  const handleExport = () => {
    // TODO: Implement CSV/PDF export
  };

  return (
    <Box padding="l">
      <Container
        header={
          <Header
            variant="h1"
            actions={
              <SpaceBetween direction="horizontal" size="xs">
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
        {isLoading ? (
          <Box textAlign="center" padding="l">
            <Spinner />
          </Box>
        ) : (
          <Table
            columnDefinitions={[
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
                cell: (item: any) => item.resource_id,
              },
              {
                id: "admin_user_id",
                header: "Admin",
                cell: (item: any) => item.admin_user_id,
              },
              {
                id: "ip_address",
                header: "IP Address",
                cell: (item: any) => item.ip_address,
              },
              {
                id: "timestamp",
                header: "Timestamp",
                cell: (item: any) =>
                  new Date(item.timestamp).toLocaleString(),
                sortingField: "timestamp",
              },
            ]}
            items={logs}
            selectionType="multi"
            selectedItems={selectedItems}
            onSelectionChange={(e) => setSelectedItems(e.detail.selectedItems)}
            variant="full-page"
            pagination={<div />}
            empty={
              <Box textAlign="center" padding="xl" color="text-status-info">
                No audit logs found
              </Box>
            }
          />
        )}
      </Container>
    </Box>
  );
}
