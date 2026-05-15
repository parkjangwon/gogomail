package alert

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DBRepository implements Repository using PostgreSQL.
type DBRepository struct {
	db *sql.DB
}

// NewDBRepository creates a new database repository.
func NewDBRepository(db *sql.DB) *DBRepository {
	return &DBRepository{db: db}
}

// CreateConfig creates a new alert configuration.
func (r *DBRepository) CreateConfig(ctx context.Context, cfg *Config) error {
	cfg.ID = uuid.New()
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()

	err := r.db.QueryRowContext(ctx,
		`INSERT INTO alert_configs (id, company_id, alert_type, threshold, name, description, check_interval_minutes, is_enabled, created_at, updated_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, created_at, updated_at`,
		cfg.ID, cfg.CompanyID, cfg.AlertType, cfg.Threshold, cfg.Name, cfg.Description,
		cfg.CheckIntervalMinutes, cfg.IsEnabled, cfg.CreatedAt, cfg.UpdatedAt, cfg.CreatedByID,
	).Scan(&cfg.ID, &cfg.CreatedAt, &cfg.UpdatedAt)

	return err
}

// GetConfig retrieves a single alert configuration with channels.
func (r *DBRepository) GetConfig(ctx context.Context, id uuid.UUID) (*Config, error) {
	cfg := &Config{Channels: []Channel{}}
	var createdBy sql.NullString

	err := r.db.QueryRowContext(ctx,
		`SELECT id, company_id, alert_type, threshold, name, description, check_interval_minutes, is_enabled, created_at, updated_at, created_by
		 FROM alert_configs WHERE id = $1`,
		id,
	).Scan(&cfg.ID, &cfg.CompanyID, (*string)(&cfg.AlertType), &cfg.Threshold, &cfg.Name, &cfg.Description,
		&cfg.CheckIntervalMinutes, &cfg.IsEnabled, &cfg.CreatedAt, &cfg.UpdatedAt, &createdBy)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if createdBy.Valid {
		id, _ := uuid.Parse(createdBy.String)
		cfg.CreatedByID = &id
	}

	// Load channels
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, alert_config_id, channel_type, config, is_enabled, created_at, updated_at
		 FROM alert_channels WHERE alert_config_id = $1`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		ch := Channel{}
		var configJSON []byte
		if err := rows.Scan(&ch.ID, &ch.ConfigID, (*string)(&ch.ChannelType), &configJSON, &ch.IsEnabled, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(configJSON, &ch.Config); err != nil {
			return nil, err
		}
		cfg.Channels = append(cfg.Channels, ch)
	}

	return cfg, rows.Err()
}

// ListConfigs lists all alert configurations for a company.
func (r *DBRepository) ListConfigs(ctx context.Context, companyID uuid.UUID) ([]Config, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, company_id, alert_type, threshold, name, description, check_interval_minutes, is_enabled, created_at, updated_at, created_by
		 FROM alert_configs WHERE company_id = $1 ORDER BY created_at DESC`,
		companyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []Config
	for rows.Next() {
		cfg := Config{Channels: []Channel{}}
		var createdBy sql.NullString

		if err := rows.Scan(&cfg.ID, &cfg.CompanyID, (*string)(&cfg.AlertType), &cfg.Threshold, &cfg.Name, &cfg.Description,
			&cfg.CheckIntervalMinutes, &cfg.IsEnabled, &cfg.CreatedAt, &cfg.UpdatedAt, &createdBy); err != nil {
			return nil, err
		}

		if createdBy.Valid {
			id, _ := uuid.Parse(createdBy.String)
			cfg.CreatedByID = &id
		}

		configs = append(configs, cfg)
	}

	return configs, rows.Err()
}

// UpdateConfig updates an alert configuration.
func (r *DBRepository) UpdateConfig(ctx context.Context, cfg *Config) error {
	cfg.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx,
		`UPDATE alert_configs SET alert_type = $2, threshold = $3, name = $4, description = $5, check_interval_minutes = $6, is_enabled = $7, updated_at = $8
		 WHERE id = $1`,
		cfg.ID, cfg.AlertType, cfg.Threshold, cfg.Name, cfg.Description, cfg.CheckIntervalMinutes, cfg.IsEnabled, cfg.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("alert config not found: %v", cfg.ID)
	}

	return nil
}

// DeleteConfig deletes an alert configuration and its channels.
func (r *DBRepository) DeleteConfig(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM alert_configs WHERE id = $1`, id)
	if err != nil {
		return err
	}

	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("alert config not found: %v", id)
	}

	return nil
}

// CreateChannel creates a notification channel.
func (r *DBRepository) CreateChannel(ctx context.Context, channel *Channel) error {
	channel.ID = uuid.New()
	channel.CreatedAt = time.Now()
	channel.UpdatedAt = time.Now()

	configJSON, err := json.Marshal(channel.Config)
	if err != nil {
		return err
	}

	err = r.db.QueryRowContext(ctx,
		`INSERT INTO alert_channels (id, alert_config_id, channel_type, config, is_enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		channel.ID, channel.ConfigID, channel.ChannelType, configJSON, channel.IsEnabled, channel.CreatedAt, channel.UpdatedAt,
	).Scan(&channel.ID, &channel.CreatedAt, &channel.UpdatedAt)

	return err
}

// DeleteChannel deletes a notification channel.
func (r *DBRepository) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM alert_channels WHERE id = $1`, id)
	if err != nil {
		return err
	}

	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("alert channel not found: %v", id)
	}

	return nil
}

// CreateNotification records a triggered alert notification.
func (r *DBRepository) CreateNotification(ctx context.Context, notif *Notification) error {
	notif.ID = uuid.New()
	notif.CreatedAt = time.Now()

	notifJSON, err := json.Marshal(notif.NotificationData)
	if err != nil {
		return err
	}

	err = r.db.QueryRowContext(ctx,
		`INSERT INTO alert_notifications (id, company_id, alert_config_id, alert_type, threshold, current_value, notification_data, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at`,
		notif.ID, notif.CompanyID, notif.AlertConfigID, notif.AlertType, notif.Threshold, notif.CurrentValue, notifJSON, notif.CreatedAt,
	).Scan(&notif.ID, &notif.CreatedAt)

	return err
}

// ListNotifications lists recent notifications for a company.
func (r *DBRepository) ListNotifications(ctx context.Context, companyID uuid.UUID, limit int) ([]Notification, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, company_id, alert_config_id, alert_type, threshold, current_value, email_sent, webhook_sent, dashboard_shown, notification_data, created_at, acknowledged_at
		 FROM alert_notifications WHERE company_id = $1 ORDER BY created_at DESC LIMIT $2`,
		companyID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		notif := Notification{}
		var notifJSON []byte

		if err := rows.Scan(&notif.ID, &notif.CompanyID, &notif.AlertConfigID, (*string)(&notif.AlertType), &notif.Threshold, &notif.CurrentValue,
			&notif.EmailSent, &notif.WebhookSent, &notif.DashboardShown, &notifJSON, &notif.CreatedAt, &notif.AcknowledgedAt); err != nil {
			return nil, err
		}

		if len(notifJSON) > 0 {
			if err := json.Unmarshal(notifJSON, &notif.NotificationData); err != nil {
				return nil, err
			}
		}

		notifications = append(notifications, notif)
	}

	return notifications, rows.Err()
}

// AcknowledgeNotification marks a notification as acknowledged.
func (r *DBRepository) AcknowledgeNotification(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE alert_notifications SET acknowledged_at = $1 WHERE id = $2`,
		time.Now(), id,
	)
	if err != nil {
		return err
	}

	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("notification not found: %v", id)
	}

	return nil
}
