package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gogomail/gogomail/internal/backup"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/imapimport"
	"github.com/gogomail/gogomail/internal/maildb"
)

// runIMAPImportCommand implements `gogomail imap-import`: it connects to a source
// IMAP server, enumerates folders, and imports messages into a target gogomail
// user, preserving flags, internal dates, and folder structure (with a
// configurable mapping). It supports dry-run, resume, rate limiting, and
// concurrent folder workers.
//
// Source credentials are read from the environment (or a secret file) only and
// are never written to stdout/stderr or logs.
func runIMAPImportCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("imap-import", flag.ContinueOnError)
	flags.SetOutput(stderr)

	configFile := flags.String("config", "", "optional YAML config file (for storage/database wiring)")
	host := flags.String("host", "", "source IMAP host (required)")
	port := flags.Int("port", 0, "source IMAP port (default 993 with TLS, 143 without)")
	useTLS := flags.Bool("tls", true, "use implicit TLS (port 993); required by default")
	startTLS := flags.Bool("starttls", false, "connect in cleartext then upgrade with STARTTLS")
	insecureTLS := flags.Bool("insecure-tls", false, "skip TLS certificate verification (explicit opt-in; testing only)")
	allowInsecure := flags.Bool("allow-insecure", false, "allow a plaintext connection with no TLS (explicit opt-in; lab only)")
	username := flags.String("username", "", "source IMAP username (required)")
	targetUser := flags.String("target-user", "", "destination gogomail user email address (required)")
	mappingRaw := flags.String("mapping", "prefix", "folder mapping strategy: prefix | merge | flat-labels")
	prefix := flags.String("prefix", "", "folder-name prefix for the prefix strategy (e.g. Imported)")
	mergeTarget := flags.String("merge-target", "INBOX", "destination folder for the merge strategy")
	separator := flags.String("separator", "/", "source hierarchy delimiter (auto-detected from LIST when possible)")
	concurrency := flags.Int("concurrency", 1, "number of folders imported in parallel")
	rate := flags.Float64("rate", 0, "max messages processed per second (0 = unlimited; use to respect source throttling)")
	dryRun := flags.Bool("dry-run", false, "report folder/message counts without importing")
	resumeFile := flags.String("resume-file", "", "path to a JSON progress file for resumable/idempotent runs")
	commandTimeout := flags.Duration("command-timeout", 2*time.Minute, "per-command timeout for the source IMAP connection")

	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: gogomail imap-import --host <h> --username <u> --target-user <email> [flags]")
		fmt.Fprintln(stderr, "imports a source IMAP mailbox into a gogomail user, preserving flags/dates/folders")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "credentials (never logged) are read from the environment:")
		fmt.Fprintln(stderr, "  IMAP_IMPORT_PASSWORD        source account password / app-password")
		fmt.Fprintln(stderr, "  IMAP_IMPORT_PASSWORD_FILE   path to a file containing the password")
		fmt.Fprintln(stderr, "  IMAP_IMPORT_OAUTH2_TOKEN    OAuth2 access token (SASL XOAUTH2)")
		fmt.Fprintln(stderr, "  IMAP_IMPORT_OAUTH2_TOKEN_FILE  path to a file containing the token")
		fmt.Fprintln(stderr, "")
		flags.PrintDefaults()
		fmt.Fprintln(stderr, "\nsee docs/IMAP_IMPORT.md for provider-specific notes (Gmail/Outlook/Fastmail).")
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(*host) == "" {
		fmt.Fprintln(stderr, "error: --host is required")
		flags.Usage()
		return 2
	}
	if strings.TrimSpace(*username) == "" {
		fmt.Fprintln(stderr, "error: --username is required")
		return 2
	}
	if strings.TrimSpace(*targetUser) == "" {
		fmt.Fprintln(stderr, "error: --target-user is required")
		return 2
	}

	strategy, err := imapimport.ParseMappingStrategy(*mappingRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	// TLS is required by default. STARTTLS is an alternative secure path.
	// --allow-insecure is the only way to run without transport security.
	if *startTLS {
		*useTLS = false
	}
	if !*useTLS && !*startTLS && !*allowInsecure {
		fmt.Fprintln(stderr, "error: refusing to connect without TLS; use --tls (default), --starttls, or explicitly --allow-insecure")
		return 2
	}

	// Read credentials from env / secret file only.
	password, err := readSecret("IMAP_IMPORT_PASSWORD", "IMAP_IMPORT_PASSWORD_FILE")
	if err != nil {
		fmt.Fprintf(stderr, "error: read password: %v\n", err)
		return 2
	}
	oauthToken, err := readSecret("IMAP_IMPORT_OAUTH2_TOKEN", "IMAP_IMPORT_OAUTH2_TOKEN_FILE")
	if err != nil {
		fmt.Fprintf(stderr, "error: read oauth2 token: %v\n", err)
		return 2
	}
	if password == "" && oauthToken == "" {
		fmt.Fprintln(stderr, "error: no source credentials provided; set IMAP_IMPORT_PASSWORD(_FILE) or IMAP_IMPORT_OAUTH2_TOKEN(_FILE)")
		return 2
	}

	cfg, err := config.LoadFile(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "error: invalid config file: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logf := func(format string, args ...any) {
		fmt.Fprintf(stdout, "imap-import: "+format+"\n", args...)
	}

	// Connect to the source server (with a per-command timeout applied inside).
	dialCtx, dialCancel := context.WithTimeout(ctx, 45*time.Second)
	src, err := imapimport.Dial(dialCtx, imapimport.DialOptions{
		Host:               strings.TrimSpace(*host),
		Port:               *port,
		TLS:                *useTLS,
		StartTLS:           *startTLS,
		AllowInsecure:      *allowInsecure,
		InsecureSkipVerify: *insecureTLS,
		Username:           strings.TrimSpace(*username),
		Password:           password,
		OAuth2Token:        oauthToken,
		CommandTimeout:     *commandTimeout,
	})
	dialCancel()
	if err != nil {
		fmt.Fprintf(stderr, "error: connect to source: %v\n", err)
		return 1
	}
	defer src.Close()
	logf("connected to source %s as %s", *host, *username)

	opts := imapimport.Options{
		Mapping: imapimport.MappingConfig{
			Strategy:    strategy,
			Separator:   *separator,
			Prefix:      *prefix,
			MergeTarget: *mergeTarget,
		},
		Concurrency: *concurrency,
		RateLimit:   *rate,
		DryRun:      *dryRun,
		Logger:      logf,
	}

	var sink imapimport.MessageSink
	var progressStore imapimport.ProgressStore
	if strings.TrimSpace(*resumeFile) != "" {
		progressStore = imapimport.NewFileProgressStore(strings.TrimSpace(*resumeFile))
	}

	if !*dryRun {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			fmt.Fprintf(stderr, "error: database connection failed: %v\n", err)
			return 1
		}
		defer db.Close()
		repo := maildb.NewRepository(db)

		userInfo, err := repo.GetUserByEmail(ctx, strings.TrimSpace(*targetUser))
		if err != nil {
			fmt.Fprintf(stderr, "error: target user %q not found: %v\n", *targetUser, err)
			return 1
		}
		store, err := backup.StoreForConfig(cfg)
		if err != nil {
			fmt.Fprintf(stderr, "error: build storage client: %v\n", err)
			return 1
		}
		sink = newMaildbSink(repo, store, userInfo.UserID)
		logf("importing into gogomail user %s (mapping=%s)", *targetUser, strategy)
	} else {
		logf("dry-run: no messages will be imported (mapping=%s)", strategy)
	}

	importer := imapimport.New(src, sink, progressStore, opts)
	report, runErr := importer.Run(ctx)

	printImportReport(stdout, report, *dryRun)

	if runErr != nil && ctx.Err() != nil {
		fmt.Fprintln(stderr, "imap-import: interrupted; progress saved (re-run with the same --resume-file to continue)")
		return 1
	}
	if runErr != nil {
		fmt.Fprintf(stderr, "error: import failed: %v\n", runErr)
		return 1
	}
	if len(report.Errors) > 0 {
		return 1
	}
	return 0
}

// readSecret returns the value of envVar, or the trimmed contents of the file
// named by fileEnvVar, or "" when neither is set. Values are never logged.
func readSecret(envVar, fileEnvVar string) (string, error) {
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	if path := strings.TrimSpace(os.Getenv(fileEnvVar)); path != "" {
		data, err := os.ReadFile(path) // #nosec G304 -- path is operator-provided secret file
		if err != nil {
			return "", fmt.Errorf("read %s: %w", fileEnvVar, err)
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	}
	return "", nil
}

func printImportReport(w io.Writer, report *imapimport.Report, dryRun bool) {
	if report == nil {
		return
	}
	fmt.Fprintln(w, "\n=== IMAP import report ===")
	fmt.Fprintf(w, "  folders: %d total, %d skipped, %d processed\n", report.FoldersTotal, report.FoldersSkipped, report.FoldersImported)
	if dryRun {
		fmt.Fprintf(w, "  messages: %d would be imported (%d bytes seen)\n", report.MessagesSeen, report.BytesSeen)
	} else {
		fmt.Fprintf(w, "  messages: %d seen, %d stored, %d skipped (duplicates)\n", report.MessagesSeen, report.MessagesStored, report.MessagesSkipped)
	}
	if len(report.PerFolder) > 0 {
		fmt.Fprintln(w, "  per destination folder:")
		for name, n := range report.PerFolder {
			fmt.Fprintf(w, "    %-30s %d\n", name, n)
		}
	}
	if len(report.Errors) > 0 {
		fmt.Fprintf(w, "  errors (%d):\n", len(report.Errors))
		for _, e := range report.Errors {
			fmt.Fprintf(w, "    - %s\n", e)
		}
	}
}
