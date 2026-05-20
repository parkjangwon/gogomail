package rdbms

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/idprovider"
)

// Config represents external RDBMS configuration for user/group sync.
type Config struct {
	ConnectionString string                 `json:"connection_string"` // DSN: user:pass@host:port/db
	MaxPoolSize      int                    `json:"max_pool_size"`
	UserQuery        string                 `json:"user_query"`  // SQL query to fetch users
	GroupQuery       string                 `json:"group_query"` // SQL query to fetch groups
	FieldMap         map[string]string      `json:"field_map"`   // SQL column -> User/Group field mapping
	Settings         map[string]interface{} `json:"settings"`
	ValidatedAt      time.Time              `json:"validated_at"`
}

// Provider implements idprovider.IdentityProvider using external RDBMS.
type Provider struct {
	config *Config
	db     *sql.DB
}

// New creates a new RDBMS identity provider.
func New(cfg *Config) *Provider {
	return &Provider{config: cfg}
}

// Connect establishes a connection to the external RDBMS using the configured connection string.
func (p *Provider) Connect() error {
	if p.config == nil {
		return fmt.Errorf("rdbms provider not configured")
	}
	if p.config.ConnectionString == "" {
		return fmt.Errorf("connection string required")
	}

	db, err := sql.Open("postgres", p.config.ConnectionString)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if p.config.MaxPoolSize > 0 {
		db.SetMaxOpenConns(p.config.MaxPoolSize)
	}

	p.db = db
	return nil
}

// Close closes the connection to the external RDBMS.
func (p *Provider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// GetUser retrieves a user from the external RDBMS by ID.
func (p *Provider) GetUser(ctx context.Context, userID string) (*idprovider.User, error) {
	if userID == "" {
		return nil, fmt.Errorf("invalid user id")
	}
	if p.config == nil {
		return nil, fmt.Errorf("rdbms provider not configured")
	}
	if p.db == nil {
		return nil, fmt.Errorf("rdbms provider not connected")
	}

	users, err := p.ListUsers(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if user != nil && user.ID == userID {
			return user, nil
		}
	}
	return nil, sql.ErrNoRows
}

// GetGroup retrieves a group from the external RDBMS by ID.
func (p *Provider) GetGroup(ctx context.Context, groupID string) (*idprovider.Group, error) {
	if groupID == "" {
		return nil, fmt.Errorf("invalid group id")
	}
	if p.config == nil {
		return nil, fmt.Errorf("rdbms provider not configured")
	}
	if p.db == nil {
		return nil, fmt.Errorf("rdbms provider not connected")
	}

	groups, err := p.ListGroups(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group != nil && group.ID == groupID {
			return group, nil
		}
	}
	return nil, sql.ErrNoRows
}

// ListUsers lists users from the external RDBMS matching the filter.
func (p *Provider) ListUsers(ctx context.Context, filter *idprovider.UserFilter) ([]*idprovider.User, error) {
	if p.config == nil {
		return nil, fmt.Errorf("rdbms provider not configured")
	}
	if p.db == nil {
		return nil, fmt.Errorf("rdbms provider not connected")
	}

	query, args, paginatedByDB := buildPaginatedSourceQuery(p.config.UserQuery, userFilterLimit(filter), userFilterOffset(filter), userFilterCanUseDBPagination(filter))
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users, err := p.scanUsers(rows)
	if err != nil {
		return nil, err
	}

	users = filterUsers(users, filter)
	if paginatedByDB {
		return users, nil
	}
	return paginateUsers(users, filter), nil
}

// ListGroups lists groups from the external RDBMS matching the filter.
func (p *Provider) ListGroups(ctx context.Context, filter *idprovider.GroupFilter) ([]*idprovider.Group, error) {
	if p.config == nil {
		return nil, fmt.Errorf("rdbms provider not configured")
	}
	if p.db == nil {
		return nil, fmt.Errorf("rdbms provider not connected")
	}

	query, args, paginatedByDB := buildPaginatedSourceQuery(p.config.GroupQuery, groupFilterLimit(filter), groupFilterOffset(filter), groupFilterCanUseDBPagination(filter))
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	groups, err := p.scanGroups(rows)
	if err != nil {
		return nil, err
	}

	groups = filterGroups(groups, filter)
	if paginatedByDB {
		return groups, nil
	}
	return paginateGroups(groups, filter), nil
}

// CreateUser creates a new user (RDBMS is read-only, returns error).
func (p *Provider) CreateUser(ctx context.Context, user *idprovider.User) error {
	if user == nil || user.DomainID == "" {
		return fmt.Errorf("invalid user: missing required fields")
	}
	return fmt.Errorf("rdbms provider is read-only")
}

// UpdateUser updates an existing user (RDBMS is read-only, returns error).
func (p *Provider) UpdateUser(ctx context.Context, user *idprovider.User) error {
	if user == nil || user.ID == "" {
		return fmt.Errorf("invalid user: missing id")
	}
	return fmt.Errorf("rdbms provider is read-only")
}

// DeleteUser deletes a user (RDBMS is read-only, returns error).
func (p *Provider) DeleteUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("invalid user id")
	}
	return fmt.Errorf("rdbms provider is read-only")
}

// CreateGroup creates a new group (RDBMS is read-only, returns error).
func (p *Provider) CreateGroup(ctx context.Context, group *idprovider.Group) error {
	if group == nil || group.DomainID == "" {
		return fmt.Errorf("invalid group: missing required fields")
	}
	return fmt.Errorf("rdbms provider is read-only")
}

// DeleteGroup deletes a group (RDBMS is read-only, returns error).
func (p *Provider) DeleteGroup(ctx context.Context, groupID string) error {
	if groupID == "" {
		return fmt.Errorf("invalid group id")
	}
	return fmt.Errorf("rdbms provider is read-only")
}

// AddMember adds a member to a group (RDBMS is read-only, returns error).
func (p *Provider) AddMember(ctx context.Context, groupID string, member *idprovider.Member) error {
	if groupID == "" || member == nil {
		return fmt.Errorf("invalid group id or member")
	}
	return fmt.Errorf("rdbms provider is read-only")
}

// RemoveMember removes a member from a group (RDBMS is read-only, returns error).
func (p *Provider) RemoveMember(ctx context.Context, groupID, memberID string) error {
	if groupID == "" || memberID == "" {
		return fmt.Errorf("invalid group id or member id")
	}
	return fmt.Errorf("rdbms provider is read-only")
}

func (p *Provider) scanUsers(rows *sql.Rows) ([]*idprovider.User, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read user columns: %w", err)
	}
	records, err := scanRows(rows, columns)
	if err != nil {
		return nil, err
	}
	users := make([]*idprovider.User, 0, len(records))
	for _, record := range records {
		user, err := p.mapUserRecord(record)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (p *Provider) scanGroups(rows *sql.Rows) ([]*idprovider.Group, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read group columns: %w", err)
	}
	records, err := scanRows(rows, columns)
	if err != nil {
		return nil, err
	}
	groups := make([]*idprovider.Group, 0, len(records))
	for _, record := range records {
		group, err := p.mapGroupRecord(record)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func (p *Provider) mapUserRecord(record map[string]any) (*idprovider.User, error) {
	user := &idprovider.User{Settings: map[string]any{}}
	user.ID = p.recordString(record, "id")
	user.DomainID = p.recordString(record, "domain_id")
	if org := p.recordString(record, "org_id"); org != "" {
		user.OrgID = &org
	}
	user.Username = p.recordString(record, "username")
	user.DisplayName = p.recordString(record, "display_name")
	user.RecoveryEmail = p.recordString(record, "recovery_email")
	user.AuthSource = p.recordString(record, "auth_source")
	user.Role = p.recordString(record, "role")
	user.Status = p.recordString(record, "status")
	if settings, ok := p.recordJSONMap(record, "settings"); ok {
		user.Settings = settings
	}
	if createdAt, ok := p.recordTime(record, "created_at"); ok {
		user.CreatedAt = createdAt
	}
	if updatedAt, ok := p.recordTime(record, "updated_at"); ok {
		user.UpdatedAt = updatedAt
	}
	return user, nil
}

func (p *Provider) mapGroupRecord(record map[string]any) (*idprovider.Group, error) {
	group := &idprovider.Group{Settings: map[string]any{}}
	group.ID = p.recordString(record, "id")
	group.DomainID = p.recordString(record, "domain_id")
	if org := p.recordString(record, "org_id"); org != "" {
		group.OrgID = &org
	}
	group.Name = p.recordString(record, "name")
	group.Slug = p.recordString(record, "slug")
	group.Description = p.recordString(record, "description")
	group.Status = p.recordString(record, "status")
	if settings, ok := p.recordJSONMap(record, "settings"); ok {
		group.Settings = settings
	}
	if createdAt, ok := p.recordTime(record, "created_at"); ok {
		group.CreatedAt = createdAt
	}
	if updatedAt, ok := p.recordTime(record, "updated_at"); ok {
		group.UpdatedAt = updatedAt
	}
	return group, nil
}

func (p *Provider) sourceColumn(field string) string {
	if p.config == nil || p.config.FieldMap == nil {
		return field
	}
	if column, ok := p.config.FieldMap[field]; ok && strings.TrimSpace(column) != "" {
		return column
	}
	return field
}

func (p *Provider) recordString(record map[string]any, field string) string {
	column := strings.ToLower(p.sourceColumn(field))
	value, ok := record[column]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case time.Time:
		return v.Format(time.RFC3339Nano)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func (p *Provider) recordTime(record map[string]any, field string) (time.Time, bool) {
	column := strings.ToLower(p.sourceColumn(field))
	value, ok := record[column]
	if !ok || value == nil {
		return time.Time{}, false
	}
	switch v := value.(type) {
	case time.Time:
		return v, true
	case string:
		return parseTimeValue(v)
	case []byte:
		return parseTimeValue(string(v))
	default:
		return time.Time{}, false
	}
}

func (p *Provider) recordJSONMap(record map[string]any, field string) (map[string]any, bool) {
	column := strings.ToLower(p.sourceColumn(field))
	value, ok := record[column]
	if !ok || value == nil {
		return nil, false
	}
	switch v := value.(type) {
	case map[string]any:
		return v, true
	case string:
		return parseJSONMap([]byte(v))
	case []byte:
		return parseJSONMap(v)
	default:
		return nil, false
	}
}

func scanRows(rows *sql.Rows, columns []string) ([]map[string]any, error) {
	records := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		record := make(map[string]any, len(columns))
		for i, column := range columns {
			record[strings.ToLower(column)] = normalizeScannedValue(values[i])
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return records, nil
}

func normalizeScannedValue(v any) any {
	switch value := v.(type) {
	case []byte:
		return string(value)
	default:
		return value
	}
}

func parseTimeValue(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func parseJSONMap(data []byte) (map[string]any, bool) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, true
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

func filterUsers(users []*idprovider.User, filter *idprovider.UserFilter) []*idprovider.User {
	if filter == nil {
		return users
	}
	search := strings.ToLower(strings.TrimSpace(safeString(filter.SearchQuery)))
	orgID := strings.TrimSpace(safeString(filter.OrgID))
	if search == "" && orgID == "" {
		return users
	}
	filtered := make([]*idprovider.User, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		if orgID != "" {
			if user.OrgID == nil || strings.TrimSpace(*user.OrgID) != orgID {
				continue
			}
		}
		if search != "" {
			haystack := strings.ToLower(strings.Join([]string{user.Username, user.DisplayName, user.RecoveryEmail}, " "))
			if !strings.Contains(haystack, search) {
				continue
			}
		}
		filtered = append(filtered, user)
	}
	return filtered
}

func paginateUsers(users []*idprovider.User, filter *idprovider.UserFilter) []*idprovider.User {
	if filter == nil {
		return users
	}
	return paginate(users, filter.Offset, filter.Limit)
}

func filterGroups(groups []*idprovider.Group, filter *idprovider.GroupFilter) []*idprovider.Group {
	if filter == nil {
		return groups
	}
	search := strings.ToLower(strings.TrimSpace(safeString(filter.SearchQuery)))
	orgID := strings.TrimSpace(safeString(filter.OrgID))
	if search == "" && orgID == "" {
		return groups
	}
	filtered := make([]*idprovider.Group, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		if orgID != "" {
			if group.OrgID == nil || strings.TrimSpace(*group.OrgID) != orgID {
				continue
			}
		}
		if search != "" {
			haystack := strings.ToLower(strings.Join([]string{group.Name, group.Slug, group.Description}, " "))
			if !strings.Contains(haystack, search) {
				continue
			}
		}
		filtered = append(filtered, group)
	}
	return filtered
}

func paginateGroups(groups []*idprovider.Group, filter *idprovider.GroupFilter) []*idprovider.Group {
	if filter == nil {
		return groups
	}
	return paginate(groups, filter.Offset, filter.Limit)
}

func buildPaginatedSourceQuery(sourceQuery string, limit int, offset int, eligible bool) (string, []any, bool) {
	if !eligible || limit <= 0 {
		return sourceQuery, nil, false
	}
	if offset < 0 {
		offset = 0
	}
	sourceQuery = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sourceQuery), ";"))
	query := fmt.Sprintf("SELECT * FROM (%s) AS gogomail_rdbms_source LIMIT $1 OFFSET $2", sourceQuery)
	return query, []any{limit, offset}, true
}

func userFilterCanUseDBPagination(filter *idprovider.UserFilter) bool {
	if filter == nil {
		return false
	}
	return strings.TrimSpace(safeString(filter.SearchQuery)) == "" && strings.TrimSpace(safeString(filter.OrgID)) == ""
}

func groupFilterCanUseDBPagination(filter *idprovider.GroupFilter) bool {
	if filter == nil {
		return false
	}
	return strings.TrimSpace(safeString(filter.SearchQuery)) == "" && strings.TrimSpace(safeString(filter.OrgID)) == ""
}

func userFilterLimit(filter *idprovider.UserFilter) int {
	if filter == nil {
		return 0
	}
	return filter.Limit
}

func groupFilterLimit(filter *idprovider.GroupFilter) int {
	if filter == nil {
		return 0
	}
	return filter.Limit
}

func userFilterOffset(filter *idprovider.UserFilter) int {
	if filter == nil {
		return 0
	}
	return filter.Offset
}

func groupFilterOffset(filter *idprovider.GroupFilter) int {
	if filter == nil {
		return 0
	}
	return filter.Offset
}

func paginate[T any](items []T, offset, limit int) []T {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []T{}
	}
	if limit <= 0 || offset+limit > len(items) {
		limit = len(items) - offset
	}
	return items[offset : offset+limit]
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
