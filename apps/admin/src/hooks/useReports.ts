import { useMutation } from "@tanstack/react-query";
import { exportToCSV, generatePDFReport, formatDataAsHTML } from "@/lib/export";

export interface ReportOptions {
  companyId: string;
  reportType: "audit-logs" | "statistics" | "domains-status" | "comprehensive";
  format: "csv" | "pdf";
  startDate?: string;
  endDate?: string;
}

export function useGenerateReport() {
  return useMutation({
    mutationFn: async (options: ReportOptions) => {
      const { companyId, reportType, format } = options;

      if (reportType === "audit-logs") {
        return generateAuditLogsReport(companyId, format);
      } else if (reportType === "statistics") {
        return generateStatisticsReport(companyId, format);
      } else if (reportType === "domains-status") {
        return generateDomainsStatusReport(companyId, format);
      } else if (reportType === "comprehensive") {
        return generateComprehensiveReport(companyId, format);
      }

      throw new Error("Unknown report type");
    },
  });
}

async function generateAuditLogsReport(
  _companyId: string,
  format: "csv" | "pdf"
) {
  const filename = `audit-logs-${new Date().toISOString().split("T")[0]}.${format}`;

  if (format === "csv") {
    const mockData = generateMockAuditLogs();
    exportToCSV(mockData, filename);
  } else {
    const mockData = generateMockAuditLogs();
    const html = formatDataAsHTML(mockData, "Audit Logs Report");
    generatePDFReport("Audit Logs Report", html, filename);
  }

  return { filename, success: true };
}

async function generateStatisticsReport(_companyId: string, format: "csv" | "pdf") {
  const filename = `statistics-${new Date().toISOString().split("T")[0]}.${format}`;

  const mockStats = {
    total_users: 142,
    active_sessions: 45,
    mail_operations: 12483,
    audit_logs_24h: 892,
    timestamp: new Date().toISOString(),
  };

  if (format === "csv") {
    exportToCSV([mockStats], filename);
  } else {
    const html = formatDataAsHTML([mockStats], "System Statistics");
    generatePDFReport("Statistics Report", html, filename);
  }

  return { filename, success: true };
}

async function generateDomainsStatusReport(_companyId: string, format: "csv" | "pdf") {
  const filename = `domains-status-${new Date().toISOString().split("T")[0]}.${format}`;

  const mockDomains = generateMockDomains();

  if (format === "csv") {
    exportToCSV(mockDomains, filename);
  } else {
    const html = formatDataAsHTML(mockDomains, "Domains Status Report");
    generatePDFReport("Domains Status Report", html, filename);
  }

  return { filename, success: true };
}

async function generateComprehensiveReport(
  _companyId: string,
  format: "csv" | "pdf"
) {
  const filename = `comprehensive-report-${new Date().toISOString().split("T")[0]}.${format}`;

  if (format === "csv") {
    const mockData = generateMockAuditLogs();
    exportToCSV(mockData, filename);
  } else {
    const auditLogsHtml = formatDataAsHTML(
      generateMockAuditLogs(),
      "Recent Audit Logs"
    );
    const domainsHtml = formatDataAsHTML(generateMockDomains(), "Domain Status");
    const html = auditLogsHtml + domainsHtml;
    generatePDFReport("Comprehensive Report", html, filename);
  }

  return { filename, success: true };
}

function generateMockAuditLogs() {
  return [
    {
      id: "log-001",
      action: "CREATE",
      resource_type: "USER",
      resource_id: "user-123",
      admin_user_id: "admin-1",
      ip_address: "192.168.1.1",
      timestamp: new Date().toISOString(),
    },
    {
      id: "log-002",
      action: "UPDATE",
      resource_type: "DOMAIN",
      resource_id: "domain-456",
      admin_user_id: "admin-1",
      ip_address: "192.168.1.1",
      timestamp: new Date(Date.now() - 3600000).toISOString(),
    },
    {
      id: "log-003",
      action: "DELETE",
      resource_type: "USER",
      resource_id: "user-789",
      admin_user_id: "admin-2",
      ip_address: "192.168.1.2",
      timestamp: new Date(Date.now() - 7200000).toISOString(),
    },
  ];
}

function generateMockDomains() {
  return [
    {
      domain: "mail.example.com",
      verified: true,
      created_at: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
      mail_count: 15234,
    },
    {
      domain: "smtp.example.org",
      verified: true,
      created_at: new Date(Date.now() - 60 * 24 * 60 * 60 * 1000).toISOString(),
      mail_count: 8942,
    },
    {
      domain: "relay.example.net",
      verified: false,
      created_at: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
      mail_count: 234,
    },
  ];
}
