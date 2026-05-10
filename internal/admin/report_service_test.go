package admin

import (
	"context"
	"strings"
	"testing"
	"time"
)

type mockReportDataProvider struct {
	logs []*AuditLog
}

func (m *mockReportDataProvider) GetAuditLogs(ctx context.Context, startTime, endTime time.Time) ([]*AuditLog, error) {
	var results []*AuditLog
	for _, log := range m.logs {
		if log.Timestamp.After(startTime) && log.Timestamp.Before(endTime) {
			results = append(results, log)
		}
	}
	return results, nil
}

func newMockReportDataProvider() *mockReportDataProvider {
	return &mockReportDataProvider{
		logs: []*AuditLog{
			{
				ID:           "log1",
				CompanyID:    "company-1",
				AdminUserID:  "admin1",
				Action:       "user.create",
				ResourceType: "user",
				ResourceID:   "user1",
				Timestamp:    time.Now().Add(-1 * time.Hour),
			},
			{
				ID:           "log2",
				CompanyID:    "company-1",
				AdminUserID:  "admin1",
				Action:       "user.update",
				ResourceType: "user",
				ResourceID:   "user1",
				Timestamp:    time.Now(),
			},
		},
	}
}

type ReportDataProvider interface {
	GetAuditLogs(ctx context.Context, startTime, endTime time.Time) ([]*AuditLog, error)
}

func TestReportServiceExportCSV(t *testing.T) {
	provider := newMockReportDataProvider()
	service := NewReportService(provider)
	ctx := context.Background()

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		shouldErr bool
	}{
		{
			name:      "export valid range",
			startTime: startTime,
			endTime:   endTime,
			shouldErr: false,
		},
		{
			name:      "export empty range",
			startTime: time.Now().Add(-100 * time.Hour),
			endTime:   time.Now().Add(-99 * time.Hour),
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv, err := service.ExportCSV(ctx, tt.startTime, tt.endTime)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ExportCSV() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && csv == "" {
				t.Error("ExportCSV() returned empty CSV")
			}
			// Verify CSV has header
			if err == nil && !strings.Contains(csv, "ID,CompanyID") {
				t.Error("ExportCSV() missing CSV header")
			}
		})
	}
}

func TestReportServiceExportPDF(t *testing.T) {
	provider := newMockReportDataProvider()
	service := NewReportService(provider)
	ctx := context.Background()

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		shouldErr bool
	}{
		{
			name:      "export valid range",
			startTime: startTime,
			endTime:   endTime,
			shouldErr: false,
		},
		{
			name:      "export empty range",
			startTime: time.Now().Add(-100 * time.Hour),
			endTime:   time.Now().Add(-99 * time.Hour),
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf, err := service.ExportPDF(ctx, tt.startTime, tt.endTime)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ExportPDF() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && len(pdf) == 0 {
				t.Error("ExportPDF() returned empty PDF")
			}
		})
	}
}

func TestReportServiceGenerateReport(t *testing.T) {
	provider := newMockReportDataProvider()
	service := NewReportService(provider)
	ctx := context.Background()

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name      string
		format    string
		startTime time.Time
		endTime   time.Time
		shouldErr bool
	}{
		{
			name:      "generate CSV report",
			format:    "csv",
			startTime: startTime,
			endTime:   endTime,
			shouldErr: false,
		},
		{
			name:      "generate PDF report",
			format:    "pdf",
			startTime: startTime,
			endTime:   endTime,
			shouldErr: false,
		},
		{
			name:      "invalid format",
			format:    "xml",
			startTime: startTime,
			endTime:   endTime,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := service.GenerateReport(ctx, tt.format, tt.startTime, tt.endTime)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GenerateReport() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && len(report) == 0 {
				t.Error("GenerateReport() returned empty report")
			}
		})
	}
}

func TestReportServiceGetReportStatus(t *testing.T) {
	provider := newMockReportDataProvider()
	service := NewReportService(provider)
	ctx := context.Background()

	tests := []struct {
		name      string
		reportID  string
		shouldErr bool
	}{
		{
			name:      "nonexistent report",
			reportID:  "nonexistent",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := service.GetReportStatus(ctx, tt.reportID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetReportStatus() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && status == "" {
				t.Error("GetReportStatus() returned empty status")
			}
		})
	}
}
