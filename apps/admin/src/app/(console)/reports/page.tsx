"use client";

import { useState } from "react";
import { useGenerateReport } from "@/hooks";
import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";
import SpaceBetween from "@cloudscape-design/components/space-between";
import FormField from "@cloudscape-design/components/form-field";
import Select, { SelectProps } from "@cloudscape-design/components/select";
import Button from "@cloudscape-design/components/button";
import Input from "@cloudscape-design/components/input";
import Alert from "@cloudscape-design/components/alert";

const DEMO_COMPANY_ID = "demo-company";

const REPORT_TYPES: SelectProps.Option[] = [
  { label: "Audit Logs", value: "audit-logs" },
  { label: "Statistics", value: "statistics" },
  { label: "Domains Status", value: "domains-status" },
  { label: "Comprehensive", value: "comprehensive" },
];

const EXPORT_FORMATS: SelectProps.Option[] = [
  { label: "CSV", value: "csv" },
  { label: "PDF (HTML)", value: "pdf" },
];

export default function ReportsPage() {
  const [selectedReport, setSelectedReport] = useState<SelectProps.Option | null>(
    REPORT_TYPES[0]
  );
  const [selectedFormat, setSelectedFormat] = useState<SelectProps.Option | null>(
    EXPORT_FORMATS[0]
  );
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const generateMutation = useGenerateReport();

  const handleGenerateReport = async () => {
    if (!selectedReport?.value || !selectedFormat?.value) {
      return;
    }

    generateMutation.mutate({
      companyId: DEMO_COMPANY_ID,
      reportType: selectedReport.value as
        | "audit-logs"
        | "statistics"
        | "domains-status"
        | "comprehensive",
      format: selectedFormat.value as "csv" | "pdf",
      startDate: startDate || undefined,
      endDate: endDate || undefined,
    });
  };

  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Reports &amp; Exports</Header>}
      >
        <SpaceBetween direction="vertical" size="l">
          <Box padding="s">
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(250px, 1fr))",
                gap: "16px",
              }}
            >
              <FormField label="Report Type">
                <Select
                  selectedOption={selectedReport}
                  onChange={(e) => setSelectedReport(e.detail.selectedOption)}
                  options={REPORT_TYPES}
                />
              </FormField>

              <FormField label="Export Format">
                <Select
                  selectedOption={selectedFormat}
                  onChange={(e) => setSelectedFormat(e.detail.selectedOption)}
                  options={EXPORT_FORMATS}
                />
              </FormField>

              {selectedReport?.value === "audit-logs" ||
              selectedReport?.value === "comprehensive" ? (
                <>
                  <FormField label="Start Date (Optional)">
                    <Input
                      type="text"
                      placeholder="YYYY-MM-DD"
                      value={startDate}
                      onChange={(e) => setStartDate(e.detail.value)}
                    />
                  </FormField>

                  <FormField label="End Date (Optional)">
                    <Input
                      type="text"
                      placeholder="YYYY-MM-DD"
                      value={endDate}
                      onChange={(e) => setEndDate(e.detail.value)}
                    />
                  </FormField>
                </>
              ) : null}
            </div>
          </Box>

          {generateMutation.isSuccess && (
            <Alert type="success">
              Report generated successfully: {generateMutation.data?.filename}
            </Alert>
          )}

          {generateMutation.isError && (
            <Alert type="error">
              Failed to generate report. Please try again.
            </Alert>
          )}

          <Box>
            <Button
              variant="primary"
              loading={generateMutation.isPending}
              onClick={handleGenerateReport}
            >
              Generate &amp; Download Report
            </Button>
          </Box>

          <Container header={<Header variant="h2">Available Reports</Header>}>
            <SpaceBetween direction="vertical" size="m">
              <Box>
                <strong>Audit Logs</strong>
                <p style={{ color: "#666", fontSize: "14px", marginTop: "4px" }}>
                  Export audit logs with optional date range filtering. Available in CSV
                  and PDF formats.
                </p>
              </Box>

              <Box>
                <strong>Statistics</strong>
                <p style={{ color: "#666", fontSize: "14px", marginTop: "4px" }}>
                  System statistics including total users, active sessions, mail
                  operations, and recent audit logs.
                </p>
              </Box>

              <Box>
                <strong>Domains Status</strong>
                <p style={{ color: "#666", fontSize: "14px", marginTop: "4px" }}>
                  Current status of all configured mail domains including verification
                  status and operation counts.
                </p>
              </Box>

              <Box>
                <strong>Comprehensive</strong>
                <p style={{ color: "#666", fontSize: "14px", marginTop: "4px" }}>
                  Complete report combining audit logs and domain status with optional
                  date range filtering.
                </p>
              </Box>
            </SpaceBetween>
          </Container>
        </SpaceBetween>
      </Container>
    </Box>
  );
}
