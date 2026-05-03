package observability

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gogomail/gogomail/internal/delivery"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

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

func (a *PrometheusAdapter) ObserveDelivery(_ context.Context, event delivery.MetricEvent) {
	a.inc("gogomail_delivery_events_total", map[string]string{
		"stage":  string(event.Stage),
		"result": string(event.Result),
		"farm":   event.Farm,
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
