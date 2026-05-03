package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gogomail/gogomail/internal/app"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
)

func main() {
	modeRaw := flag.String("mode", string(app.ModeAllInOne), "component mode to run")
	runMigrations := flag.Bool("migrate", false, "run database migrations before starting")
	flag.Parse()

	mode, err := app.ParseMode(*modeRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *runMigrations {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		if err := database.MigrateUp(ctx, db, cfg.MigrationDir); err != nil {
			logger.Error("database migration failed", "error", err, "dir", cfg.MigrationDir)
			os.Exit(1)
		}
		logger.Info("database migrations completed", "dir", cfg.MigrationDir)
	}

	if err := app.Run(ctx, mode, cfg, logger); err != nil {
		logger.Error("gogomail stopped with error", "error", err)
		os.Exit(1)
	}
}
