package app

import (
	"context"
	"database/sql"

	"github.com/gogomail/gogomail/internal/idprovider"
	"github.com/gogomail/gogomail/internal/idprovider/database"
	"github.com/gogomail/gogomail/internal/maildb"
)

// initializeIdPProvider sets up the identity provider system with database provider registration.
func initializeIdPProvider(ctx context.Context, db *sql.DB) error {
	// Create database provider
	maildbRepo := maildb.NewRepository(db)
	dbProvider := database.New(db, maildbRepo)

	// Register database provider as default
	idprovider.Register("database", dbProvider)

	// Note: Per-domain IdP configuration loading can be extended here
	// to support LDAP and other providers as they are added.
	// For now, all domains default to database mode.

	return nil
}
