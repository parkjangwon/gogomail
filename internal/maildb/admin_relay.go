package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) ListTrustedRelays(ctx context.Context, req TrustedRelayListRequest) ([]TrustedRelayView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateTrustedRelayListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	cidr := strings.TrimSpace(req.CIDR)
	if cidr != "" {
		normalized, err := normalizeTrustedRelayCIDR(cidr)
		if err != nil {
			return nil, err
		}
		cidr = normalized
	}
	description := strings.TrimSpace(req.Description)

	query, args := buildListTrustedRelaysQuery(cidr, description, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list trusted relays: %w", err)
	}
	defer rows.Close()

	var relays []TrustedRelayView
	for rows.Next() {
		var relay TrustedRelayView
		if err := rows.Scan(&relay.ID, &relay.CIDR, &relay.Description, &relay.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan trusted relay: %w", err)
		}
		relays = append(relays, relay)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted relays: %w", err)
	}
	return relays, nil
}

func buildListTrustedRelaysQuery(cidr string, description string, limit int) (string, []any) {
	args := make([]any, 0, 3)
	conditions := make([]string, 0, 2)
	if cidr = strings.TrimSpace(cidr); cidr != "" {
		args = append(args, cidr)
		conditions = append(conditions, fmt.Sprintf("cidr = $%d::cidr", len(args)))
	}
	if description = strings.TrimSpace(description); description != "" {
		args = append(args, description)
		conditions = append(conditions, fmt.Sprintf("description ILIKE '%%' || $%d || '%%'", len(args)))
	}
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", len(args))

	where := ""
	if len(conditions) > 0 {
		where = "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}

	query := `
SELECT
  id::text,
  cidr::text,
  description,
  created_at
FROM trusted_relays` + where + `
ORDER BY created_at DESC, id DESC
LIMIT ` + limitPlaceholder
	return query, args
}

func (r *Repository) CreateTrustedRelay(ctx context.Context, req CreateTrustedRelayRequest) (TrustedRelayView, error) {
	if r.db == nil {
		return TrustedRelayView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateTrustedRelayRequest(req); err != nil {
		return TrustedRelayView{}, err
	}
	cidr, err := normalizeTrustedRelayCIDR(req.CIDR)
	if err != nil {
		return TrustedRelayView{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TrustedRelayView{}, fmt.Errorf("begin trusted relay create transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO trusted_relays (cidr, description)
VALUES ($1, $2)
RETURNING id::text, cidr::text, description, created_at`

	var relay TrustedRelayView
	if err := tx.QueryRowContext(ctx, query, cidr, strings.TrimSpace(req.Description)).Scan(
		&relay.ID,
		&relay.CIDR,
		&relay.Description,
		&relay.CreatedAt,
	); err != nil {
		return TrustedRelayView{}, fmt.Errorf("create trusted relay: %w", err)
	}
	detail, err := trustedRelayAuditDetail(relay)
	if err != nil {
		return TrustedRelayView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "trusted_relay.create",
		TargetType: "trusted_relay",
		TargetID:   relay.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return TrustedRelayView{}, fmt.Errorf("record trusted relay create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TrustedRelayView{}, fmt.Errorf("commit trusted relay create transaction: %w", err)
	}
	return relay, nil
}

func (r *Repository) DeleteTrustedRelay(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("trusted relay id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin trusted relay delete transaction: %w", err)
	}
	defer tx.Rollback()

	var relay TrustedRelayView
	if err := tx.QueryRowContext(ctx, `
SELECT id::text, cidr::text, description, created_at
FROM trusted_relays
WHERE id = $1
FOR UPDATE`, id).Scan(&relay.ID, &relay.CIDR, &relay.Description, &relay.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("trusted relay %q not found", id)
		}
		return fmt.Errorf("read trusted relay for deletion: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM trusted_relays WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete trusted relay: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("trusted relay %q not found", id)
	}
	detail, err := trustedRelayAuditDetail(relay)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "trusted_relay.delete",
		TargetType: "trusted_relay",
		TargetID:   relay.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record trusted relay delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit trusted relay delete transaction: %w", err)
	}
	return nil
}

func trustedRelayAuditDetail(relay TrustedRelayView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"trusted_relay_id": relay.ID,
		"cidr":             relay.CIDR,
		"description":      relay.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal trusted relay audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListDeliveryRoutes(ctx context.Context, req DeliveryRouteListRequest) ([]DeliveryRouteView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateDeliveryRouteListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)

	query, args := buildListDeliveryRoutesQuery(req, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list delivery routes: %w", err)
	}
	defer rows.Close()

	var routes []DeliveryRouteView
	for rows.Next() {
		var route DeliveryRouteView
		if err := rows.Scan(
			&route.ID,
			&route.DomainPattern,
			&route.Farm,
			(*stringArray)(&route.Hosts),
			&route.Port,
			&route.TLSMode,
			&route.ImplicitTLS,
			&route.SMTPHello,
			&route.PoolName,
			&route.AuthIdentity,
			&route.AuthUsername,
			&route.Status,
			&route.Description,
			&route.CreatedAt,
			&route.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery route: %w", err)
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery routes: %w", err)
	}
	return routes, nil
}

func buildListDeliveryRoutesQuery(req DeliveryRouteListRequest, limit int) (string, []any) {
	args := make([]any, 0, 4)
	conditions := make([]string, 0, 3)
	if status := strings.ToLower(strings.TrimSpace(req.Status)); status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if farm := strings.TrimSpace(req.Farm); farm != "" {
		args = append(args, farm)
		conditions = append(conditions, fmt.Sprintf("farm = $%d", len(args)))
	}
	if domainPattern := strings.TrimSpace(req.DomainPattern); domainPattern != "" {
		args = append(args, domainPattern)
		conditions = append(conditions, fmt.Sprintf("domain_pattern = $%d", len(args)))
	}
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", len(args))

	where := ""
	if len(conditions) > 0 {
		where = "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}

	query := `
SELECT
  id::text,
  domain_pattern,
  farm,
  hosts,
  port,
  tls_mode,
  implicit_tls,
  smtp_hello,
  pool_name,
  auth_identity,
  auth_username,
  status,
  description,
  created_at,
  updated_at
FROM delivery_routes` + where + `
ORDER BY created_at DESC, id DESC
LIMIT ` + limitPlaceholder
	return query, args
}

func (r *Repository) CreateDeliveryRoute(ctx context.Context, req CreateDeliveryRouteRequest) (DeliveryRouteView, error) {
	if r.db == nil {
		return DeliveryRouteView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateDeliveryRouteRequest(req); err != nil {
		return DeliveryRouteView{}, err
	}
	domainPattern, err := normalizeDeliveryRouteDomainPattern(req.DomainPattern)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	hosts, err := normalizeDeliveryRouteHosts(req.Hosts)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	tlsMode, err := normalizeDeliveryRouteTLSMode(req.TLSMode)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	port := req.Port
	if port == 0 {
		port = 25
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return DeliveryRouteView{}, fmt.Errorf("begin delivery route create transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO delivery_routes (
  domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, auth_password,
  description
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING
  id::text, domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, status, description,
  created_at, updated_at`

	var route DeliveryRouteView
	if err := tx.QueryRowContext(
		ctx,
		query,
		domainPattern,
		strings.TrimSpace(req.Farm),
		stringArray(hosts),
		port,
		tlsMode,
		req.ImplicitTLS,
		strings.TrimSpace(req.SMTPHello),
		strings.TrimSpace(req.PoolName),
		strings.TrimSpace(req.AuthIdentity),
		strings.TrimSpace(req.AuthUsername),
		strings.TrimSpace(req.AuthPassword),
		strings.TrimSpace(req.Description),
	).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		return DeliveryRouteView{}, fmt.Errorf("create delivery route: %w", err)
	}
	detail, err := deliveryRouteAuditDetail(route)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "delivery_route.create",
		TargetType: "delivery_route",
		TargetID:   route.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return DeliveryRouteView{}, fmt.Errorf("record delivery route create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DeliveryRouteView{}, fmt.Errorf("commit delivery route create transaction: %w", err)
	}
	return route, nil
}

func (r *Repository) UpdateDeliveryRouteStatus(ctx context.Context, req UpdateDeliveryRouteStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDeliveryRouteStatusRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delivery route status transaction: %w", err)
	}
	defer tx.Rollback()

	var route DeliveryRouteView
	if err := tx.QueryRowContext(ctx, `
UPDATE delivery_routes
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING
  id::text, domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, status, description,
  created_at, updated_at`, strings.TrimSpace(req.ID), strings.ToLower(strings.TrimSpace(req.Status))).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("delivery route %q not found", req.ID)
		}
		return fmt.Errorf("update delivery route status: %w", err)
	}
	detail, err := deliveryRouteAuditDetail(route)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "delivery_route.status_update",
		TargetType: "delivery_route",
		TargetID:   route.ID,
		Result:     route.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record delivery route status audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delivery route status transaction: %w", err)
	}
	return nil
}

func (r *Repository) DeliveryRouteForDomain(ctx context.Context, domain string) (DeliveryRouteView, error) {
	if r.db == nil {
		return DeliveryRouteView{}, fmt.Errorf("database handle is required")
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !validAdminDomainName(domain) {
		return DeliveryRouteView{}, fmt.Errorf("domain must be a domain name")
	}

	const query = `
SELECT
  id::text,
  domain_pattern,
  farm,
  hosts,
  port,
  tls_mode,
  implicit_tls,
  smtp_hello,
  pool_name,
  auth_identity,
  auth_username,
  auth_password,
  status,
  description,
  created_at,
  updated_at
FROM delivery_routes
WHERE status = 'active'
  AND (
    domain_pattern = $1
    OR domain_pattern = '*'
    OR (
      left(domain_pattern, 2) = '*.'
      AND right($1, length(domain_pattern) - 1) = substring(domain_pattern from 2)
    )
  )
ORDER BY
  CASE
    WHEN domain_pattern = $1 THEN 0
    WHEN left(domain_pattern, 2) = '*.' THEN 1
    ELSE 2
  END,
  length(domain_pattern) DESC,
  created_at DESC
LIMIT 1`

	var route DeliveryRouteView
	if err := r.db.QueryRowContext(ctx, query, domain).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.AuthPassword,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DeliveryRouteView{}, ErrDeliveryRouteNotFound
		}
		return DeliveryRouteView{}, fmt.Errorf("get delivery route for domain: %w", err)
	}
	return route, nil
}

func (r *Repository) ResolveDeliveryRoute(ctx context.Context, domain string) (DeliveryRouteResolveView, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !validAdminDomainName(domain) {
		return DeliveryRouteResolveView{}, fmt.Errorf("domain must be a domain name")
	}
	route, err := r.DeliveryRouteForDomain(ctx, domain)
	if err != nil {
		if errors.Is(err, ErrDeliveryRouteNotFound) {
			return DeliveryRouteResolveView{Domain: domain, Matched: false}, nil
		}
		return DeliveryRouteResolveView{}, err
	}
	return DeliveryRouteResolveView{Domain: domain, Matched: true, Route: &route}, nil
}

func (r *Repository) DeleteDeliveryRoute(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("delivery route id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delivery route delete transaction: %w", err)
	}
	defer tx.Rollback()

	var route DeliveryRouteView
	if err := tx.QueryRowContext(ctx, `
SELECT
  id::text, domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, status, description,
  created_at, updated_at
FROM delivery_routes
WHERE id = $1
FOR UPDATE`, id).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("delivery route %q not found", id)
		}
		return fmt.Errorf("read delivery route for deletion: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM delivery_routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete delivery route: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("delivery route %q not found", id)
	}
	detail, err := deliveryRouteAuditDetail(route)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "delivery_route.delete",
		TargetType: "delivery_route",
		TargetID:   route.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record delivery route delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delivery route delete transaction: %w", err)
	}
	return nil
}

func deliveryRouteAuditDetail(route DeliveryRouteView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"delivery_route_id": route.ID,
		"domain_pattern":    route.DomainPattern,
		"farm":              route.Farm,
		"hosts":             route.Hosts,
		"port":              route.Port,
		"tls_mode":          route.TLSMode,
		"implicit_tls":      route.ImplicitTLS,
		"smtp_hello":        route.SMTPHello,
		"pool_name":         route.PoolName,
		"auth_identity":     route.AuthIdentity,
		"auth_username":     route.AuthUsername,
		"status":            route.Status,
		"description":       route.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal delivery route audit detail: %w", err)
	}
	return detail, nil
}
