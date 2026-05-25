package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gogomail/gogomail/internal/app"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, app.Run))
}

func run(args []string, stdout io.Writer, stderr io.Writer, runApp func(context.Context, app.Mode, config.Config, *slog.Logger) error) int {
	// Intercept "admin" subcommand before flag parsing.
	if len(args) > 0 && args[0] == "admin" {
		return runAdminCommand(args[1:], stdout, stderr)
	}

	flags := flag.NewFlagSet("gogomail", flag.ContinueOnError)
	flags.SetOutput(stderr)
	modeRaw := flags.String("mode", string(app.ModeAllInOne), "component mode to run")
	runMigrations := flags.Bool("migrate", false, "run database migrations before starting")
	configFile := flags.String("config", "", "optional YAML config file")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	modeValue := *modeRaw
	if !flagWasSet(flags, "mode") {
		if envMode := os.Getenv("APP_MODE"); envMode != "" {
			modeValue = envMode
		}
	}
	mode, err := app.ParseMode(modeValue)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	cfg, err := config.LoadFile(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "invalid config file: %v\n", err)
		return 2
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return 2
	}
	var logHandler slog.Handler
	logOptions := &slog.HandlerOptions{Level: slog.LevelInfo, ReplaceAttr: redactLogAttr}
	if strings.EqualFold(strings.TrimSpace(cfg.LogFormat), "text") {
		logHandler = slog.NewTextHandler(stdout, logOptions)
	} else {
		logHandler = slog.NewJSONHandler(stdout, logOptions)
	}
	logger := slog.New(newContextHandler(logHandler))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *runMigrations {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database connection failed", "error", err)
			return 1
		}
		defer db.Close()

		if err := database.MigrateUp(ctx, db, cfg.MigrationDir); err != nil {
			logger.Error("database migration failed", "error", err, "dir", cfg.MigrationDir)
			return 1
		}
		logger.Info("database migrations completed", "dir", cfg.MigrationDir)
	}

	if err := runApp(ctx, mode, cfg, logger); err != nil {
		logger.Error("gogomail stopped with error", "error", err)
		return 1
	}
	return 0
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	seen := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			seen = true
		}
	})
	return seen
}
