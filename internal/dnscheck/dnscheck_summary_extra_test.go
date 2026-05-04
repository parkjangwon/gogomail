package dnscheck

import "testing"

func TestDomainReportSummaryStatusPrioritizesOperationalSeverity(t *testing.T) {
	t.Parallel()

	report := DomainReport{
		MX:    RecordCheck{Status: StatusOK},
		SPF:   RecordCheck{Status: StatusMissing},
		DMARC: RecordCheck{Status: StatusMismatch},
		DKIM:  []RecordCheck{{Status: StatusError}},
	}

	if got := report.SummaryStatus(); got != StatusError {
		t.Fatalf("SummaryStatus = %q, want %q", got, StatusError)
	}
}

func TestDomainReportSummaryStatusReturnsMissingBeforeOK(t *testing.T) {
	t.Parallel()

	report := DomainReport{
		MX:    RecordCheck{Status: StatusOK},
		SPF:   RecordCheck{Status: StatusMissing},
		DMARC: RecordCheck{Status: StatusOK},
	}

	if got := report.SummaryStatus(); got != StatusMissing {
		t.Fatalf("SummaryStatus = %q, want %q", got, StatusMissing)
	}
}
