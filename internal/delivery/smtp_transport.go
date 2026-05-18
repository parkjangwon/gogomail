package delivery

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/dane"
	"github.com/gogomail/gogomail/internal/mtasts"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/tlsrpt"
)

type MXResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
}

type DeliveryTLSMode string

const (
	DeliveryTLSOpportunistic DeliveryTLSMode = "opportunistic"
	DeliveryTLSRequire       DeliveryTLSMode = "require"
	DeliveryTLSDisable       DeliveryTLSMode = "disable"
)

type DirectSMTPTransport struct {
	Resolver     MXResolver
	Router       Router
	Timeout      time.Duration
	Hello        string
	TLSMode      DeliveryTLSMode
	TLSConfig    *tls.Config
	Transformers TransformChain
	// MaxRecipientsPerBatch caps one SMTP transaction's RCPT set. Zero means unlimited.
	MaxRecipientsPerBatch int
	daneValidator         *dane.Validator
	mtastsClient          *mtasts.Client
	tlsrptCollector       *tlsrpt.Collector
	deliverHost           func(context.Context, Job, Route, string, []outbound.Address) error
	pool                  *SMTPConnectionPool
	poolOnce              sync.Once // Protect pool initialization
}

func NewDirectSMTPTransport() *DirectSMTPTransport {
	dnsResolver := &dane.NetResolver{Resolver: net.DefaultResolver}
	return &DirectSMTPTransport{
		Resolver:        net.DefaultResolver,
		Timeout:         30 * time.Second,
		Hello:           "localhost",
		TLSMode:         DeliveryTLSOpportunistic,
		daneValidator:   dane.NewValidator(dnsResolver),
		mtastsClient:    mtasts.NewClient(),
		tlsrptCollector: tlsrpt.NewCollector("localhost"), // Domain placeholder
		pool:            NewSMTPConnectionPool(4, 30*time.Second, 5*time.Minute),
	}
}

func (t *DirectSMTPTransport) Deliver(ctx context.Context, job Job) error {
	batches := PlanRecipientBatchesWithLimit(job.Recipients(), t.MaxRecipientsPerBatch)
	if len(batches) == 0 {
		return &SMTPStatusError{
			Op:      "recipient",
			Code:    554,
			Message: "no deliverable recipients",
		}
	}
	delivered := make([]outbound.Address, 0, len(job.Recipients()))
	failures := make([]RecipientDeliveryError, 0)
	for _, batch := range batches {
		if err := t.deliverDomain(ctx, job, batch.Domain, batch.Recipients); err != nil {
			var partial *PartialDeliveryError
			if errors.As(err, &partial) {
				delivered = append(delivered, partial.Delivered...)
				failures = append(failures, partial.Failed...)
				continue
			}
			for _, recipient := range batch.Recipients {
				failures = append(failures, RecipientDeliveryError{Recipient: recipient, Err: err})
			}
			continue
		}
		delivered = append(delivered, batch.Recipients...)
	}
	if len(failures) > 0 {
		return &PartialDeliveryError{Delivered: delivered, Failed: failures}
	}
	return nil
}

func (t *DirectSMTPTransport) deliverDomain(ctx context.Context, job Job, domain string, recipients []outbound.Address) error {
	route, err := t.route(ctx, job, domain)
	if err != nil {
		return err
	}
	hosts := route.Hosts
	if len(hosts) == 0 {
		hosts, err = t.mxHosts(ctx, domain)
		if err != nil {
			return err
		}
		route.Hosts = hosts
	}
	errs := make([]error, 0, len(hosts))
	for _, host := range hosts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := t.deliverHostFunc()(ctx, job, route, host, recipients); err != nil {
			var partial *PartialDeliveryError
			if errors.As(err, &partial) {
				if len(partial.Delivered) == 0 && len(partial.Failed) > 0 && len(partial.TemporaryFailures()) == len(partial.Failed) {
					errs = append(errs, err)
					continue
				}
				return err
			}
			if IsPermanentFailure(err) {
				return err
			}
			errs = append(errs, err)
			continue
		}
		return nil
	}
	return fmt.Errorf("deliver to %s via %d mx host(s): %w", domain, len(hosts), errors.Join(errs...))
}

func (t *DirectSMTPTransport) deliverHostFunc() func(context.Context, Job, Route, string, []outbound.Address) error {
	if t.deliverHost != nil {
		return t.deliverHost
	}
	return t.deliverHostDefault
}

func (t *DirectSMTPTransport) ensurePool() {
	t.poolOnce.Do(func() {
		if t.pool == nil {
			t.pool = NewSMTPConnectionPool(4, 30*time.Second, 5*time.Minute)
		}
	})
}

func (t *DirectSMTPTransport) deliverHostDefault(ctx context.Context, job Job, route Route, host string, recipients []outbound.Address) error {
	// Check MTA-STS policy before connecting
	if err := t.checkMTASTSPolicy(ctx, route.Domain, host); err != nil {
		return fmt.Errorf("mta-sts: %w", err)
	}

	t.ensurePool()

	// Create pool key for this route
	poolKey := SMTPConnPoolKey{
		Host:        host,
		Port:        normalizeRoutePort(route.Port),
		ImplicitTLS: route.ImplicitTLS,
		AuthUser:    route.Auth.Username,
	}

	// Try to get existing connection from pool
	pooledConn, _ := t.pool.Get(ctx, poolKey)
	var client *smtp.Client
	var conn net.Conn
	var pooledConnUsed bool

	if pooledConn != nil {
		client = pooledConn.Client
		conn = pooledConn.Conn
		pooledConnUsed = true
	} else {
		// Pool miss: create new connection
		timeout := t.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		dialer := net.Dialer{Timeout: timeout}
		addr := net.JoinHostPort(host, fmt.Sprintf("%d", poolKey.Port))
		var err error
		if route.ImplicitTLS {
			tlsDialer := tls.Dialer{
				NetDialer: &dialer,
				Config:    t.deliveryTLSConfig(host),
			}
			conn, err = tlsDialer.DialContext(ctx, "tcp", addr)
		} else {
			conn, err = dialer.DialContext(ctx, "tcp", addr)
		}
		if err != nil {
			return fmt.Errorf("dial mx %s for %s: %w", host, route.Domain, err)
		}
		if deadline := deliveryDeadline(ctx, timeout, time.Now()); !deadline.IsZero() {
			if err := conn.SetDeadline(deadline); err != nil {
				conn.Close()
				return fmt.Errorf("set smtp session deadline for %s: %w", host, err)
			}
		}

		client, err = smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("create smtp client for %s: %w", host, err)
		}

		hello := strings.TrimSpace(t.Hello)
		if route.Hello != "" {
			hello = route.Hello
		}
		if hello == "" {
			hello = "localhost"
		}
		if err := client.Hello(hello); err != nil {
			client.Close()
			return WrapSMTPError("hello", err)
		}
		if !route.ImplicitTLS {
			if err := t.startTLS(ctx, client, host, route.TLSMode); err != nil {
				client.Close()
				return WrapSMTPError("starttls", err)
			}
		}

		// Check DANE policy after TLS is established
		if err := t.checkDANEPolicy(ctx, route.Domain, host, 25, client); err != nil {
			client.Close()
			return fmt.Errorf("dane: %w", err)
		}
		if routeRequiresAuth(route) {
			if err := client.Auth(smtp.PlainAuth(route.Auth.Identity, route.Auth.Username, route.Auth.Password, host)); err != nil {
				client.Close()
				return WrapSMTPError("auth", err)
			}
		}
	}

	if err := smtpMail(client, job); err != nil {
		if pooledConnUsed {
			_ = conn.Close()
			_ = client.Close()
		} else {
			_ = client.Close()
		}
		return WrapSMTPError("mail", err)
	}
	acceptedRecipients, recipientFailures := pipelineRCPTs(client, job, recipients)
	if len(acceptedRecipients) == 0 {
		if pooledConnUsed {
			_ = conn.Close()
			_ = client.Close()
		} else {
			_ = client.Close()
		}
		return recipientsRejectedResult(recipientFailures)
	}

	writer, err := client.Data()
	if err != nil {
		if pooledConnUsed {
			_ = conn.Close()
			_ = client.Close()
		} else {
			_ = client.Close()
		}
		return WrapSMTPError("data", err)
	}
	message, err := t.openMessage(ctx, job)
	if err != nil {
		_ = writer.Close()
		if pooledConnUsed {
			_ = conn.Close()
			_ = client.Close()
		} else {
			_ = client.Close()
		}
		return fmt.Errorf("open queued message: %w", err)
	}

	// Inject RFC 5321 Received header
	receivedHeader := fmt.Sprintf("Received: from %s by %s with SMTP\n\t%s\n",
		t.Hello, host, time.Now().Format(time.RFC1123Z))
	message = newHeaderInjector(receivedHeader, message)
	_, copyErr := io.Copy(writer, message)
	closeMessageErr := message.Close()
	closeDataErr := writer.Close()
	if copyErr != nil {
		if pooledConnUsed {
			_ = conn.Close()
			_ = client.Close()
		} else {
			_ = client.Close()
		}
		return fmt.Errorf("write smtp data: %w", copyErr)
	}
	if closeMessageErr != nil {
		if pooledConnUsed {
			_ = conn.Close()
			_ = client.Close()
		} else {
			_ = client.Close()
		}
		return fmt.Errorf("close queued message: %w", closeMessageErr)
	}
	if closeDataErr != nil {
		if pooledConnUsed {
			_ = conn.Close()
			_ = client.Close()
		} else {
			_ = client.Close()
		}
		return WrapSMTPError("data", closeDataErr)
	}

	// Successful delivery: return connection to pool for reuse (no Quit yet)
	_ = t.pool.Put(poolKey, &PooledSMTPConn{
		Key:      poolKey,
		Client:   client,
		Conn:     conn,
		LastUsed: time.Now(),
	})
	return dataAcceptedResult(acceptedRecipients, recipientFailures)
}

func smtpMail(client *smtp.Client, job Job) error {
	if jobNeedsUTF8(job) && !smtpClientSupports(client, "SMTPUTF8") {
		return &SMTPStatusError{Op: "mail", Code: 553, Message: "5.6.7 SMTPUTF8 required but not advertised"}
	}
	needsExtensions := shouldSendOutboundDSNMailOptions(job) ||
		jobNeedsUTF8(job)
	if !needsExtensions {
		return client.Mail(job.From.Email)
	}
	parts := []string{"MAIL FROM:<" + job.From.Email + ">"}
	if smtpClientSupports(client, "8BITMIME") {
		parts = append(parts, "BODY=8BITMIME")
	}
	if jobNeedsUTF8(job) && smtpClientSupports(client, "SMTPUTF8") {
		parts = append(parts, "SMTPUTF8")
	}
	if shouldSendOutboundDSNMailOptions(job) && smtpClientSupports(client, "DSN") {
		if job.DSN.Return != "" {
			parts = append(parts, "RET="+job.DSN.Return)
		}
		if job.DSN.EnvelopeID != "" {
			parts = append(parts, "ENVID="+job.DSN.EnvelopeID)
		}
	}
	return smtpCommand(client, 250, strings.Join(parts, " "))
}

func shouldSendOutboundDSNMailOptions(job Job) bool {
	return job.From.Email != "" && (job.DSN.Return != "" || job.DSN.EnvelopeID != "")
}

func jobNeedsUTF8(job Job) bool {
	if containsNonASCIIByte(job.From.Email) {
		return true
	}
	for _, r := range job.Recipients() {
		if containsNonASCIIByte(r.Email) {
			return true
		}
	}
	return false
}

func containsNonASCIIByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return true
		}
	}
	return false
}

func smtpRcpt(client *smtp.Client, job Job, recipient outbound.Address) error {
	if containsNonASCIIByte(recipient.Email) && !smtpClientSupports(client, "SMTPUTF8") {
		return &SMTPStatusError{Op: "rcpt", Code: 553, Message: "5.6.7 SMTPUTF8 required but not advertised"}
	}
	options := dsnOptionsForRecipient(job.DSN.Recipients, recipient.Email)
	if !shouldSendOutboundDSNRcptOptions(job) || !smtpClientSupports(client, "DSN") || len(options) == 0 {
		return client.Rcpt(recipient.Email)
	}
	parts := []string{"RCPT TO:<" + recipient.Email + ">"}
	parts = append(parts, options...)
	return smtpCommand(client, 250, strings.Join(parts, " "))
}

func shouldSendOutboundDSNRcptOptions(job Job) bool {
	return job.From.Email != ""
}

func dsnOptionsForRecipient(recipients []DSNRecipientOptions, address string) []string {
	normalized := strings.ToLower(strings.TrimSpace(address))
	for _, recipient := range recipients {
		if strings.ToLower(strings.TrimSpace(recipient.Address)) != normalized {
			continue
		}
		options := make([]string, 0, 2)
		if len(recipient.Notify) > 0 {
			options = append(options, "NOTIFY="+strings.Join(recipient.Notify, ","))
		}
		if recipient.OriginalRecipient != "" {
			options = append(options, "ORCPT="+recipient.OriginalRecipient)
		}
		return options
	}
	return nil
}

func smtpClientSupports(client *smtp.Client, extension string) bool {
	ok, _ := client.Extension(extension)
	return ok
}

func smtpCommand(client *smtp.Client, expectCode int, command string) error {
	if strings.ContainsAny(command, "\r\n") {
		return fmt.Errorf("smtp command contains newline")
	}
	id, err := client.Text.Cmd("%s", command)
	if err != nil {
		return err
	}
	client.Text.StartResponse(id)
	defer client.Text.EndResponse(id)
	_, _, err = client.Text.ReadResponse(expectCode)
	return err
}

func acceptRecipients(recipients []outbound.Address, rcpt func(outbound.Address) error) ([]outbound.Address, []RecipientDeliveryError) {
	accepted := make([]outbound.Address, 0, len(recipients))
	failures := make([]RecipientDeliveryError, 0)
	for _, recipient := range recipients {
		if err := rcpt(recipient); err != nil {
			failures = append(failures, RecipientDeliveryError{Recipient: recipient, Err: err})
			continue
		}
		accepted = append(accepted, recipient)
	}
	return accepted, failures
}

// pipelineRCPTs sends multiple RCPT commands via pipelining (RFC 2920)
// Returns accepted recipients and failures, enabling bulk recipient handling
func pipelineRCPTs(client *smtp.Client, job Job, recipients []outbound.Address) ([]outbound.Address, []RecipientDeliveryError) {
	if len(recipients) == 0 {
		return []outbound.Address{}, []RecipientDeliveryError{}
	}

	// Send all RCPT commands without waiting for responses
	rcptCmds := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		if containsNonASCIIByte(recipient.Email) && !smtpClientSupports(client, "SMTPUTF8") {
			return []outbound.Address{}, []RecipientDeliveryError{
				{Recipient: recipient, Err: &SMTPStatusError{Op: "rcpt", Code: 553, Message: "5.6.7 SMTPUTF8 required but not advertised"}},
			}
		}

		options := dsnOptionsForRecipient(job.DSN.Recipients, recipient.Email)
		cmd := "RCPT TO:<" + recipient.Email + ">"
		if shouldSendOutboundDSNRcptOptions(job) && smtpClientSupports(client, "DSN") && len(options) > 0 {
			cmd += " " + strings.Join(options, " ")
		}
		rcptCmds = append(rcptCmds, cmd)
	}

	// Pipeline: send all commands at once
	cmdIDs := make([]uint, 0, len(rcptCmds))
	for _, cmd := range rcptCmds {
		if strings.ContainsAny(cmd, "\r\n") {
			return []outbound.Address{}, []RecipientDeliveryError{
				{Recipient: recipients[0], Err: fmt.Errorf("smtp command contains newline")},
			}
		}
		id, err := client.Text.Cmd("%s", cmd)
		if err != nil {
			return []outbound.Address{}, []RecipientDeliveryError{
				{Recipient: recipients[0], Err: WrapSMTPError("rcpt", err)},
			}
		}
		cmdIDs = append(cmdIDs, id)
	}

	// Read all responses
	accepted := make([]outbound.Address, 0, len(recipients))
	failures := make([]RecipientDeliveryError, 0)

	for i, id := range cmdIDs {
		client.Text.StartResponse(id)
		code, _, err := client.Text.ReadResponse(250)
		client.Text.EndResponse(id)

		if err != nil {
			failures = append(failures, RecipientDeliveryError{Recipient: recipients[i], Err: WrapSMTPError("rcpt", err)})
			continue
		}
		if code != 250 {
			failures = append(failures, RecipientDeliveryError{Recipient: recipients[i], Err: &SMTPStatusError{Op: "rcpt", Code: code, Message: fmt.Sprintf("%d recipient rejected", code)}})
			continue
		}
		accepted = append(accepted, recipients[i])
	}

	return accepted, failures
}

func recipientFailureErrors(failures []RecipientDeliveryError) []error {
	errs := make([]error, 0, len(failures))
	for _, failure := range failures {
		errs = append(errs, failure)
	}
	return errs
}

func recipientsRejectedResult(failures []RecipientDeliveryError) error {
	if len(failures) == 0 {
		return nil
	}
	return &PartialDeliveryError{Failed: failures}
}

func dataAcceptedResult(accepted []outbound.Address, failures []RecipientDeliveryError) error {
	if len(failures) > 0 {
		return &PartialDeliveryError{Delivered: accepted, Failed: failures}
	}
	return nil
}

func (t *DirectSMTPTransport) startTLS(ctx context.Context, client *smtp.Client, host string, modeOverride DeliveryTLSMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mode := normalizeDeliveryTLSMode(t.TLSMode)
	if modeOverride != "" {
		mode = normalizeDeliveryTLSMode(modeOverride)
	}
	if mode == DeliveryTLSDisable {
		return nil
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		if mode == DeliveryTLSRequire {
			return fmt.Errorf("STARTTLS is required but not advertised by %s", host)
		}
		return nil
	}
	return client.StartTLS(t.deliveryTLSConfig(host))
}

func (t *DirectSMTPTransport) deliveryTLSConfig(host string) *tls.Config {
	var cfg *tls.Config
	if t.TLSConfig != nil {
		cfg = t.TLSConfig.Clone()
	} else {
		cfg = &tls.Config{}
	}
	if strings.TrimSpace(cfg.ServerName) == "" {
		cfg.ServerName = strings.TrimSpace(host)
	}
	if cfg.MinVersion == 0 || cfg.MinVersion < tls.VersionTLS12 {
		cfg.MinVersion = tls.VersionTLS12
	}
	return cfg
}

// recordTLSResult records TLS delivery result for TLS-RPT reporting.
func (t *DirectSMTPTransport) recordTLSResult(sendingMTA string, tlsVersion string, tlsCipherSuite string, err error) {
	if t.tlsrptCollector == nil {
		return
	}
	if err != nil {
		// Record TLS failure
		failure := &tlsrpt.FailureDetails{
			ResultType:        "tlsa",                      // Could be other types
			FailureReasonCode: "certificate-host-mismatch", // Simplified
			FailureReasonText: err.Error(),
		}
		t.tlsrptCollector.RecordFailure("tlsa", sendingMTA, failure)
	} else {
		// Record TLS success
		success := &tlsrpt.SuccessDetails{
			TLSVersion:     tlsVersion,
			TLSCipherSuite: tlsCipherSuite,
		}
		t.tlsrptCollector.RecordSuccess("tlsa", sendingMTA, success)
	}
}

func (t *DirectSMTPTransport) route(ctx context.Context, job Job, domain string) (Route, error) {
	if t.Router == nil {
		return normalizeRoute(job, domain, Route{TLSMode: t.TLSMode}), nil
	}
	route, err := t.Router.Route(ctx, job, domain)
	if err != nil {
		return Route{}, fmt.Errorf("route delivery for %s: %w", domain, err)
	}
	if strings.TrimSpace(string(route.TLSMode)) == "" {
		route.TLSMode = t.TLSMode
	}
	return normalizeRoute(job, domain, route), nil
}

func (t *DirectSMTPTransport) openMessage(ctx context.Context, job Job) (io.ReadCloser, error) {
	message, err := job.OpenMessage(ctx)
	if err != nil {
		return nil, err
	}
	if len(t.Transformers) == 0 {
		return message, nil
	}
	return t.Transformers.Transform(ctx, job, message)
}

// headerInjector prepends RFC 5321 Received header to message.
type headerInjector struct {
	reader io.Reader
	msgR   io.ReadCloser
}

func newHeaderInjector(header string, msg io.ReadCloser) *headerInjector {
	return &headerInjector{
		reader: io.MultiReader(strings.NewReader(header), msg),
		msgR:   msg,
	}
}

func (h *headerInjector) Read(p []byte) (n int, err error) {
	return h.reader.Read(p)
}

func (h *headerInjector) Close() error {
	return h.msgR.Close()
}

func (t *DirectSMTPTransport) mxHosts(ctx context.Context, domain string) ([]string, error) {
	resolver := t.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	records, err := resolver.LookupMX(ctx, domain)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && (dnsErr.IsTemporary || dnsErr.IsTimeout) {
			return nil, &SMTPStatusError{
				Op:      "mx",
				Code:    451,
				Message: fmt.Sprintf("temporary MX lookup failure for %s", domain),
				Err:     err,
			}
		}
		return []string{domain}, nil
	}
	if len(records) == 0 {
		return []string{domain}, nil
	}
	if isNullMX(records) {
		return nil, &SMTPStatusError{
			Op:      "mx",
			Code:    556,
			Message: fmt.Sprintf("domain %s publishes null MX and does not accept mail", domain),
		}
	}
	hosts := orderedMXHosts(records)
	if len(hosts) == 0 {
		return []string{domain}, nil
	}
	return hosts, nil
}

func orderedMXHosts(records []*net.MX) []string {
	ordered := make([]*net.MX, 0, len(records))
	for _, record := range records {
		if record != nil {
			ordered = append(ordered, record)
		}
	}
	hosts := make([]string, 0, len(ordered))
	if len(ordered) == 0 {
		return hosts
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Pref == ordered[j].Pref {
			return ordered[i].Host < ordered[j].Host
		}
		return ordered[i].Pref < ordered[j].Pref
	})
	seen := make(map[string]struct{}, len(ordered))
	for _, record := range ordered {
		host := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(record.Host, ".")))
		if host == "" || host == "." {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}
	return hosts
}

func isNullMX(records []*net.MX) bool {
	return len(records) == 1 && records[0] != nil && records[0].Pref == 0 && strings.TrimSpace(records[0].Host) == "."
}

func deliveryDeadline(ctx context.Context, timeout time.Duration, now time.Time) time.Time {
	var deadline time.Time
	if timeout > 0 {
		deadline = now.Add(timeout)
	}
	if ctxDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || ctxDeadline.Before(deadline)) {
		deadline = ctxDeadline
	}
	return deadline
}

func normalizeDeliveryTLSMode(mode DeliveryTLSMode) DeliveryTLSMode {
	switch DeliveryTLSMode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case DeliveryTLSRequire:
		return DeliveryTLSRequire
	case DeliveryTLSDisable:
		return DeliveryTLSDisable
	default:
		return DeliveryTLSOpportunistic
	}
}

type RecipientBatch struct {
	Domain     string
	Recipients []outbound.Address
}

func PlanRecipientBatches(recipients []outbound.Address) []RecipientBatch {
	return PlanRecipientBatchesWithLimit(recipients, 0)
}

func PlanRecipientBatchesWithLimit(recipients []outbound.Address, maxRecipientsPerBatch int) []RecipientBatch {
	type domainBatch struct {
		domain     string
		recipients []outbound.Address
	}

	domains := make([]string, 0)
	byDomain := make(map[string][]domainBatch)
	for _, recipient := range recipients {
		domain := normalizedRecipientDomain(recipient.Email)
		if domain == "" {
			continue
		}
		if _, ok := byDomain[domain]; !ok {
			domains = append(domains, domain)
		}
		domainBatches := byDomain[domain]
		lastBatch := len(domainBatches) - 1
		if lastBatch >= 0 && (maxRecipientsPerBatch <= 0 || len(domainBatches[lastBatch].recipients) < maxRecipientsPerBatch) {
			domainBatches[lastBatch].recipients = append(domainBatches[lastBatch].recipients, recipient)
			byDomain[domain] = domainBatches
			continue
		}
		byDomain[domain] = append(domainBatches, domainBatch{
			domain:     domain,
			recipients: []outbound.Address{recipient},
		})
	}

	batches := make([]RecipientBatch, 0, len(domains))
	for _, domain := range domains {
		for _, batch := range byDomain[domain] {
			batches = append(batches, RecipientBatch{
				Domain:     batch.domain,
				Recipients: batch.recipients,
			})
		}
	}
	return batches
}

func groupRecipientsByDomain(recipients []outbound.Address) map[string][]outbound.Address {
	groups := make(map[string][]outbound.Address)
	for _, batch := range PlanRecipientBatches(recipients) {
		groups[batch.Domain] = append(groups[batch.Domain], batch.Recipients...)
	}
	return groups
}

func normalizedRecipientDomain(address string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(address), "@")
	if !ok || domain == "" {
		return ""
	}
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	return domain
}

// checkMTASTSPolicy verifies the MX host matches MTA-STS policy for the domain.
func (t *DirectSMTPTransport) checkMTASTSPolicy(ctx context.Context, domain string, host string) error {
	if t.mtastsClient == nil {
		return nil
	}

	policy, err := t.mtastsClient.GetPolicy(ctx, domain)
	if err != nil {
		// DNS/HTTPS errors are non-fatal; log but continue
		return nil
	}

	// No policy or policy mode=none: OK
	if policy == nil || policy.Mode == "none" {
		return nil
	}

	// Policy exists: check if host matches
	if !policy.MatchesMX(host) {
		if policy.Mode == "enforce" {
			return fmt.Errorf("host %s not in MTA-STS policy for %s", host, domain)
		}
		// testing mode: log but allow
		_ = fmt.Sprintf("mta-sts testing: host %s not in policy for %s", host, domain)
	}

	return nil
}

// checkDANEPolicy validates the TLS certificate against DANE policy.
func (t *DirectSMTPTransport) checkDANEPolicy(ctx context.Context, domain string, host string, port int, client *smtp.Client) error {
	if t.daneValidator == nil {
		return nil
	}

	state, ok := client.TLSConnectionState()
	if !ok {
		// No TLS connection: DANE check not applicable
		return nil
	}

	// Convert peer certificates to tls.Certificate format
	var tlsCerts []*tls.Certificate
	if len(state.PeerCertificates) > 0 {
		tlsCert := &tls.Certificate{
			Certificate: [][]byte{state.PeerCertificates[0].Raw},
			Leaf:        state.PeerCertificates[0],
		}
		tlsCerts = []*tls.Certificate{tlsCert}
	}

	result, err := t.daneValidator.Validate(ctx, host, port, tlsCerts)
	if err != nil {
		// DANE lookup errors are non-fatal
		return nil
	}

	// If DANE records exist, certificate must match
	if result.Present && !result.Valid {
		return fmt.Errorf("DANE validation failed for %s: %s", host, result.Reason)
	}

	return nil
}
