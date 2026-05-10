"use client";

import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";

export default function RolesPage() {
  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Roles</Header>}
      >
        <Box color="text-status-info">
          Roles and permissions management coming soon...
        </Box>
      </Container>
    </Box>
  );
}
