"use client";

import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";

export default function ReportsPage() {
  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Reports</Header>}
      >
        <Box color="text-status-info">
          CSV/PDF report generation and download coming soon...
        </Box>
      </Container>
    </Box>
  );
}
