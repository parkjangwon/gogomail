package observability

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/ldapgw"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

// Compile-time interface checks.
var _ smtpd.Metrics = (*PrometheusAdapter)(nil)
var _ delivery.Metrics = (*PrometheusAdapter)(nil)
var _ ldapgw.Metrics = (*PrometheusAdapter)(nil)

var durationBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

type PrometheusAdapter struct {
	mu         sync.Mutex
	counters   map[promCounterKey]uint64
	histograms map[string]*promHistogram
}

type promCounterKey struct {
	Name   string
	Labels string
}

func NewPrometheusAdapter() *PrometheusAdapter {
	return &PrometheusAdapter{
		counters:   make(map[promCounterKey]uint64),
		histograms: make(map[string]*promHistogram),
	}
}

func (a *PrometheusAdapter) ObserveSMTP(_ context.Context, event smtpd.MetricEvent) {
	a.inc("gogomail_smtp_events_total", map[string]string{
		"stage":  string(event.Stage),
		"result": string(event.Result),
	})
	if event.Duration > 0 {
		a.observe("gogomail_smtp_session_duration_seconds", durationBuckets, event.Duration, nil)
	}
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

// ObserveHTTPRequest records HTTP request duration as a histogram.
func (a *PrometheusAdapter) ObserveHTTPRequest(method, route, status string, dur time.Duration) {
	labels := map[string]string{
		"method": method,
		"route":  route,
		"status": status,
	}
	a.observe("gogomail_http_request_duration_seconds", durationBuckets, dur.Seconds(), labels)
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

	// Emit histograms.
	histNames := make([]string, 0, len(a.histograms))
	for name := range a.histograms {
		histNames = append(histNames, name)
	}
	sort.Strings(histNames)
	for _, name := range histNames {
		h := a.histograms[name]
		b.WriteString(h.text(name))
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

func (a *PrometheusAdapter) observe(name string, buckets []float64, value float64, labels map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.histograms == nil {
		a.histograms = make(map[string]*promHistogram)
	}
	key := name
	if len(labels) > 0 {
		key = name + "{" + promLabels(labels) + "}"
	}
	h, ok := a.histograms[key]
	if !ok {
		h = newPromHistogram(buckets, labels)
		a.histograms[key] = h
	}
	h.record(value)
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

// promHistogram is a simple in-process histogram.
type promHistogram struct {
	buckets []float64
	counts  []uint64
	inf     uint64
	sum     float64
	total   uint64
	labels  map[string]string
}

func newPromHistogram(buckets []float64, labels map[string]string) *promHistogram {
	b := make([]float64, len(buckets))
	copy(b, buckets)
	sort.Float64s(b)
	return &promHistogram{
		buckets: b,
		counts:  make([]uint64, len(b)),
		labels:  labels,
	}
}

func (h *promHistogram) record(value float64) {
	h.sum += value
	h.total++
	for i, upper := range h.buckets {
		if value <= upper {
			h.counts[i]++
		}
	}
	h.inf++
}

func (h *promHistogram) text(name string) string {
	labelStr := promLabels(h.labels)
	var b strings.Builder
	for i, upper := range h.buckets {
		leLabel := fmt.Sprintf(`le="%s"`, formatFloat(upper))
		if labelStr != "" {
			b.WriteString(fmt.Sprintf("%s_bucket{%s,%s} %d\n", name, labelStr, leLabel, h.counts[i]))
		} else {
			b.WriteString(fmt.Sprintf("%s_bucket{%s} %d\n", name, leLabel, h.counts[i]))
		}
	}
	leInf := `le="+Inf"`
	if labelStr != "" {
		b.WriteString(fmt.Sprintf("%s_bucket{%s,%s} %d\n", name, labelStr, leInf, h.inf))
		b.WriteString(fmt.Sprintf("%s_sum{%s} %g\n", name, labelStr, h.sum))
		b.WriteString(fmt.Sprintf("%s_count{%s} %d\n", name, labelStr, h.total))
	} else {
		b.WriteString(fmt.Sprintf("%s_bucket{%s} %d\n", name, leInf, h.inf))
		b.WriteString(fmt.Sprintf("%s_sum %g\n", name, h.sum))
		b.WriteString(fmt.Sprintf("%s_count %d\n", name, h.total))
	}
	return b.String()
}

func formatFloat(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%.1f", f)
	}
	s := fmt.Sprintf("%g", f)
	return s
}
