package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/maildb"
)

func runAdminCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: gogomail admin <subcommand> [flags]")
		fmt.Fprintln(stderr, "subcommands:")
		fmt.Fprintln(stderr, "  mfa-reset  --email <email>   Disable MFA for an admin user")
		return 2
	}

	switch args[0] {
	case "mfa-reset":
		return runAdminMFAReset(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown admin subcommand: %s\n", args[0])
		return 2
	}
}

func runAdminMFAReset(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("mfa-reset", flag.ContinueOnError)
	flags.SetOutput(stderr)
	email := flags.String("email", "", "email address of the admin user")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *email == "" {
		fmt.Fprintln(stderr, "error: --email is required")
		flags.Usage()
		return 2
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		host := os.Getenv("POSTGRES_HOST")
		port := os.Getenv("POSTGRES_PORT")
		user := os.Getenv("POSTGRES_USER")
		pass := os.Getenv("POSTGRES_PASSWORD")
		name := os.Getenv("POSTGRES_DB")
		if port == "" {
			port = "5432"
		}
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, name)
	}

	ctx := context.Background()
	db, err := database.Open(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: database connection failed: %v\n", err)
		return 1
	}
	defer db.Close()

	repo := maildb.NewRepository(db)

	userInfo, err := repo.GetUserByEmail(ctx, *email)
	if err != nil {
		fmt.Fprintf(stderr, "error: user not found: %v\n", err)
		return 1
	}

	if err := repo.DisableMFA(ctx, userInfo.UserID); err != nil {
		fmt.Fprintf(stderr, "error: mfa reset failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "[%s] MFA reset successful for %s\n",
		time.Now().UTC().Format(time.RFC3339), *email)
	return 0
}
