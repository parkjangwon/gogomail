"use client";

import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";

export default function MailLogsPage() {
  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Mail Logs</Header>}
      >
        <Box color="text-status-info">
          Mail operation logs coming soon...
        </Box>
      </Container>
    </Box>
  );
}
