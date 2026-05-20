package rdbms

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/idprovider"
)

type scriptedRDBMSDriver struct {
	mu      sync.Mutex
	scripts []scriptedRDBMSQuery
}

type scriptedRDBMSQuery struct {
	query   string
	columns []string
	rows    [][]driver.Value
}

func registerScriptedRDBMSDriver(t *testing.T, scripts []scriptedRDBMSQuery) *sql.DB {
	t.Helper()

	driverName := fmt.Sprintf("gogomail_rdbms_test_%d", time.Now().UnixNano())
	sql.Register(driverName, &scriptedRDBMSDriver{scripts: scripts})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func (d *scriptedRDBMSDriver) Open(string) (driver.Conn, error) {
	return &scriptedRDBMSConn{driver: d}, nil
}

type scriptedRDBMSConn struct {
	driver *scriptedRDBMSDriver
}

func (c *scriptedRDBMSConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare not implemented")
}

func (c *scriptedRDBMSConn) Close() error { return nil }

func (c *scriptedRDBMSConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("begin not implemented")
}

func (c *scriptedRDBMSConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := normalizeSQLForTest(query)
	c.driver.mu.Lock()
	defer c.driver.mu.Unlock()
	for _, script := range c.driver.scripts {
		if normalizeSQLForTest(script.query) == normalized {
			return &scriptedRDBMSRows{columns: script.columns, rows: script.rows}, nil
		}
	}
	return nil, fmt.Errorf("unexpected query: %s", normalized)
}

func (c *scriptedRDBMSConn) Ping(context.Context) error { return nil }

var _ driver.QueryerContext = (*scriptedRDBMSConn)(nil)
var _ driver.Pinger = (*scriptedRDBMSConn)(nil)

type scriptedRDBMSRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *scriptedRDBMSRows) Columns() []string { return r.columns }
func (r *scriptedRDBMSRows) Close() error      { return nil }

func (r *scriptedRDBMSRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.index]
	r.index++
	copy(dest, row)
	return nil
}

func normalizeSQLForTest(query string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(strings.TrimSuffix(query, ";"))), " ")
}

func TestGetUserMapsFieldsFromConfiguredQuery(t *testing.T) {
	db := registerScriptedRDBMSDriver(t, []scriptedRDBMSQuery{
		{
			query: "SELECT employee_id, email_address, full_name, recovery_email, org_id, profile_json, created_at, updated_at FROM employees",
			columns: []string{
				"employee_id",
				"email_address",
				"full_name",
				"recovery_email",
				"org_id",
				"profile_json",
				"created_at",
				"updated_at",
			},
			rows: [][]driver.Value{
				{
					"emp-1",
					"alice@example.com",
					"Alice Example",
					"alice.recovery@example.com",
					"org-9",
					driver.Value([]byte(`{"region":"apac","level":3}`)),
					time.Date(2026, time.May, 15, 10, 0, 0, 0, time.UTC),
					time.Date(2026, time.May, 15, 10, 30, 0, 0, time.UTC),
				},
			},
		},
	})

	p := New(&Config{
		UserQuery: "SELECT employee_id, email_address, full_name, recovery_email, org_id, profile_json, created_at, updated_at FROM employees",
		FieldMap: map[string]string{
			"id":             "employee_id",
			"username":       "email_address",
			"display_name":   "full_name",
			"recovery_email": "recovery_email",
			"org_id":         "org_id",
			"settings":       "profile_json",
			"created_at":     "created_at",
			"updated_at":     "updated_at",
		},
	})
	p.db = db

	got, err := p.GetUser(context.Background(), "emp-1")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got.ID != "emp-1" || got.Username != "alice@example.com" || got.DisplayName != "Alice Example" {
		t.Fatalf("GetUser() = %+v", got)
	}
	if got.OrgID == nil || *got.OrgID != "org-9" {
		t.Fatalf("GetUser() org_id = %+v", got.OrgID)
	}
	if got.Settings["region"] != "apac" {
		t.Fatalf("GetUser() settings = %+v", got.Settings)
	}
}

func TestListUsersAppliesSearchAndOrgFilters(t *testing.T) {
	db := registerScriptedRDBMSDriver(t, []scriptedRDBMSQuery{
		{
			query: "SELECT employee_id, email_address, full_name, org_id, profile_json FROM employees",
			columns: []string{
				"employee_id",
				"email_address",
				"full_name",
				"org_id",
				"profile_json",
			},
			rows: [][]driver.Value{
				{"emp-1", "alice@example.com", "Alice Example", "org-9", driver.Value([]byte(`{"active":true}`))},
				{"emp-2", "bob@example.com", "Bob Builder", "org-10", driver.Value([]byte(`{"active":false}`))},
				{"emp-3", "brenda@example.com", "Brenda Beta", "org-9", driver.Value([]byte(`{"active":true}`))},
			},
		},
	})

	orgID := "org-9"
	search := "beta"
	p := New(&Config{
		UserQuery: "SELECT employee_id, email_address, full_name, org_id, profile_json FROM employees",
		FieldMap: map[string]string{
			"id":           "employee_id",
			"username":     "email_address",
			"display_name": "full_name",
			"org_id":       "org_id",
			"settings":     "profile_json",
		},
	})
	p.db = db

	got, err := p.ListUsers(context.Background(), &idprovider.UserFilter{
		OrgID:       &orgID,
		SearchQuery: &search,
		Limit:       1,
		Offset:      0,
	})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListUsers() len = %d, want 1", len(got))
	}
	if got[0].ID != "emp-3" || got[0].DisplayName != "Brenda Beta" {
		t.Fatalf("ListUsers() = %+v", got[0])
	}
}

func TestListUsersPushesPlainPaginationToSourceQuery(t *testing.T) {
	db := registerScriptedRDBMSDriver(t, []scriptedRDBMSQuery{
		{
			query: "SELECT * FROM (SELECT employee_id, email_address, full_name FROM employees) AS gogomail_rdbms_source LIMIT $1 OFFSET $2",
			columns: []string{
				"employee_id",
				"email_address",
				"full_name",
			},
			rows: [][]driver.Value{
				{"emp-2", "bob@example.com", "Bob Builder"},
			},
		},
	})

	p := New(&Config{
		UserQuery: "SELECT employee_id, email_address, full_name FROM employees",
		FieldMap: map[string]string{
			"id":           "employee_id",
			"username":     "email_address",
			"display_name": "full_name",
		},
	})
	p.db = db

	got, err := p.ListUsers(context.Background(), &idprovider.UserFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "emp-2" {
		t.Fatalf("ListUsers() = %+v, want emp-2 only", got)
	}
}

func TestListGroupsPushesPlainPaginationToSourceQuery(t *testing.T) {
	db := registerScriptedRDBMSDriver(t, []scriptedRDBMSQuery{
		{
			query: "SELECT * FROM (SELECT group_id, group_name, group_slug FROM groups) AS gogomail_rdbms_source LIMIT $1 OFFSET $2",
			columns: []string{
				"group_id",
				"group_name",
				"group_slug",
			},
			rows: [][]driver.Value{
				{"group-2", "Sales", "sales"},
			},
		},
	})

	p := New(&Config{
		GroupQuery: "SELECT group_id, group_name, group_slug FROM groups;",
		FieldMap: map[string]string{
			"id":   "group_id",
			"name": "group_name",
			"slug": "group_slug",
		},
	})
	p.db = db

	got, err := p.ListGroups(context.Background(), &idprovider.GroupFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListGroups() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "group-2" {
		t.Fatalf("ListGroups() = %+v, want group-2 only", got)
	}
}

func TestSyncUsersCountsFetchedRows(t *testing.T) {
	db := registerScriptedRDBMSDriver(t, []scriptedRDBMSQuery{
		{
			query: "SELECT employee_id, email_address, full_name FROM employees",
			columns: []string{
				"employee_id",
				"email_address",
				"full_name",
			},
			rows: [][]driver.Value{
				{"emp-1", "alice@example.com", "Alice Example"},
				{"emp-2", "bob@example.com", "Bob Builder"},
			},
		},
	})

	p := New(&Config{
		UserQuery: "SELECT employee_id, email_address, full_name FROM employees",
		FieldMap: map[string]string{
			"id":           "employee_id",
			"username":     "email_address",
			"display_name": "full_name",
		},
	})
	p.db = db

	result, err := p.SyncUsers(context.Background(), SyncRequest{DomainID: "domain-1"})
	if err != nil {
		t.Fatalf("SyncUsers() error = %v", err)
	}
	if result.UsersCreated != 2 {
		t.Fatalf("SyncUsers() UsersCreated = %d, want 2", result.UsersCreated)
	}
}

func TestSyncGroupsCountsFetchedRows(t *testing.T) {
	db := registerScriptedRDBMSDriver(t, []scriptedRDBMSQuery{
		{
			query: "SELECT group_id, group_name, group_slug FROM groups",
			columns: []string{
				"group_id",
				"group_name",
				"group_slug",
			},
			rows: [][]driver.Value{
				{"group-1", "Engineering", "engineering"},
				{"group-2", "Sales", "sales"},
			},
		},
	})

	p := New(&Config{
		GroupQuery: "SELECT group_id, group_name, group_slug FROM groups",
		FieldMap: map[string]string{
			"id":   "group_id",
			"name": "group_name",
			"slug": "group_slug",
		},
	})
	p.db = db

	result, err := p.SyncGroups(context.Background(), SyncRequest{DomainID: "domain-1"})
	if err != nil {
		t.Fatalf("SyncGroups() error = %v", err)
	}
	if result.GroupsCreated != 2 {
		t.Fatalf("SyncGroups() GroupsCreated = %d, want 2", result.GroupsCreated)
	}
}

func TestSyncMembershipsUnsupportedWithConfiguredProvider(t *testing.T) {
	p := New(&Config{
		UserQuery:  "SELECT employee_id, email_address, full_name FROM employees",
		GroupQuery: "SELECT group_id, group_name, group_slug FROM groups",
		FieldMap: map[string]string{
			"id":           "employee_id",
			"username":     "email_address",
			"display_name": "full_name",
			"name":         "group_name",
			"slug":         "group_slug",
		},
	})
	p.db = registerScriptedRDBMSDriver(t, nil)

	_, err := p.SyncMemberships(context.Background(), SyncRequest{DomainID: "domain-1"})
	if !errors.Is(err, ErrMembershipSyncUnsupported) {
		t.Fatalf("SyncMemberships() error = %v, want ErrMembershipSyncUnsupported", err)
	}
}
