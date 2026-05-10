"use client";

import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";

export default function StatisticsPage() {
  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Statistics</Header>}
      >
        <Box color="text-status-info">
          Detailed statistics and charts coming soon...
        </Box>
      </Container>
    </Box>
  );
}
