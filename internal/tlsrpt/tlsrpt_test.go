package tlsrpt_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/tlsrpt"
)

func TestParsePolicy(t *testing.T) {
	tests := []struct {
		name    string
		txt     string
		want    *tlsrpt.Policy
		wantErr bool
	}{
		{
			name: "valid policy",
			txt:  "v=TLSRPTv1; rua=mailto:tlsrpt@example.com; ruf=mailto:tlsrpt-fail@example.com",
			want: &tlsrpt.Policy{
				Version: "TLSRPTv1",
				RUA:     "tlsrpt@example.com",
				RUF:     "tlsrpt-fail@example.com",
			},
			wantErr: false,
		},
		{
			name: "rua only",
			txt:  "v=TLSRPTv1; rua=mailto:tlsrpt@example.com",
			want: &tlsrpt.Policy{
				Version: "TLSRPTv1",
				RUA:     "tlsrpt@example.com",
			},
			wantErr: false,
		},
		{
			name:    "missing rua",
			txt:     "v=TLSRPTv1; ruf=mailto:fail@example.com",
			wantErr: true,
		},
		{
			name:    "invalid version",
			txt:     "v=TLSRPTv2; rua=mailto:tlsrpt@example.com",
			wantErr: true,
		},
		{
			name:    "rua not mailto",
			txt:     "v=TLSRPTv1; rua=https://example.com/tlsrpt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tlsrpt.ParsePolicy(tt.txt)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePolicy(%s) error = %v, wantErr %v", tt.txt, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Version != tt.want.Version || got.RUA != tt.want.RUA || got.RUF != tt.want.RUF {
				t.Fatalf("ParsePolicy(%s) = %+v, want %+v", tt.txt, got, tt.want)
			}
		})
	}
}

func TestIsValidReportAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantValid bool
	}{
		{"valid email", "tlsrpt@example.com", true},
		{"email with subdomain", "tlsrpt@mail.example.com", true},
		{"missing @", "tlsrpt-example.com", false},
		{"empty", "", false},
		{"local only", "tlsrpt@", false},
		{"domain only", "@example.com", false},
		{"multiple @", "tlsrpt@example.com@evil.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tlsrpt.IsValidReportAddress(tt.addr)
			if got != tt.wantValid {
				t.Fatalf("IsValidReportAddress(%s) = %v, want %v", tt.addr, got, tt.wantValid)
			}
		})
	}
}

func TestCollectorRecordFailure(t *testing.T) {
	collector := tlsrpt.NewCollector("example.com")

	failure := &tlsrpt.FailureDetails{
		ResultType:        "certificate-host-mismatch",
		SendingMTAIP:      "192.0.2.1",
		ReceivingMTAIP:    "192.0.2.10",
		FailureReasonCode: "certificate-host-mismatch",
		FailureReasonText: "Certificate does not match hostname",
	}

	collector.RecordFailure("tlsa", "mx.example.com", failure)
	collector.RecordFailure("tlsa", "mx.example.com", failure) // Record same twice

	report := collector.GenerateReport("Example Corp", "tlsrpt@example.com")
	if report.DomainName != "example.com" {
		t.Fatalf("expected domain example.com, got %s", report.DomainName)
	}
	if report.ReportCount != 1 {
		t.Fatalf("expected 1 policy section, got %d", report.ReportCount)
	}
	if len(report.Policies) == 0 {
		t.Fatal("expected at least one policy section")
	}

	policy := report.Policies[0]
	if policy.SummaryResults.TotalFailureSessionCount != 2 {
		t.Fatalf("expected 2 failure sessions, got %d", policy.SummaryResults.TotalFailureSessionCount)
	}
}

func TestCollectorRecordSuccess(t *testing.T) {
	collector := tlsrpt.NewCollector("example.com")

	success := &tlsrpt.SuccessDetails{
		TLSVersion:     "TLSv1.3",
		TLSCipherSuite: "TLS_AES_256_GCM_SHA384",
	}

	collector.RecordSuccess("tlsa", "mx.example.com", success)
	collector.RecordSuccess("tlsa", "mx.example.com", success)

	report := collector.GenerateReport("Example Corp", "tlsrpt@example.com")
	if len(report.Policies) == 0 {
		t.Fatal("expected at least one policy section")
	}

	policy := report.Policies[0]
	if policy.SummaryResults.TotalSuccessfulSessionCount != 2 {
		t.Fatalf("expected 2 success sessions, got %d", policy.SummaryResults.TotalSuccessfulSessionCount)
	}
}

func TestReportDateRange(t *testing.T) {
	collector := tlsrpt.NewCollector("example.com")
	report := collector.GenerateReport("Example Corp", "tlsrpt@example.com")

	// Check date range is in RFC 3339 format
	if !isValidRFC3339(report.DateRange.StartDatetime) {
		t.Fatalf("invalid start datetime: %s", report.DateRange.StartDatetime)
	}
	if !isValidRFC3339(report.DateRange.EndDatetime) {
		t.Fatalf("invalid end datetime: %s", report.DateRange.EndDatetime)
	}

	// Check end is 24 hours after start
	start, _ := time.Parse(time.RFC3339, report.DateRange.StartDatetime)
	end, _ := time.Parse(time.RFC3339, report.DateRange.EndDatetime)
	duration := end.Sub(start)
	if duration != 24*time.Hour {
		t.Fatalf("expected 24-hour duration, got %v", duration)
	}
}

func TestReportMarshalJSON(t *testing.T) {
	collector := tlsrpt.NewCollector("example.com")
	report := collector.GenerateReport("Example Corp", "tlsrpt@example.com")

	jsonBytes, err := report.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Fatal("expected non-empty JSON")
	}

	// Just verify it's valid JSON by parsing it back
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonBytes, &parsed)
	if err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Check for expected top-level fields
	if parsed["organization-name"] == nil {
		t.Fatal("missing organization-name in JSON")
	}
	if parsed["domain-name"] == nil {
		t.Fatal("missing domain-name in JSON")
	}
	if parsed["report-id"] == nil {
		t.Fatal("missing report-id in JSON")
	}
}

func isValidRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
