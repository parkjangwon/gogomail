package dnsbl_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/gogomail/gogomail/internal/dnsbl"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type stubResolver struct {
	records map[string][]string
	err     error
}

func (r *stubResolver) LookupHost(host string) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	if addrs, ok := r.records[host]; ok {
		return addrs, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func backpressureEvent(remoteAddr string) smtpd.Event {
	return smtpd.Event{
		Stage:      smtpd.StageBackpressureChecked,
		RemoteAddr: remoteAddr,
	}
}

// --- Checker (multi-zone) tests ---

func TestCheckerListedFirstZone(t *testing.T) {
	r := &stubResolver{records: map[string][]string{
		"4.3.2.1.zone-a.example": {"127.0.0.2"},
	}}
	c := dnsbl.NewChecker([]string{"zone-a.example", "zone-b.example"}, r)

	result, err := c.CheckAddr("1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Listed {
		t.Fatal("expected listed")
	}
	if result.Zone != "zone-a.example" {
		t.Fatalf("zone = %q, want zone-a.example", result.Zone)
	}
}

func TestCheckerListedSecondZone(t *testing.T) {
	r := &stubResolver{records: map[string][]string{
		"4.3.2.1.zone-b.example": {"127.0.0.3"},
	}}
	c := dnsbl.NewChecker([]string{"zone-a.example", "zone-b.example"}, r)

	result, err := c.CheckAddr("1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Listed {
		t.Fatal("expected listed from second zone")
	}
	if result.Zone != "zone-b.example" {
		t.Fatalf("zone = %q, want zone-b.example", result.Zone)
	}
}

func TestCheckerUnlisted(t *testing.T) {
	r := &stubResolver{}
	c := dnsbl.NewChecker([]string{"zone-a.example"}, r)

	result, err := c.CheckAddr("1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Listed {
		t.Fatal("expected not listed")
	}
}

func TestCheckerStripPort(t *testing.T) {
	r := &stubResolver{records: map[string][]string{
		"4.3.2.1.bl.example": {"127.0.0.2"},
	}}
	c := dnsbl.NewChecker([]string{"bl.example"}, r)

	result, err := c.CheckAddr("1.2.3.4:25")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Listed {
		t.Fatal("expected listed with port stripped")
	}
}

func TestCheckerNoZones(t *testing.T) {
	r := &stubResolver{}
	c := dnsbl.NewChecker(nil, r)

	result, err := c.CheckAddr("1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Listed {
		t.Fatal("no zones: expected not listed")
	}
}

// --- Hook tests ---

func TestHookSkipsWrongStage(t *testing.T) {
	r := &stubResolver{records: map[string][]string{
		"4.3.2.1.bl.example": {"127.0.0.2"},
	}}
	c := dnsbl.NewChecker([]string{"bl.example"}, r)
	h := dnsbl.Hook(dnsbl.HookOptions{Checker: c, Policy: dnsbl.PolicyReject})

	ev := backpressureEvent("1.2.3.4:25")
	ev.Stage = smtpd.StageParsed
	if err := h(context.Background(), ev); err != nil {
		t.Fatalf("wrong stage: unexpected error: %v", err)
	}
}

func TestHookRejectPolicyListed(t *testing.T) {
	r := &stubResolver{records: map[string][]string{
		"4.3.2.1.bl.example": {"127.0.0.2"},
	}}
	c := dnsbl.NewChecker([]string{"bl.example"}, r)
	h := dnsbl.Hook(dnsbl.HookOptions{Checker: c, Policy: dnsbl.PolicyReject})

	if err := h(context.Background(), backpressureEvent("1.2.3.4:25")); err == nil {
		t.Fatal("reject policy: expected error for listed IP")
	}
}

func TestHookMonitorPolicyListed(t *testing.T) {
	r := &stubResolver{records: map[string][]string{
		"4.3.2.1.bl.example": {"127.0.0.2"},
	}}
	c := dnsbl.NewChecker([]string{"bl.example"}, r)
	h := dnsbl.Hook(dnsbl.HookOptions{Checker: c, Policy: dnsbl.PolicyMonitor})

	if err := h(context.Background(), backpressureEvent("1.2.3.4:25")); err != nil {
		t.Fatalf("monitor policy: unexpected error: %v", err)
	}
}

func TestHookUnlistedIP(t *testing.T) {
	r := &stubResolver{}
	c := dnsbl.NewChecker([]string{"bl.example"}, r)
	h := dnsbl.Hook(dnsbl.HookOptions{Checker: c, Policy: dnsbl.PolicyReject})

	if err := h(context.Background(), backpressureEvent("5.6.7.8:25")); err != nil {
		t.Fatalf("unlisted: unexpected error: %v", err)
	}
}

func TestHookNilChecker(t *testing.T) {
	h := dnsbl.Hook(dnsbl.HookOptions{Checker: nil, Policy: dnsbl.PolicyReject})
	if err := h(context.Background(), backpressureEvent("1.2.3.4:25")); err != nil {
		t.Fatalf("nil checker: unexpected error: %v", err)
	}
}

func TestHookLookupErrorFailsOpen(t *testing.T) {
	r := &stubResolver{err: errors.New("network timeout")}
	c := dnsbl.NewChecker([]string{"bl.example"}, r)
	h := dnsbl.Hook(dnsbl.HookOptions{Checker: c, Policy: dnsbl.PolicyReject})

	// Lookup error should not reject the message (fail open)
	if err := h(context.Background(), backpressureEvent("1.2.3.4:25")); err != nil {
		t.Fatalf("lookup error: should fail open, got: %v", err)
	}
}
