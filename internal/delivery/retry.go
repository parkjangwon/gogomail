package delivery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

var ErrRetryExhausted = errors.New("delivery retry attempts exhausted")

var retryAttemptJSONKey = []byte("retry_attempt")

type RetryPolicy struct {
	Delays      []time.Duration
	JitterRatio float64
	MaxDelay    time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Delays: []time.Duration{
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		8 * time.Hour,
		24 * time.Hour,
	}, JitterRatio: 0.20, MaxDelay: 24 * time.Hour}
}

// AggressiveBulkRetryPolicy optimizes for high-volume scenarios
// with faster recovery on transient errors and bounded retry window
func AggressiveBulkRetryPolicy() RetryPolicy {
	return RetryPolicy{Delays: []time.Duration{
		2 * time.Minute,  // First retry: 2 minutes (faster for transient)
		10 * time.Minute, // Second retry: 10 minutes
		1 * time.Hour,    // Third retry: 1 hour
		6 * time.Hour,    // Fourth retry: 6 hours (smaller window, fail-fast)
		12 * time.Hour,   // Final retry: 12 hours (bounded)
	}, JitterRatio: 0.15, MaxDelay: 12 * time.Hour}
}

func (p RetryPolicy) NextDelay(currentAttempt int) (time.Duration, bool) {
	if len(p.Delays) == 0 {
		return 0, false
	}
	if currentAttempt < 0 {
		currentAttempt = 0
	}
	if currentAttempt >= len(p.Delays) {
		return 0, false
	}
	return p.Delays[currentAttempt], true
}

func (p RetryPolicy) NextScheduledDelay(key string, currentAttempt int) (time.Duration, bool) {
	delay, ok := p.NextDelay(currentAttempt)
	if !ok {
		return 0, false
	}
	delay = deterministicJitter(delay, p.JitterRatio, key, currentAttempt)
	if p.MaxDelay > 0 && delay > p.MaxDelay {
		return p.MaxDelay, true
	}
	return delay, true
}

type RetryScheduler interface {
	ScheduleRetry(ctx context.Context, job Job, cause error) error
}

type PostgresRetryScheduler struct {
	db     *sql.DB
	policy RetryPolicy
	now    func() time.Time
}

func NewPostgresRetryScheduler(db *sql.DB, policy RetryPolicy) *PostgresRetryScheduler {
	return &PostgresRetryScheduler{db: db, policy: policy, now: time.Now}
}

func (s *PostgresRetryScheduler) ScheduleRetry(ctx context.Context, job Job, cause error) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	delay, ok := s.policy.NextScheduledDelay(job.MessageID, job.RetryAttempt)
	if !ok {
		return fmt.Errorf("%w for message %s", ErrRetryExhausted, job.MessageID)
	}

	next := job.QueuedMessage
	next.RetryAttempt = job.RetryAttempt + 1
	payload, err := retryPayload(job, next)
	if err != nil {
		return fmt.Errorf("marshal delivery retry payload: %w", err)
	}

	availableAt := s.now().UTC().Add(delay)
	const query = `
INSERT INTO outbox (topic, partition_key, dedupe_key, payload, status, available_at, last_error)
VALUES ($1, $2, $3, $4::jsonb, 'pending', $5, $6)
ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING`

	topic := "mail.outbound." + string(normalizeRetryFarm(job.Farm))
	errorMessage := retryErrorMessage(cause)

	if _, err := s.db.ExecContext(ctx, query, topic, job.MessageID, retryDedupeKey(job), string(payload), availableAt, errorMessage); err != nil {
		return fmt.Errorf("schedule delivery retry: %w", err)
	}
	return nil
}

func retryPayload(job Job, next QueuedMessage) ([]byte, error) {
	if len(job.rawPayload) > 0 {
		if payload, ok := patchRetryAttemptPayload(job.rawPayload, next.RetryAttempt); ok {
			return payload, nil
		}
	}
	return json.Marshal(next)
}

func patchRetryAttemptPayload(payload json.RawMessage, attempt int) ([]byte, bool) {
	raw := bytes.TrimSpace(payload)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil, false
	}
	attemptValue := strconv.AppendInt(nil, int64(attempt), 10)
	retryAttemptStart := -1
	retryAttemptEnd := -1
	for i := 1; i < len(raw)-1; {
		i = skipJSONSpace(raw, i)
		if i >= len(raw)-1 {
			break
		}
		if raw[i] == ',' {
			i++
			continue
		}
		if raw[i] != '"' {
			return nil, false
		}
		keyEnd, ok := skipJSONString(raw, i)
		if !ok {
			return nil, false
		}
		isRetryAttemptKey, ok := jsonObjectKeyIsRetryAttempt(raw[i:keyEnd])
		if !ok {
			return nil, false
		}
		i = skipJSONSpace(raw, keyEnd)
		if i >= len(raw) || raw[i] != ':' {
			return nil, false
		}
		valueStart := skipJSONSpace(raw, i+1)
		valueEnd, ok := skipJSONValue(raw, valueStart)
		if !ok {
			return nil, false
		}
		if isRetryAttemptKey {
			if retryAttemptStart >= 0 {
				return nil, false
			}
			retryAttemptStart = valueStart
			retryAttemptEnd = valueEnd
		}
		i = valueEnd
	}
	if retryAttemptStart >= 0 {
		out := make([]byte, 0, len(raw)+len(attemptValue)-(retryAttemptEnd-retryAttemptStart))
		out = append(out, raw[:retryAttemptStart]...)
		out = append(out, attemptValue...)
		out = append(out, raw[retryAttemptEnd:]...)
		return out, true
	}
	insertAt := len(raw) - 1
	out := make([]byte, 0, len(raw)+len(`,"retry_attempt":`)+len(attemptValue))
	out = append(out, raw[:insertAt]...)
	if !jsonObjectIsEmpty(raw) {
		out = append(out, ',')
	}
	out = append(out, `"retry_attempt":`...)
	out = append(out, attemptValue...)
	out = append(out, raw[insertAt:]...)
	return out, true
}

func skipJSONSpace(raw []byte, i int) int {
	for i < len(raw) {
		switch raw[i] {
		case ' ', '\n', '\r', '\t':
			i++
		default:
			return i
		}
	}
	return i
}

func skipJSONString(raw []byte, i int) (int, bool) {
	if i >= len(raw) || raw[i] != '"' {
		return 0, false
	}
	for i++; i < len(raw); i++ {
		switch raw[i] {
		case '\\':
			i++
		case '"':
			return i + 1, true
		}
	}
	return 0, false
}

func skipJSONValue(raw []byte, i int) (int, bool) {
	if i >= len(raw) {
		return 0, false
	}
	switch raw[i] {
	case '"':
		return skipJSONString(raw, i)
	case '{':
		return skipJSONComposite(raw, i, '{', '}')
	case '[':
		return skipJSONComposite(raw, i, '[', ']')
	default:
		for i < len(raw) {
			switch raw[i] {
			case ',', '}', ']':
				return skipJSONSpaceBack(raw, i), true
			case ' ', '\n', '\r', '\t':
				return skipJSONSpaceBack(raw, i), true
			default:
				i++
			}
		}
		return i, true
	}
}

func jsonObjectKeyIsRetryAttempt(raw []byte) (bool, bool) {
	if !bytes.Contains(raw, []byte{'\\'}) {
		if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
			return false, false
		}
		return bytes.Equal(raw[1:len(raw)-1], retryAttemptJSONKey), true
	}
	var key string
	if err := json.Unmarshal(raw, &key); err != nil {
		return false, false
	}
	return key == "retry_attempt", true
}

func skipJSONComposite(raw []byte, i int, open, close byte) (int, bool) {
	if i >= len(raw) || raw[i] != open {
		return 0, false
	}
	stack := []byte{close}
	for i++; i < len(raw); i++ {
		switch raw[i] {
		case '"':
			end, ok := skipJSONString(raw, i)
			if !ok {
				return 0, false
			}
			i = end - 1
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) == 0 || raw[i] != stack[len(stack)-1] {
				return 0, false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func skipJSONSpaceBack(raw []byte, i int) int {
	for i > 0 {
		switch raw[i-1] {
		case ' ', '\n', '\r', '\t':
			i--
		default:
			return i
		}
	}
	return i
}

func jsonObjectIsEmpty(raw []byte) bool {
	return skipJSONSpace(raw, 1) == len(raw)-1
}

func normalizeRetryFarm(farm outbound.Farm) outbound.Farm {
	return outbound.NormalizeFarm(farm)
}

func retryDedupeKey(job Job) string {
	recipients := job.Recipients()
	values := make([]string, 0, len(recipients))

	// Normalize and collect recipient emails
	for _, recipient := range recipients {
		values = append(values, strings.ToLower(strings.TrimSpace(recipient.Email)))
	}

	// Fast path: check if already sorted to avoid unnecessary sort
	isSorted := true
	for i := 1; i < len(values); i++ {
		if values[i] < values[i-1] {
			isSorted = false
			break
		}
	}
	if !isSorted {
		sort.Strings(values)
	}

	// Use StringBuilder to reduce allocations in concatenation
	messageID := strings.TrimSpace(job.MessageID)
	nextAttempt := strconv.Itoa(job.RetryAttempt + 1)
	var sb strings.Builder
	sb.Grow(len("retry:::") + len(messageID) + len(nextAttempt) + joinedStringBytes(values))
	sb.WriteString("retry:")
	sb.WriteString(messageID)
	sb.WriteString(":")
	sb.WriteString(nextAttempt)
	sb.WriteString(":")
	for i, value := range values {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(value)
	}

	return sb.String()
}

func joinedStringBytes(values []string) int {
	if len(values) == 0 {
		return 0
	}
	n := len(values) - 1
	for _, value := range values {
		n += len(value)
	}
	return n
}

func retryErrorMessage(cause error) string {
	if cause == nil {
		return ""
	}
	return truncateUTF8Bytes(cause.Error(), 2000)
}

func deterministicJitter(base time.Duration, ratio float64, key string, attempt int) time.Duration {
	if base <= 0 || ratio <= 0 || math.IsNaN(ratio) {
		return base
	}
	ratio = math.Min(ratio, 1)
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%s:%d", key, attempt)
	unit := float64(hash.Sum64()%1_000_000) / 999_999
	factor := 1 - ratio + unit*(2*ratio)
	delay := time.Duration(float64(base) * factor)
	if delay <= 0 {
		return time.Millisecond
	}
	return delay
}
