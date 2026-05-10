"use client";

import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";

export default function AuditPolicyPage() {
  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Audit Policy</Header>}
      >
        <Box color="text-status-info">
          Audit policy configuration (Level 1-3 audit levels) coming soon...
        </Box>
      </Container>
    </Box>
  );
}
