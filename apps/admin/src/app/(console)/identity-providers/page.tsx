"use client";

import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";

export default function IdentityProvidersPage() {
  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Identity Providers</Header>}
      >
        <Box color="text-status-info">
          Database, LDAP, and RDBMS identity provider configuration coming soon...
        </Box>
      </Container>
    </Box>
  );
}
