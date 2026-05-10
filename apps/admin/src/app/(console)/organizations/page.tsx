"use client";

import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";

export default function OrganizationsPage() {
  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Organizations</Header>}
      >
        <Box color="text-status-info">
          Organizations management coming soon...
        </Box>
      </Container>
    </Box>
  );
}
