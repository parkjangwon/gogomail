"use client";

import { useState } from "react";
import { useUsers } from "@/hooks";
import Table from "@cloudscape-design/components/table";
import Header from "@cloudscape-design/components/header";
import Container from "@cloudscape-design/components/container";
import Button from "@cloudscape-design/components/button";
import Box from "@cloudscape-design/components/box";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Spinner from "@cloudscape-design/components/spinner";

export default function UsersPage() {
  const { data: users = [], isLoading } = useUsers();
  const [selectedItems, setSelectedItems] = useState<typeof users>([]);

  return (
    <Box padding="l">
      <Container
        header={
          <Header
            variant="h1"
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button disabled={selectedItems.length === 0}>Delete</Button>
                <Button variant="primary">Create User</Button>
              </SpaceBetween>
            }
          >
            Users
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
                id: "email",
                header: "Email",
                cell: (item: any) => item.email,
                sortingField: "email",
              },
              {
                id: "name",
                header: "Name",
                cell: (item: any) => item.name || "-",
              },
              {
                id: "role_id",
                header: "Role",
                cell: (item: any) => item.role_id,
              },
              {
                id: "created_at",
                header: "Created",
                cell: (item: any) =>
                  new Date(item.created_at).toLocaleDateString(),
              },
            ]}
            items={users}
            selectionType="multi"
            selectedItems={selectedItems}
            onSelectionChange={(e) => setSelectedItems(e.detail.selectedItems)}
            variant="full-page"
            pagination={<div />}
            empty={
              <Box textAlign="center" padding="xl" color="text-status-info">
                No users found
              </Box>
            }
          />
        )}
      </Container>
    </Box>
  );
}
