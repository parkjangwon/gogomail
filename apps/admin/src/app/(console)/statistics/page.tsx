"use client";

import { useStatistics, useMailVolumeMetrics, useTopDomainsMetrics } from "@/hooks";
import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Spinner from "@cloudscape-design/components/spinner";
import Table from "@cloudscape-design/components/table";
import ProgressBar from "@cloudscape-design/components/progress-bar";
import Badge from "@cloudscape-design/components/badge";

const DEMO_COMPANY_ID = "demo-company";

function MetricCard({
  label,
  value,
  loading,
}: {
  label: string;
  value: string | number;
  loading?: boolean;
}) {
  return (
    <Box padding="m" textAlign="center">
      <div
        style={{
          borderRadius: "6px",
          backgroundColor: "#f0f2f5",
          padding: "24px",
          minHeight: "120px",
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
        }}
      >
        <div
          style={{
            fontSize: "14px",
            color: "#434d5c",
            marginBottom: "8px",
            fontWeight: 500,
          }}
        >
          {label}
        </div>
        {loading ? (
          <Spinner />
        ) : (
          <div
            style={{
              fontSize: "32px",
              fontWeight: 700,
              color: "#0972d3",
            }}
          >
            {value}
          </div>
        )}
      </div>
    </Box>
  );
}

export default function StatisticsPage() {
  const { data: stats, isLoading: statsLoading } = useStatistics(DEMO_COMPANY_ID);
  const { data: mailVolume, isLoading: mailVolumeLoading } = useMailVolumeMetrics(
    DEMO_COMPANY_ID
  );
  const { data: topDomains, isLoading: topDomainsLoading } = useTopDomainsMetrics(
    DEMO_COMPANY_ID
  );

  const lastUpdated = stats?.timestamp
    ? new Date(stats.timestamp).toLocaleTimeString()
    : "—";

  return (
    <Box padding="l">
      <Container
        header={
          <Header
            variant="h1"
            actions={
              <Box color="text-status-info" fontSize="body-s">
                Last updated: {lastUpdated}
              </Box>
            }
          >
            Statistics &amp; Dashboard
          </Header>
        }
      >
        <SpaceBetween direction="vertical" size="l">
          {/* Key Metrics */}
          <Box>
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
                gap: "16px",
              }}
            >
              <MetricCard
                label="Total Users"
                value={stats?.total_users ?? 0}
                loading={statsLoading}
              />
              <MetricCard
                label="Active Sessions"
                value={stats?.active_sessions ?? 0}
                loading={statsLoading}
              />
              <MetricCard
                label="Mail Operations (24h)"
                value={stats?.mail_operations ?? 0}
                loading={statsLoading}
              />
              <MetricCard
                label="Audit Logs (24h)"
                value={stats?.audit_logs_24h ?? 0}
                loading={statsLoading}
              />
            </div>
          </Box>

          {/* Mail Volume Metrics */}
          <Container header={<Header variant="h2">Mail Volume (Last 24h)</Header>}>
            {mailVolumeLoading ? (
              <Box textAlign="center" padding="l">
                <Spinner />
              </Box>
            ) : mailVolume && mailVolume.length > 0 ? (
              <Table
                columnDefinitions={[
                  {
                    id: "hour",
                    header: "Hour",
                    cell: (item: any) => item.hour,
                  },
                  {
                    id: "sent",
                    header: "Sent",
                    cell: (item: any) => (
                      <Badge color="green">{item.sent}</Badge>
                    ),
                  },
                  {
                    id: "received",
                    header: "Received",
                    cell: (item: any) => (
                      <Badge color="blue">{item.received}</Badge>
                    ),
                  },
                  {
                    id: "failed",
                    header: "Failed",
                    cell: (item: any) => (
                      <Badge color="red">{item.failed}</Badge>
                    ),
                  },
                ]}
                items={mailVolume}
                variant="embedded"
                empty={
                  <Box textAlign="center" padding="l">
                    No mail volume data available
                  </Box>
                }
              />
            ) : (
              <Box textAlign="center" padding="l" color="text-status-info">
                No mail volume data available
              </Box>
            )}
          </Container>

          {/* Top Domains */}
          <Container header={<Header variant="h2">Top Domains</Header>}>
            {topDomainsLoading ? (
              <Box textAlign="center" padding="l">
                <Spinner />
              </Box>
            ) : topDomains && topDomains.length > 0 ? (
              <SpaceBetween direction="vertical" size="m">
                {topDomains.map((domain, idx) => (
                  <Box key={idx}>
                    <div style={{ marginBottom: "8px" }}>
                      <strong>{domain.domain}</strong>
                      <span
                        style={{
                          marginLeft: "12px",
                          color: "#666",
                          fontSize: "12px",
                        }}
                      >
                        {domain.mail_count} operations
                      </span>
                    </div>
                    <div style={{ marginBottom: "4px", fontSize: "12px" }}>
                      Error Rate: {(domain.error_rate * 100).toFixed(2)}%
                    </div>
                    <ProgressBar
                      value={domain.error_rate * 100}
                      additionalInfo={`${(domain.error_rate * 100).toFixed(2)}%`}
                    />
                  </Box>
                ))}
              </SpaceBetween>
            ) : (
              <Box textAlign="center" padding="l" color="text-status-info">
                No domain metrics available
              </Box>
            )}
          </Container>
        </SpaceBetween>
      </Container>
    </Box>
  );
}
