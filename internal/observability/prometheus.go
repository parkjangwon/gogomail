package observability

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/ldapgw"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

// Compile-time interface checks.
var _ smtpd.Metrics = (*PrometheusAdapter)(nil)
var _ delivery.Metrics = (*PrometheusAdapter)(nil)
var _ ldapgw.Metrics = (*PrometheusAdapter)(nil)

type PrometheusAdapter struct {
	mu       sync.Mutex
	counters map[promCounterKey]uint64
}

type promCounterKey struct {
	Name   string
	Labels string
}

func NewPrometheusAdapter() *PrometheusAdapter {
	return &PrometheusAdapter{counters: make(map[promCounterKey]uint64)}
}

func (a *PrometheusAdapter) ObserveSMTP(_ context.Context, event smtpd.MetricEvent) {
	a.inc("gogomail_smtp_events_total", map[string]string{
		"stage":  string(event.Stage),
		"result": string(event.Result),
	})
}

func (a *PrometheusAdapter) ObserveRFCNonCompliance(compliance smtpd.RFCCompliance) {
	labels := map[string]string{
		"rfc5322": boolLabel(compliance.RFC5322Valid),
		"rfc5321": boolLabel(compliance.RFC5321Valid),
	}
	a.inc("gogomail_smtp_rfc_noncompliance_total", labels)
}

func boolLabel(v bool) string {
	if v {
		return "valid"
	}
	return "invalid"
}

func (a *PrometheusAdapter) ObserveDelivery(_ context.Context, event delivery.MetricEvent) {
	a.inc("gogomail_delivery_events_total", map[string]string{
		"stage":            string(event.Stage),
		"result":           string(event.Result),
		"farm":             event.Farm,
		"route_pool":       event.RoutePool,
		"recipient_bucket": recipientBucket(event.RecipientCount),
	})
}

func (a *PrometheusAdapter) ObserveLDAP(_ context.Context, event ldapgw.MetricEvent) {
	a.inc("gogomail_ldap_events_total", map[string]string{
		"operation": event.Operation,
		"result":    string(event.Result),
	})
}

func (a *PrometheusAdapter) Text() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	keys := make([]promCounterKey, 0, len(a.counters))
	for key := range a.counters {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return promLine(keys[i], a.counters[keys[i]]) < promLine(keys[j], a.counters[keys[j]])
	})
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(promLine(key, a.counters[key]))
		b.WriteByte('\n')
	}
	return b.String()
}

func (a *PrometheusAdapter) inc(name string, labels map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.counters == nil {
		a.counters = make(map[promCounterKey]uint64)
	}
	a.counters[promCounterKey{Name: name, Labels: promLabels(labels)}]++
}

func promLine(key promCounterKey, value uint64) string {
	if key.Labels == "" {
		return fmt.Sprintf("%s %d", key.Name, value)
	}
	return fmt.Sprintf("%s{%s} %d", key.Name, key.Labels, value)
}

func promLabels(labels map[string]string) string {
	labelKeys := make([]string, 0, len(labels))
	for label := range labels {
		labelKeys = append(labelKeys, label)
	}
	sort.Strings(labelKeys)
	parts := make([]string, 0, len(labelKeys))
	for _, label := range labelKeys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, label, promEscape(labels[label])))
	}
	return strings.Join(parts, ",")
}

func promEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func recipientBucket(count int) string {
	switch {
	case count <= 0:
		return "0"
	case count == 1:
		return "1"
	case count <= 10:
		return "2-10"
	case count <= 100:
		return "11-100"
	default:
		return "100+"
	}
}
