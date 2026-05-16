package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type MailFlowDirection string

const (
	MailFlowDirectionInbound  MailFlowDirection = "inbound"
	MailFlowDirectionOutbound MailFlowDirection = "outbound"
)

type MailFlowStatus string

const (
	MailFlowStatusReceived  MailFlowStatus = "received"
	MailFlowStatusDelivered MailFlowStatus = "delivered"
	MailFlowStatusFailed    MailFlowStatus = "failed"
	MailFlowStatusBounced   MailFlowStatus = "bounced"
	MailFlowStatusFiltered  MailFlowStatus = "filtered"
	MailFlowStatusRejected  MailFlowStatus = "rejected"
	MailFlowStatusPending   MailFlowStatus = "pending"
)

type MailFlowLogView struct {
	ID             string     `json:"id"`
	Direction      string     `json:"direction"`
	CompanyID      string     `json:"company_id,omitempty"`
	DomainID       string     `json:"domain_id,omitempty"`
	UserID         string     `json:"user_id,omitempty"`
	MessageID      string     `json:"message_id,omitempty"`
	RFCMessageID   string     `json:"rfc_message_id,omitempty"`
	FromAddr       string     `json:"from_addr,omitempty"`
	FromName       string     `json:"from_name,omitempty"`
	ToAddrs        []string   `json:"to_addrs"`
	Subject        string     `json:"subject,omitempty"`
	FlowStatus     string     `json:"flow_status"`
	EnhancedStatus string     `json:"enhanced_status,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	SpamScore      *float64   `json:"spam_score,omitempty"`
	DKIMResult     string     `json:"dkim_result,omitempty"`
	SPFResult      string     `json:"spf_result,omitempty"`
	DMARCResult    string     `json:"dmarc_result,omitempty"`
	Transport      string     `json:"transport,omitempty"`
	Farm           string     `json:"farm,omitempty"`
	Size           int64      `json:"size"`
	ReceivedAt     *time.Time `json:"received_at,omitempty"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
	InReplyTo      string     `json:"in_reply_to,omitempty"`
	References     string     `json:"references,omitempty"`
	ThreadID       string     `json:"thread_id,omitempty"`
	IPAddress      string     `json:"ip_address,omitempty"`
	MailFrom       string     `json:"mail_from,omitempty"`
	RcptTo         string     `json:"rcpt_to,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type MailFlowLogListRequest struct {
	Limit        int
	Direction    string
	CompanyID    string
	DomainID     string
	UserID       string
	MessageID    string
	RFCMessageID string
	FromAddr     string
	ToAddr       string
	Subject      string
	FlowStatus   string
	Since        time.Time
	Until        time.Time
}

type MailFlowLogStatsRequest struct {
	Direction string
	CompanyID string
	DomainID  string
	UserID    string
	Since     time.Time
	Until     time.Time
}

type MailFlowLogStatsView struct {
	TotalMessages    int64   `json:"total_messages"`
	UniqueSenders    int64   `json:"unique_senders"`
	UniqueDomains    int64   `json:"unique_domains"`
	TotalSizeBytes   int64   `json:"total_size_bytes"`
	AverageSizeBytes float64 `json:"average_size_bytes"`
	MaxSizeBytes     int64   `json:"max_size_bytes"`
	Delivered        int64   `json:"delivered"`
	Failed           int64   `json:"failed"`
	Bounced          int64   `json:"bounced"`
	Filtered         int64   `json:"filtered"`
	Rejected         int64   `json:"rejected"`
	DeliveryRate     float64 `json:"delivery_rate"`
}

type MailFlowLogDailyStatsView struct {
	Date             time.Time `json:"date"`
	InboundMessages  int64     `json:"inbound_messages"`
	OutboundMessages int64     `json:"outbound_messages"`
	InboundSize      int64     `json:"inbound_size_bytes"`
	OutboundSize     int64     `json:"outbound_size_bytes"`
	Delivered        int64     `json:"delivered"`
	Failed           int64     `json:"failed"`
	Bounced          int64     `json:"bounced"`
	Filtered         int64     `json:"filtered"`
	Rejected         int64     `json:"rejected"`
}

type MailFlowLogDailyStatsRequest struct {
	Direction string
	CompanyID string
	DomainID  string
	UserID    string
	Since     time.Time
	Until     time.Time
}

func (r *Repository) ListMailFlowLogs(ctx context.Context, req MailFlowLogListRequest) ([]MailFlowLogView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req = normalizeMailFlowLogListRequest(req)

	query := `
SELECT
  id::text,
  direction,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  COALESCE(user_id::text, ''),
  COALESCE(message_id::text, ''),
  rfc_message_id,
  from_addr,
  from_name,
  to_addrs,
  subject,
  flow_status,
  enhanced_status,
  error_message,
  spam_score,
  dkim_result,
  spf_result,
  dmarc_result,
  transport,
  farm,
  size,
  received_at,
  processed_at,
  in_reply_to,
  "references",
  COALESCE(thread_id::text, ''),
  COALESCE(ip_address::text, ''),
  mail_from,
  rcpt_to,
  created_at
FROM mail_flow_logs`
	var conditions []string
	var args []any
	if req.Direction != "" {
		args = append(args, req.Direction)
		conditions = append(conditions, fmt.Sprintf("direction = $%d", len(args)))
	}
	if req.CompanyID != "" {
		args = append(args, req.CompanyID)
		conditions = append(conditions, fmt.Sprintf("company_id::text = $%d", len(args)))
	}
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		conditions = append(conditions, fmt.Sprintf("domain_id::text = $%d", len(args)))
	}
	if req.UserID != "" {
		args = append(args, req.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id::text = $%d", len(args)))
	}
	if req.MessageID != "" {
		args = append(args, req.MessageID)
		conditions = append(conditions, fmt.Sprintf("message_id::text = $%d", len(args)))
	}
	if req.RFCMessageID != "" {
		args = append(args, req.RFCMessageID)
		conditions = append(conditions, fmt.Sprintf("rfc_message_id = $%d", len(args)))
	}
	if req.FromAddr != "" {
		args = append(args, req.FromAddr)
		conditions = append(conditions, fmt.Sprintf("from_addr ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if req.ToAddr != "" {
		args = append(args, req.ToAddr)
		conditions = append(conditions, fmt.Sprintf("rcpt_to ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if req.Subject != "" {
		args = append(args, req.Subject)
		conditions = append(conditions, fmt.Sprintf("subject ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if req.FlowStatus != "" {
		args = append(args, req.FlowStatus)
		conditions = append(conditions, fmt.Sprintf("flow_status = $%d", len(args)))
	}
	if !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !req.Until.IsZero() {
		args = append(args, req.Until.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	args = append(args, req.Limit)
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mail flow logs: %w", err)
	}
	defer rows.Close()

	var logs []MailFlowLogView
	for rows.Next() {
		log, err := scanMailFlowLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mail flow logs: %w", err)
	}
	return logs, nil
}

func (r *Repository) GetMailFlowLog(ctx context.Context, id string) (MailFlowLogView, error) {
	if r.db == nil {
		return MailFlowLogView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return MailFlowLogView{}, fmt.Errorf("mail flow log id is required")
	}
	const query = `
SELECT
  id::text,
  direction,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  COALESCE(user_id::text, ''),
  COALESCE(message_id::text, ''),
  rfc_message_id,
  from_addr,
  from_name,
  to_addrs,
  subject,
  flow_status,
  enhanced_status,
  error_message,
  spam_score,
  dkim_result,
  spf_result,
  dmarc_result,
  transport,
  farm,
  size,
  received_at,
  processed_at,
  in_reply_to,
  "references",
  COALESCE(thread_id::text, ''),
  COALESCE(ip_address::text, ''),
  mail_from,
  rcpt_to,
  created_at
FROM mail_flow_logs
WHERE id = $1`
	log, err := scanMailFlowLog(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return MailFlowLogView{}, fmt.Errorf("get mail flow log: %w", err)
	}
	return log, nil
}

func (r *Repository) GetMailFlowLogStats(ctx context.Context, req MailFlowLogStatsRequest) (MailFlowLogStatsView, error) {
	if r.db == nil {
		return MailFlowLogStatsView{}, fmt.Errorf("database handle is required")
	}
	req = normalizeMailFlowLogStatsRequest(req)

	var conditions []string
	var args []any
	if req.Direction != "" {
		args = append(args, req.Direction)
		conditions = append(conditions, fmt.Sprintf("direction = $%d", len(args)))
	}
	if req.CompanyID != "" {
		args = append(args, req.CompanyID)
		conditions = append(conditions, fmt.Sprintf("company_id::text = $%d", len(args)))
	}
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		conditions = append(conditions, fmt.Sprintf("domain_id::text = $%d", len(args)))
	}
	if req.UserID != "" {
		args = append(args, req.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id::text = $%d", len(args)))
	}
	if !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !req.Until.IsZero() {
		args = append(args, req.Until.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}

	query := fmt.Sprintf(`
SELECT
  COUNT(DISTINCT message_id) AS total_messages,
  COUNT(DISTINCT from_addr) AS unique_senders,
  COUNT(DISTINCT COALESCE(NULLIF(split_part(from_addr, '@', 2), ''), mail_from)) AS unique_domains,
  COALESCE(SUM(size), 0) AS total_size_bytes,
  COALESCE(AVG(size), 0) AS average_size_bytes,
  COALESCE(MAX(size), 0) AS max_size_bytes,
  COUNT(*) FILTER (WHERE flow_status = 'delivered') AS delivered,
  COUNT(*) FILTER (WHERE flow_status = 'failed') AS failed,
  COUNT(*) FILTER (WHERE flow_status = 'bounced') AS bounced,
  COUNT(*) FILTER (WHERE flow_status = 'filtered') AS filtered,
  COUNT(*) FILTER (WHERE flow_status = 'rejected') AS rejected
FROM mail_flow_logs%s`, where)

	row := r.db.QueryRowContext(ctx, query, args...)
	var stats MailFlowLogStatsView
	var uniqueDomains sql.NullInt64
	var totalSize, maxSize sql.NullInt64
	var avgSize sql.NullFloat64
	if err := row.Scan(
		&stats.TotalMessages,
		&stats.UniqueSenders,
		&uniqueDomains,
		&totalSize,
		&avgSize,
		&maxSize,
		&stats.Delivered,
		&stats.Failed,
		&stats.Bounced,
		&stats.Filtered,
		&stats.Rejected,
	); err != nil {
		return MailFlowLogStatsView{}, fmt.Errorf("get mail flow log stats: %w", err)
	}
	if uniqueDomains.Valid {
		stats.UniqueDomains = uniqueDomains.Int64
	}
	if totalSize.Valid {
		stats.TotalSizeBytes = totalSize.Int64
	}
	if avgSize.Valid {
		stats.AverageSizeBytes = avgSize.Float64
	}
	if maxSize.Valid {
		stats.MaxSizeBytes = maxSize.Int64
	}
	if stats.TotalMessages > 0 {
		stats.DeliveryRate = float64(stats.Delivered) / float64(stats.TotalMessages)
	}
	return stats, nil
}

func (r *Repository) GetMailFlowLogDailyStats(ctx context.Context, req MailFlowLogDailyStatsRequest) ([]MailFlowLogDailyStatsView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req = normalizeMailFlowLogDailyStatsRequest(req)

	var conditions []string
	var args []any
	hasBounds := !req.Since.IsZero() && !req.Until.IsZero()
	if hasBounds {
		args = append(args, req.Since.UTC(), req.Until.UTC())
		conditions = append(conditions, "created_at >= $1", "created_at <= $2")
	}
	if req.Direction != "" {
		args = append(args, req.Direction)
		conditions = append(conditions, fmt.Sprintf("direction = $%d", len(args)))
	}
	if req.CompanyID != "" {
		args = append(args, req.CompanyID)
		conditions = append(conditions, fmt.Sprintf("company_id::text = $%d", len(args)))
	}
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		conditions = append(conditions, fmt.Sprintf("domain_id::text = $%d", len(args)))
	}
	if req.UserID != "" {
		args = append(args, req.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id::text = $%d", len(args)))
	}
	if !hasBounds && !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !hasBounds && !req.Until.IsZero() {
		args = append(args, req.Until.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	var query string
	if hasBounds {
		where := ""
		if len(conditions) > 0 {
			where = "\nWHERE " + strings.Join(conditions, "\n  AND ")
		}
		query = fmt.Sprintf(`
WITH days AS (
  SELECT generate_series(
    DATE_TRUNC('day', $1::timestamptz AT TIME ZONE 'UTC'),
    DATE_TRUNC('day', $2::timestamptz AT TIME ZONE 'UTC'),
    INTERVAL '1 day'
  )::date AS date
),
daily AS (
  SELECT
    DATE(created_at AT TIME ZONE 'UTC') AS date,
    COUNT(*) FILTER (WHERE direction = 'inbound') AS inbound_messages,
    COUNT(*) FILTER (WHERE direction = 'outbound') AS outbound_messages,
    COALESCE(SUM(size) FILTER (WHERE direction = 'inbound'), 0) AS inbound_size,
    COALESCE(SUM(size) FILTER (WHERE direction = 'outbound'), 0) AS outbound_size,
    COUNT(*) FILTER (WHERE flow_status = 'delivered') AS delivered,
    COUNT(*) FILTER (WHERE flow_status = 'failed') AS failed,
    COUNT(*) FILTER (WHERE flow_status = 'bounced') AS bounced,
    COUNT(*) FILTER (WHERE flow_status = 'filtered') AS filtered,
    COUNT(*) FILTER (WHERE flow_status = 'rejected') AS rejected
  FROM mail_flow_logs%s
  GROUP BY DATE(created_at AT TIME ZONE 'UTC')
)
SELECT
  days.date,
  COALESCE(daily.inbound_messages, 0) AS inbound_messages,
  COALESCE(daily.outbound_messages, 0) AS outbound_messages,
  COALESCE(daily.inbound_size, 0) AS inbound_size,
  COALESCE(daily.outbound_size, 0) AS outbound_size,
  COALESCE(daily.delivered, 0) AS delivered,
  COALESCE(daily.failed, 0) AS failed,
  COALESCE(daily.bounced, 0) AS bounced,
  COALESCE(daily.filtered, 0) AS filtered,
  COALESCE(daily.rejected, 0) AS rejected
FROM days
LEFT JOIN daily USING (date)
ORDER BY days.date DESC`, where)
	} else {
		where := ""
		if len(conditions) > 0 {
			where = "\nWHERE " + strings.Join(conditions, "\n  AND ")
		}
		query = fmt.Sprintf(`
SELECT
  DATE(created_at AT TIME ZONE 'UTC') AS date,
  COUNT(*) FILTER (WHERE direction = 'inbound') AS inbound_messages,
  COUNT(*) FILTER (WHERE direction = 'outbound') AS outbound_messages,
  COALESCE(SUM(size) FILTER (WHERE direction = 'inbound'), 0) AS inbound_size,
  COALESCE(SUM(size) FILTER (WHERE direction = 'outbound'), 0) AS outbound_size,
  COUNT(*) FILTER (WHERE flow_status = 'delivered') AS delivered,
  COUNT(*) FILTER (WHERE flow_status = 'failed') AS failed,
  COUNT(*) FILTER (WHERE flow_status = 'bounced') AS bounced,
  COUNT(*) FILTER (WHERE flow_status = 'filtered') AS filtered,
  COUNT(*) FILTER (WHERE flow_status = 'rejected') AS rejected
FROM mail_flow_logs%s
GROUP BY DATE(created_at AT TIME ZONE 'UTC')
ORDER BY date DESC`, where)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get mail flow log daily stats: %w", err)
	}
	defer rows.Close()

	var stats []MailFlowLogDailyStatsView
	for rows.Next() {
		var s MailFlowLogDailyStatsView
		var date sql.NullTime
		if err := rows.Scan(
			&date,
			&s.InboundMessages,
			&s.OutboundMessages,
			&s.InboundSize,
			&s.OutboundSize,
			&s.Delivered,
			&s.Failed,
			&s.Bounced,
			&s.Filtered,
			&s.Rejected,
		); err != nil {
			return nil, fmt.Errorf("scan mail flow log daily stats: %w", err)
		}
		if date.Valid {
			s.Date = date.Time.UTC()
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mail flow log daily stats: %w", err)
	}
	return stats, nil
}

func normalizeMailFlowLogDailyStatsRequest(req MailFlowLogDailyStatsRequest) MailFlowLogDailyStatsRequest {
	req.Direction = strings.TrimSpace(req.Direction)
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.DomainID = strings.TrimSpace(req.DomainID)
	req.UserID = strings.TrimSpace(req.UserID)
	return req
}

func normalizeMailFlowLogListRequest(req MailFlowLogListRequest) MailFlowLogListRequest {
	req.Limit = normalizeLimit(req.Limit)
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 500 {
		req.Limit = 500
	}
	req.Direction = strings.TrimSpace(req.Direction)
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.DomainID = strings.TrimSpace(req.DomainID)
	req.UserID = strings.TrimSpace(req.UserID)
	req.MessageID = strings.TrimSpace(req.MessageID)
	req.RFCMessageID = strings.TrimSpace(req.RFCMessageID)
	req.FromAddr = strings.TrimSpace(req.FromAddr)
	req.ToAddr = strings.TrimSpace(req.ToAddr)
	req.Subject = strings.TrimSpace(req.Subject)
	req.FlowStatus = strings.TrimSpace(req.FlowStatus)
	return req
}

func normalizeMailFlowLogStatsRequest(req MailFlowLogStatsRequest) MailFlowLogStatsRequest {
	req.Direction = strings.TrimSpace(req.Direction)
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.DomainID = strings.TrimSpace(req.DomainID)
	req.UserID = strings.TrimSpace(req.UserID)
	return req
}

type mailFlowLogScanner interface {
	Scan(dest ...any) error
}

func scanMailFlowLog(scanner mailFlowLogScanner) (MailFlowLogView, error) {
	var log MailFlowLogView
	var toAddrsRaw []byte
	var spamScore sql.NullFloat64
	var receivedAt, processedAt sql.NullTime
	var threadID sql.NullString
	var ipAddress sql.NullString
	var rcptTo sql.NullString

	if err := scanner.Scan(
		&log.ID,
		&log.Direction,
		&log.CompanyID,
		&log.DomainID,
		&log.UserID,
		&log.MessageID,
		&log.RFCMessageID,
		&log.FromAddr,
		&log.FromName,
		&toAddrsRaw,
		&log.Subject,
		&log.FlowStatus,
		&log.EnhancedStatus,
		&log.ErrorMessage,
		&spamScore,
		&log.DKIMResult,
		&log.SPFResult,
		&log.DMARCResult,
		&log.Transport,
		&log.Farm,
		&log.Size,
		&receivedAt,
		&processedAt,
		&log.InReplyTo,
		&log.References,
		&threadID,
		&ipAddress,
		&log.MailFrom,
		&rcptTo,
		&log.CreatedAt,
	); err != nil {
		return MailFlowLogView{}, fmt.Errorf("scan mail flow log: %w", err)
	}
	if spamScore.Valid {
		log.SpamScore = &spamScore.Float64
	}
	if receivedAt.Valid {
		log.ReceivedAt = &receivedAt.Time
	}
	if processedAt.Valid {
		log.ProcessedAt = &processedAt.Time
	}
	if threadID.Valid {
		log.ThreadID = threadID.String
	}
	if ipAddress.Valid {
		log.IPAddress = ipAddress.String
	}
	if rcptTo.Valid {
		log.RcptTo = rcptTo.String
	}
	if len(toAddrsRaw) > 0 {
		if err := json.Unmarshal(toAddrsRaw, &log.ToAddrs); err != nil {
			log.ToAddrs = []string{}
		}
	}
	if log.ToAddrs == nil {
		log.ToAddrs = []string{}
	}
	log.CreatedAt = log.CreatedAt.UTC()
	return log, nil
}

type MailFlowLogWriter struct {
	db *sql.DB
}

func NewMailFlowLogWriter(db *sql.DB) *MailFlowLogWriter {
	return &MailFlowLogWriter{db: db}
}

func (w *MailFlowLogWriter) InsertInbound(ctx context.Context, log MailFlowLogEntry) error {
	if w.db == nil {
		return fmt.Errorf("database handle is required")
	}
	return w.insert(ctx, log, "inbound")
}

func (w *MailFlowLogWriter) InsertOutbound(ctx context.Context, log MailFlowLogEntry) error {
	if w.db == nil {
		return fmt.Errorf("database handle is required")
	}
	return w.insert(ctx, log, "outbound")
}

func (w *MailFlowLogWriter) insert(ctx context.Context, log MailFlowLogEntry, direction string) error {
	toAddrs, err := json.Marshal(log.ToAddrs)
	if err != nil {
		return fmt.Errorf("marshal to_addrs: %w", err)
	}

	const query = `
INSERT INTO mail_flow_logs (
  direction, company_id, domain_id, user_id,
  message_id, rfc_message_id, from_addr, from_name, to_addrs, subject,
  flow_status, enhanced_status, error_message,
  spam_score, dkim_result, spf_result, dmarc_result,
  transport, farm, size,
  received_at, processed_at,
  in_reply_to, "references", thread_id,
  ip_address, mail_from, rcpt_to
) VALUES (
  $1, $2, $3, $4,
  $5, $6, $7, $8, $9, $10,
  $11, $12, $13,
  $14, $15, $16, $17,
  $18, $19, $20,
  $21, $22,
  $23, $24, $25,
  $26, $27, $28
)`

	_, err = w.db.ExecContext(ctx, query,
		direction,
		nullString(log.CompanyID),
		nullString(log.DomainID),
		nullString(log.UserID),
		nullString(log.MessageID),
		log.RFCMessageID,
		log.FromAddr,
		log.FromName,
		toAddrs,
		log.Subject,
		log.FlowStatus,
		log.EnhancedStatus,
		log.ErrorMessage,
		log.SpamScore,
		log.DKIMResult,
		log.SPFResult,
		log.DMARCResult,
		log.Transport,
		log.Farm,
		log.Size,
		log.ReceivedAt,
		log.ProcessedAt,
		log.InReplyTo,
		log.References,
		nullString(log.ThreadID),
		nullString(log.IPAddress),
		log.MailFrom,
		log.RcptTo,
	)
	if err != nil {
		return fmt.Errorf("insert mail flow log: %w", err)
	}
	return nil
}

type MailFlowLogEntry struct {
	CompanyID      string
	DomainID       string
	UserID         string
	MessageID      string
	RFCMessageID   string
	FromAddr       string
	FromName       string
	ToAddrs        []string
	Subject        string
	FlowStatus     string
	EnhancedStatus string
	ErrorMessage   string
	SpamScore      *float64
	DKIMResult     string
	SPFResult      string
	DMARCResult    string
	Transport      string
	Farm           string
	Size           int64
	ReceivedAt     *time.Time
	ProcessedAt    *time.Time
	InReplyTo      string
	References     string
	ThreadID       string
	IPAddress      string
	MailFrom       string
	RcptTo         string
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
