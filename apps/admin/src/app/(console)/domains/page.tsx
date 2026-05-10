"use client";

import { useState } from "react";
import { useDomains } from "@/hooks";
import Table from "@cloudscape-design/components/table";
import Header from "@cloudscape-design/components/header";
import Container from "@cloudscape-design/components/container";
import Button from "@cloudscape-design/components/button";
import Box from "@cloudscape-design/components/box";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Spinner from "@cloudscape-design/components/spinner";
import Badge from "@cloudscape-design/components/badge";

export default function DomainsPage() {
  const { data: domains = [], isLoading } = useDomains();
  const [selectedItems, setSelectedItems] = useState<typeof domains>([]);

  return (
    <Box padding="l">
      <Container
        header={
          <Header
            variant="h1"
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button disabled={selectedItems.length === 0}>Delete</Button>
                <Button variant="primary">Add Domain</Button>
              </SpaceBetween>
            }
          >
            Domains
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
                id: "name",
                header: "Domain Name",
                cell: (item: any) => item.name,
                sortingField: "name",
              },
              {
                id: "verified",
                header: "Verified",
                cell: (item: any) => (
                  <Badge
                    color={item.verified ? "green" : "red"}
                  >
                    {item.verified ? "Verified" : "Pending"}
                  </Badge>
                ),
              },
              {
                id: "created_at",
                header: "Created",
                cell: (item: any) =>
                  new Date(item.created_at).toLocaleDateString(),
              },
            ]}
            items={domains}
            selectionType="multi"
            selectedItems={selectedItems}
            onSelectionChange={(e) => setSelectedItems(e.detail.selectedItems)}
            variant="full-page"
            pagination={<div />}
            empty={
              <Box textAlign="center" padding="xl" color="text-status-info">
                No domains found
              </Box>
            }
          />
        )}
      </Container>
    </Box>
  );
}
