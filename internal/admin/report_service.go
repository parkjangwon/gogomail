package admin

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"
)

// ReportService generates audit reports in various formats
type ReportService struct {
	provider ReportDataProvider
}

// NewReportService creates a new report service
func NewReportService(provider ReportDataProvider) *ReportService {
	return &ReportService{
		provider: provider,
	}
}

// ExportCSV exports audit logs as CSV
func (rs *ReportService) ExportCSV(ctx context.Context, startTime, endTime time.Time) (string, error) {
	logs, err := rs.provider.GetAuditLogs(ctx, startTime, endTime)
	if err != nil {
		return "", fmt.Errorf("failed to get audit logs: %w", err)
	}

	var buf bytes.Buffer

	// Write CSV header
	buf.WriteString("ID,CompanyID,AdminUserID,Action,ResourceType,ResourceID,IPAddress,Timestamp\n")

	// Write data rows
	for _, log := range logs {
		row := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s\n",
			csvQuote(log.ID),
			csvQuote(log.CompanyID),
			csvQuote(log.AdminUserID),
			csvQuote(log.Action),
			csvQuote(log.ResourceType),
			csvQuote(log.ResourceID),
			csvQuote(log.IPAddress),
			log.Timestamp.Format(time.RFC3339),
		)
		buf.WriteString(row)
	}

	return buf.String(), nil
}

// ExportPDF exports audit logs as PDF (simplified text-based)
func (rs *ReportService) ExportPDF(ctx context.Context, startTime, endTime time.Time) ([]byte, error) {
	logs, err := rs.provider.GetAuditLogs(ctx, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}

	var buf bytes.Buffer

	// Write PDF-like text header
	buf.WriteString("AUDIT REPORT\n")
	buf.WriteString("=" + strings.Repeat("=", 50) + "\n")
	buf.WriteString(fmt.Sprintf("Period: %s to %s\n", startTime.Format("2006-01-02"), endTime.Format("2006-01-02")))
	buf.WriteString(fmt.Sprintf("Total Records: %d\n", len(logs)))
	buf.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	// Write audit log entries
	for i, log := range logs {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, log.ID))
		buf.WriteString(fmt.Sprintf("   Company: %s\n", log.CompanyID))
		buf.WriteString(fmt.Sprintf("   Admin: %s\n", log.AdminUserID))
		buf.WriteString(fmt.Sprintf("   Action: %s\n", log.Action))
		buf.WriteString(fmt.Sprintf("   Resource: %s (%s)\n", log.ResourceType, log.ResourceID))
		buf.WriteString(fmt.Sprintf("   IP: %s\n", log.IPAddress))
		buf.WriteString(fmt.Sprintf("   Time: %s\n\n", log.Timestamp.Format(time.RFC3339)))
	}

	return buf.Bytes(), nil
}

// GenerateReport generates a report in the specified format
func (rs *ReportService) GenerateReport(ctx context.Context, format string, startTime, endTime time.Time) ([]byte, error) {
	switch strings.ToLower(format) {
	case "csv":
		csv, err := rs.ExportCSV(ctx, startTime, endTime)
		if err != nil {
			return nil, err
		}
		return []byte(csv), nil
	case "pdf":
		return rs.ExportPDF(ctx, startTime, endTime)
	default:
		return nil, fmt.Errorf("unsupported report format: %s", format)
	}
}

// GetReportStatus returns the status of a report generation job
func (rs *ReportService) GetReportStatus(ctx context.Context, reportID string) (string, error) {
	if reportID == "" {
		return "", fmt.Errorf("%w: reportID", ErrMissingRequiredField)
	}

	// In a real implementation, would check a job queue/database
	// For now, return error to simulate "not found"
	return "", fmt.Errorf("report not found: %s", reportID)
}

// csvQuote escapes CSV values with quotes if needed
func csvQuote(s string) string {
	if strings.ContainsAny(s, ",\"'\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
