package imapimport

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// DialOptions configures a connection to a source IMAP server.
type DialOptions struct {
	Host string
	Port int
	// TLS selects implicit TLS on connect (port 993). When false, the client
	// connects in cleartext and then issues STARTTLS unless AllowInsecure is set.
	TLS bool
	// StartTLS upgrades a cleartext connection with STARTTLS. Mutually exclusive
	// with TLS.
	StartTLS bool
	// AllowInsecure permits skipping TLS entirely (no implicit TLS, no
	// STARTTLS). This is an explicit opt-in for lab/testing only; production
	// imports must use TLS.
	AllowInsecure bool
	// InsecureSkipVerify disables certificate verification when using TLS. Also
	// an explicit opt-in.
	InsecureSkipVerify bool
	// Username is the source account login.
	Username string
	// Password is the source account password or app-password. Mutually
	// exclusive with OAuth2Token.
	Password string
	// OAuth2Token, when set, authenticates via SASL XOAUTH2 (RFC 7628-style
	// used by Gmail/Microsoft 365).
	OAuth2Token string
	// DialTimeout bounds the initial TCP/TLS handshake.
	DialTimeout time.Duration
	// CommandTimeout bounds each individual command round-trip.
	CommandTimeout time.Duration
}

func (o DialOptions) commandTimeout() time.Duration {
	if o.CommandTimeout <= 0 {
		return 60 * time.Second
	}
	return o.CommandTimeout
}

// imapClient is a minimal RFC 3501/9051 client sufficient for read-only import:
// LOGIN / AUTHENTICATE XOAUTH2, LIST, EXAMINE, STATUS, UID FETCH. It is not a
// general-purpose IMAP client and deliberately avoids new dependencies.
type imapClient struct {
	conn    net.Conn
	r       *bufio.Reader
	opts    DialOptions
	tag     int
	delim   string // last-seen hierarchy delimiter from LIST
}

// Dial connects, upgrades TLS as configured, authenticates, and returns a ready
// SourceClient. Credentials are never logged by this package.
func Dial(ctx context.Context, opts DialOptions) (SourceClient, error) {
	if strings.TrimSpace(opts.Host) == "" {
		return nil, errors.New("imap host is required")
	}
	if opts.Port == 0 {
		if opts.TLS {
			opts.Port = 993
		} else {
			opts.Port = 143
		}
	}
	if !opts.TLS && !opts.StartTLS && !opts.AllowInsecure {
		return nil, errors.New("refusing to connect without TLS; enable TLS/STARTTLS or explicitly allow insecure")
	}
	if opts.Password == "" && opts.OAuth2Token == "" {
		return nil, errors.New("either a password or an OAuth2 token is required")
	}

	dialTimeout := opts.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 30 * time.Second
	}
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	dialer := &net.Dialer{Timeout: dialTimeout}

	var conn net.Conn
	var err error
	if opts.TLS {
		tlsCfg := &tls.Config{
			ServerName:         opts.Host,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: opts.InsecureSkipVerify, // #nosec G402 -- explicit operator opt-in via --insecure-tls
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("dial imap %s: %w", addr, err)
	}

	c := &imapClient{conn: conn, r: bufio.NewReader(conn), opts: opts, delim: "/"}

	// Read the server greeting.
	if _, err := c.readResponse(ctx, ""); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read imap greeting: %w", err)
	}

	if opts.StartTLS && !opts.TLS {
		if err := c.startTLS(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	if err := c.authenticate(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

func (c *imapClient) nextTag() string {
	c.tag++
	return fmt.Sprintf("a%03d", c.tag)
}

func (c *imapClient) startTLS(ctx context.Context) error {
	if _, err := c.command(ctx, "STARTTLS"); err != nil {
		return fmt.Errorf("starttls: %w", err)
	}
	tlsCfg := &tls.Config{
		ServerName:         c.opts.Host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.opts.InsecureSkipVerify, // #nosec G402 -- explicit operator opt-in
	}
	tlsConn := tls.Client(c.conn, tlsCfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("starttls handshake: %w", err)
	}
	c.conn = tlsConn
	c.r = bufio.NewReader(tlsConn)
	return nil
}

func (c *imapClient) authenticate(ctx context.Context) error {
	if c.opts.OAuth2Token != "" {
		// SASL XOAUTH2 initial response (used by Gmail / Microsoft 365).
		ir := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.opts.Username, c.opts.OAuth2Token)
		enc := base64.StdEncoding.EncodeToString([]byte(ir))
		if _, err := c.command(ctx, "AUTHENTICATE XOAUTH2 "+enc); err != nil {
			return fmt.Errorf("xoauth2 authenticate: %w", err)
		}
		return nil
	}
	// LOGIN with quoted arguments (RFC 3501 §6.2.3).
	cmd := fmt.Sprintf("LOGIN %s %s", quoteIMAP(c.opts.Username), quoteIMAP(c.opts.Password))
	if _, err := c.command(ctx, cmd); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	return nil
}

// ListFolders implements SourceClient.
func (c *imapClient) ListFolders(ctx context.Context) ([]SourceFolder, error) {
	lines, err := c.command(ctx, `LIST "" "*"`)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	folders := make([]SourceFolder, 0, len(lines))
	for _, line := range lines {
		f, ok := parseListLine(line)
		if !ok {
			continue
		}
		if f.Delimiter != "" {
			c.delim = f.Delimiter
		}
		folders = append(folders, f)
	}
	return folders, nil
}

// Status implements SourceClient using STATUS (MESSAGES).
func (c *imapClient) Status(ctx context.Context, folder string) (FolderStatus, error) {
	lines, err := c.command(ctx, fmt.Sprintf("STATUS %s (MESSAGES)", quoteIMAP(folder)))
	if err != nil {
		return FolderStatus{}, fmt.Errorf("status %q: %w", folder, err)
	}
	var status FolderStatus
	for _, line := range lines {
		if n, ok := parseStatusMessages(line); ok {
			status.Messages = n
		}
	}
	return status, nil
}

// FetchMessages implements SourceClient. It EXAMINEs (read-only) the folder and
// streams every message via UID FETCH.
func (c *imapClient) FetchMessages(ctx context.Context, folder string, fn func(SourceMessage) error) error {
	lines, err := c.command(ctx, fmt.Sprintf("EXAMINE %s", quoteIMAP(folder)))
	if err != nil {
		return fmt.Errorf("examine %q: %w", folder, err)
	}
	var exists uint32
	for _, line := range lines {
		if n, ok := parseExists(line); ok {
			exists = n
		}
	}
	if exists == 0 {
		return nil
	}
	// Fetch flags, internal date, and the full body for the whole folder.
	// Streamed message-by-message parsing keeps memory bounded per message.
	return c.uidFetchStream(ctx, "1:*", fn)
}

// Close implements SourceClient.
func (c *imapClient) Close() error {
	if c.conn == nil {
		return nil
	}
	// Best-effort LOGOUT then close.
	_, _ = c.command(context.Background(), "LOGOUT")
	return c.conn.Close()
}

// command sends a tagged command and returns the untagged response lines that
// preceded the tagged completion. It returns an error when the tagged response
// is not OK.
func (c *imapClient) command(ctx context.Context, cmd string) ([]string, error) {
	tag := c.nextTag()
	if err := c.writeLine(ctx, tag+" "+cmd); err != nil {
		return nil, err
	}
	return c.readResponse(ctx, tag)
}

func (c *imapClient) writeLine(ctx context.Context, line string) error {
	if dl, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(dl)
	} else {
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.opts.commandTimeout()))
	}
	_, err := io.WriteString(c.conn, line+"\r\n")
	return err
}

// readResponse reads lines until the tagged completion line (tag != "") or the
// greeting (tag == ""). Literals ({n}) are read and appended inline so callers
// see the full untagged response. Returns the collected untagged lines.
func (c *imapClient) readResponse(ctx context.Context, tag string) ([]string, error) {
	if dl, ok := ctx.Deadline(); ok {
		_ = c.conn.SetReadDeadline(dl)
	} else {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.opts.commandTimeout()))
	}
	var lines []string
	for {
		line, err := c.readLineWithLiterals()
		if err != nil {
			return lines, err
		}
		if tag == "" {
			// Greeting: single line starting with "* OK" (or BYE/NO).
			if strings.HasPrefix(line, "* OK") || strings.HasPrefix(line, "* PREAUTH") {
				return lines, nil
			}
			return lines, fmt.Errorf("unexpected greeting: %s", firstLine(line))
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, tag+" "))
			upper := strings.ToUpper(rest)
			switch {
			case strings.HasPrefix(upper, "OK"):
				return lines, nil
			case strings.HasPrefix(upper, "NO"), strings.HasPrefix(upper, "BAD"):
				return lines, fmt.Errorf("imap command failed: %s", rest)
			default:
				return lines, fmt.Errorf("unexpected tagged response: %s", rest)
			}
		}
		if strings.HasPrefix(line, "+ ") {
			// Continuation request — not expected in our command set.
			continue
		}
		lines = append(lines, line)
	}
}

// readLineWithLiterals reads one logical response line, expanding any IMAP
// literal ({n}\r\n<n bytes>) that appears at the end of the line into the
// returned string so parsers get the full data inline.
func (c *imapClient) readLineWithLiterals() (string, error) {
	var b strings.Builder
	for {
		raw, err := c.r.ReadString('\n')
		if err != nil {
			if len(raw) > 0 {
				b.WriteString(strings.TrimRight(raw, "\r\n"))
			}
			return b.String(), err
		}
		trimmed := strings.TrimRight(raw, "\r\n")
		if n, ok := trailingLiteral(trimmed); ok {
			b.WriteString(trimmed)
			b.WriteByte('\n')
			buf := make([]byte, n)
			if _, err := io.ReadFull(c.r, buf); err != nil {
				return b.String(), err
			}
			b.Write(buf)
			// Continue reading the remainder of the line after the literal.
			continue
		}
		b.WriteString(trimmed)
		return b.String(), nil
	}
}

// uidFetchStream issues a UID FETCH for the given set and parses each message.
// Because full bodies are transferred as literals, we parse the accumulated
// response in one pass but hand each message to fn as it is completed.
func (c *imapClient) uidFetchStream(ctx context.Context, set string, fn func(SourceMessage) error) error {
	if dl, ok := ctx.Deadline(); ok {
		_ = c.conn.SetReadDeadline(dl)
		_ = c.conn.SetWriteDeadline(dl)
	} else {
		d := time.Now().Add(c.opts.commandTimeout())
		_ = c.conn.SetReadDeadline(d)
		_ = c.conn.SetWriteDeadline(d)
	}
	tag := c.nextTag()
	cmd := fmt.Sprintf("%s UID FETCH %s (UID FLAGS INTERNALDATE BODY.PEEK[])", tag, set)
	if err := c.writeLine(ctx, cmd); err != nil {
		return err
	}
	for {
		line, err := c.readLineWithLiterals()
		if err != nil {
			return err
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, tag+" "))
			if strings.HasPrefix(strings.ToUpper(rest), "OK") {
				return nil
			}
			return fmt.Errorf("uid fetch failed: %s", rest)
		}
		if !strings.HasPrefix(line, "* ") {
			continue
		}
		msg, ok := parseFetchMessage(line)
		if !ok {
			continue
		}
		if err := fn(msg); err != nil {
			return err
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
