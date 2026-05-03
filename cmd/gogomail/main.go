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
)

func main() {
	modeRaw := flag.String("mode", string(app.ModeAllInOne), "component mode to run")
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

	if err := app.Run(ctx, mode, cfg, logger); err != nil {
		logger.Error("gogomail stopped with error", "error", err)
		os.Exit(1)
	}
}
