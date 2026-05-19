package configstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrConfigNotFound  = errors.New("config not found")
	ErrConfigLocked    = errors.New("config is locked")
	ErrVersionConflict = errors.New("version conflict")
)

type ScopeType string

const (
	ScopeCompany ScopeType = "company"
	ScopeDomain  ScopeType = "domain"
	ScopeUser    ScopeType = "user"
)

type ConfigEntry struct {
	ID        string
	ScopeType ScopeType
	ScopeID   string
	Key       string
	Value     json.RawMessage
	Locked    bool
	Version   int64
	UpdatedAt string
}

type ConfigChangeEvent struct {
	ScopeType ScopeType
	ScopeID   string
	Key       string
	Action    string
}

type Notifier interface {
	Subscribe() chan ConfigChangeEvent
	Unsubscribe(ch chan ConfigChangeEvent)
}

type ConfigStore interface {
	Resolve(ctx context.Context, userID, domainID, companyID string, key string) (json.RawMessage, error)
	Get(ctx context.Context, scopeType ScopeType, scopeID, key string) (*ConfigEntry, error)
	Set(ctx context.Context, scopeType ScopeType, scopeID, key string, value json.RawMessage, locked bool, expectedVersion int64) (*ConfigEntry, error)
	Delete(ctx context.Context, scopeType ScopeType, scopeID, key string, expectedVersion int64) error
	List(ctx context.Context, scopeType ScopeType, scopeID string) ([]ConfigEntry, error)
	Propagate(ctx context.Context, companyID string, scope PropagateScope, key string, value json.RawMessage, locked bool) error
	PropagateFromParent(ctx context.Context, scopeType ScopeType, scopeID string, parentScopeType ScopeType, parentScopeID string) error
	Notify(ctx context.Context, event ConfigChangeEvent)
	Close() error
}

type PropagateScope string

const (
	PropagateSubtree  PropagateScope = "subtree"
	PropagateChildren PropagateScope = "children"
	PropagateDomains  PropagateScope = "domains"
)

func (s PropagateScope) IsValid() bool {
	switch s {
	case PropagateSubtree, PropagateChildren, PropagateDomains:
		return true
	}
	return false
}

func (s ScopeType) IsValid() bool {
	switch s {
	case ScopeCompany, ScopeDomain, ScopeUser:
		return true
	}
	return false
}

type PostgresConfigStore struct {
	db          *sql.DB
	mu          sync.RWMutex
	cache       map[string]map[string]*ConfigEntry // "scopeType:scopeID" → key → entry
	companyTree map[string][]string                // parentID → []childID
	parentOf    map[string]string                  // childID → parentID
	subscribers []chan ConfigChangeEvent
	subMu       sync.Mutex
	notifyChan  chan ConfigChangeEvent
	closed      bool
	closeWg     sync.WaitGroup
}

func NewPostgresConfigStore(db *sql.DB) *PostgresConfigStore {
	store := &PostgresConfigStore{
		db:          db,
		cache:       make(map[string]map[string]*ConfigEntry),
		companyTree: make(map[string][]string),
		parentOf:    make(map[string]string),
		notifyChan:  make(chan ConfigChangeEvent, 100),
	}
	return store
}

func (s *PostgresConfigStore) loadAll(ctx context.Context) error {
	const query = `
		SELECT id::text, scope_type, scope_id::text, key, value, locked, version, updated_at::text
		FROM runtime_config`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("load runtime config: %w", err)
	}
	defer rows.Close()

	newCache := make(map[string]map[string]*ConfigEntry)
	for rows.Next() {
		var entry ConfigEntry
		if err := rows.Scan(&entry.ID, &entry.ScopeType, &entry.ScopeID, &entry.Key, &entry.Value, &entry.Locked, &entry.Version, &entry.UpdatedAt); err != nil {
			return fmt.Errorf("scan config row: %w", err)
		}
		scopeKey := string(entry.ScopeType) + ":" + entry.ScopeID
		if newCache[scopeKey] == nil {
			newCache[scopeKey] = make(map[string]*ConfigEntry)
		}
		e := entry
		newCache[scopeKey][entry.Key] = &e
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}

	newTree, newParentOf, err := s.loadCompanyTree(ctx)
	if err != nil {
		return fmt.Errorf("load company tree: %w", err)
	}

	s.mu.Lock()
	s.cache = newCache
	s.companyTree = newTree
	s.parentOf = newParentOf
	s.mu.Unlock()

	return nil
}

func (s *PostgresConfigStore) loadCompanyTree(ctx context.Context) (tree map[string][]string, parentOf map[string]string, err error) {
	const query = `SELECT id::text, parent_id::text FROM companies WHERE parent_id IS NOT NULL`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("load company tree: %w", err)
	}
	defer rows.Close()

	tree = make(map[string][]string)
	parentOf = make(map[string]string)
	for rows.Next() {
		var id, parentID string
		if err := rows.Scan(&id, &parentID); err != nil {
			return nil, nil, fmt.Errorf("scan company row: %w", err)
		}
		tree[parentID] = append(tree[parentID], id)
		parentOf[id] = parentID
	}

	return tree, parentOf, rows.Err()
}

// buildCompanyChain returns [companyID, parentID, grandparentID, ...] up to the root.
// The lock is expected to be held by the caller (read or write).
func (s *PostgresConfigStore) buildCompanyChain(companyID string) []string {
	if companyID == "" {
		return nil
	}
	var chain []string
	seen := make(map[string]bool) // cycle protection
	current := companyID
	for current != "" && !seen[current] {
		chain = append(chain, current)
		seen[current] = true
		current = s.parentOf[current]
	}
	return chain
}

func (s *PostgresConfigStore) Start(ctx context.Context) error {
	if err := s.loadAll(ctx); err != nil {
		return err
	}

	s.closeWg.Add(1)
	go s.listenNotifications(ctx)

	return nil
}

func (s *PostgresConfigStore) listenNotifications(ctx context.Context) {
	defer s.closeWg.Done()

	for {
		if ctx.Err() != nil {
			return
		}
		s.runListenLoop(ctx)

		// Back off before reconnecting unless context is done.
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// runListenLoop acquires a dedicated connection from the pool, issues LISTEN,
// and blocks in WaitForNotification until the context is cancelled or an error
// occurs. On each notification it reloads the full config cache.
func (s *PostgresConfigStore) runListenLoop(ctx context.Context) {
	sqlConn, err := s.db.Conn(ctx)
	if err != nil {
		return
	}
	defer sqlConn.Close()

	_ = sqlConn.Raw(func(dc any) error {
		pgxConn, ok := dc.(*stdlib.Conn)
		if !ok {
			return fmt.Errorf("not a pgx conn")
		}
		conn := pgxConn.Conn()

		if _, err := conn.Exec(ctx, "LISTEN config_changed"); err != nil {
			return err
		}

		for {
			if _, err := conn.WaitForNotification(ctx); err != nil {
				return err
			}
			// Reload on any notification; payload is not used for targeted refresh.
			_ = s.loadAll(ctx)
		}
	})
}

// Resolve returns the effective value for key by walking the config hierarchy
// from the most ancestral scope (root company) down to the most specific
// (user). The first locked entry encountered stops the walk — no lower scope
// can override a locked value. Among non-locked entries, the most specific
// scope wins.
func (s *PostgresConfigStore) Resolve(ctx context.Context, userID, domainID, companyID, key string) (json.RawMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// companyChain: [companyID, parentID, grandparentID, ..., rootID]
	companyChain := s.buildCompanyChain(companyID)

	// Build the lookup order from root (most general) to leaf (most specific).
	type scopeRef struct {
		scopeType ScopeType
		scopeID   string
	}
	scopes := make([]scopeRef, 0, len(companyChain)+2)
	for i := len(companyChain) - 1; i >= 0; i-- {
		scopes = append(scopes, scopeRef{ScopeCompany, companyChain[i]})
	}
	if domainID != "" {
		scopes = append(scopes, scopeRef{ScopeDomain, domainID})
	}
	if userID != "" {
		scopes = append(scopes, scopeRef{ScopeUser, userID})
	}

	// Walk root → leaf. A locked entry stops the walk immediately.
	// Among non-locked entries, the last one visited (most specific) wins.
	var current json.RawMessage
	for _, sc := range scopes {
		if sc.scopeID == "" {
			continue
		}
		scopeKey := string(sc.scopeType) + ":" + sc.scopeID
		if entries, ok := s.cache[scopeKey]; ok {
			if entry, ok := entries[key]; ok {
				current = entry.Value
				if entry.Locked {
					return current, nil
				}
			}
		}
	}

	if current != nil {
		return current, nil
	}
	return nil, ErrConfigNotFound
}

func (s *PostgresConfigStore) Get(ctx context.Context, scopeType ScopeType, scopeID, key string) (*ConfigEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scopeKey := string(scopeType) + ":" + scopeID
	if entries, ok := s.cache[scopeKey]; ok {
		if entry, ok := entries[key]; ok {
			cp := *entry
			return &cp, nil
		}
	}
	return nil, ErrConfigNotFound
}

func (s *PostgresConfigStore) Set(ctx context.Context, scopeType ScopeType, scopeID, key string, value json.RawMessage, locked bool, expectedVersion int64) (*ConfigEntry, error) {
	const upsertQuery = `
		INSERT INTO runtime_config (scope_type, scope_id, key, value, locked, version)
		VALUES ($1, $2::uuid, $3, $4, $5, 1)
		ON CONFLICT (scope_type, scope_id, key) DO UPDATE SET
			value      = EXCLUDED.value,
			locked     = EXCLUDED.locked,
			version    = runtime_config.version + 1,
			updated_at = now()
		WHERE runtime_config.version = $6 OR $6 = -1
		RETURNING id::text, scope_type, scope_id::text, key, value, locked, version, updated_at::text`

	var entry ConfigEntry
	err := s.db.QueryRowContext(ctx, upsertQuery,
		string(scopeType), scopeID, key, value, locked, expectedVersion,
	).Scan(&entry.ID, &entry.ScopeType, &entry.ScopeID, &entry.Key, &entry.Value, &entry.Locked, &entry.Version, &entry.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) && expectedVersion != -1 {
			return nil, ErrVersionConflict
		}
		return nil, fmt.Errorf("set config: %w", err)
	}

	action := "updated"
	if entry.Version == 1 {
		action = "created"
	}
	s.writeChangeLog(ctx, scopeType, scopeID, key, nil, value, action, "")

	s.mu.Lock()
	scopeKey := string(scopeType) + ":" + scopeID
	if s.cache[scopeKey] == nil {
		s.cache[scopeKey] = make(map[string]*ConfigEntry)
	}
	cp := entry
	s.cache[scopeKey][key] = &cp
	s.mu.Unlock()

	s.Notify(ctx, ConfigChangeEvent{
		ScopeType: scopeType,
		ScopeID:   scopeID,
		Key:       key,
		Action:    action,
	})

	return &entry, nil
}

func (s *PostgresConfigStore) Delete(ctx context.Context, scopeType ScopeType, scopeID, key string, expectedVersion int64) error {
	const checkQuery = `
		SELECT locked, value FROM runtime_config
		WHERE scope_type = $1 AND scope_id = $2::uuid AND key = $3`

	var locked bool
	var oldValue json.RawMessage
	if err := s.db.QueryRowContext(ctx, checkQuery, string(scopeType), scopeID, key).Scan(&locked, &oldValue); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConfigNotFound
		}
		return fmt.Errorf("check config: %w", err)
	}
	if locked {
		return ErrConfigLocked
	}

	const deleteQuery = `
		DELETE FROM runtime_config
		WHERE scope_type = $1 AND scope_id = $2::uuid AND key = $3 AND (version = $4 OR $4 = -1)
		RETURNING id`

	var id string
	err := s.db.QueryRowContext(ctx, deleteQuery, string(scopeType), scopeID, key, expectedVersion).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if expectedVersion == -1 {
				return fmt.Errorf("delete config: not found")
			}
			return ErrVersionConflict
		}
		return fmt.Errorf("delete config: %w", err)
	}

	s.writeChangeLog(ctx, scopeType, scopeID, key, oldValue, nil, "deleted", "")

	s.mu.Lock()
	scopeKey := string(scopeType) + ":" + scopeID
	if entries, ok := s.cache[scopeKey]; ok {
		delete(entries, key)
	}
	s.mu.Unlock()

	s.Notify(ctx, ConfigChangeEvent{
		ScopeType: scopeType,
		ScopeID:   scopeID,
		Key:       key,
		Action:    "deleted",
	})

	return nil
}

func (s *PostgresConfigStore) List(ctx context.Context, scopeType ScopeType, scopeID string) ([]ConfigEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scopeKey := string(scopeType) + ":" + scopeID
	if entries, ok := s.cache[scopeKey]; ok {
		result := make([]ConfigEntry, 0, len(entries))
		for _, entry := range entries {
			result = append(result, *entry)
		}
		return result, nil
	}
	return nil, nil
}

// Propagate writes key/value/locked to all target scopes determined by scope:
//   - PropagateSubtree:  company + all sub-companies + all their domains
//   - PropagateChildren: immediate child companies only
//   - PropagateDomains:  direct domains of companyID only
func (s *PostgresConfigStore) Propagate(ctx context.Context, companyID string, scope PropagateScope, key string, value json.RawMessage, locked bool) error {
	type target struct {
		scopeType ScopeType
		scopeID   string
	}
	var targets []target

	s.mu.RLock()
	switch scope {
	case PropagateSubtree:
		for _, cID := range s.collectSubtree(companyID) {
			targets = append(targets, target{ScopeCompany, cID})
		}
	case PropagateChildren:
		for _, cID := range s.companyTree[companyID] {
			targets = append(targets, target{ScopeCompany, cID})
		}
	case PropagateDomains:
		// Domain scope IDs are looked up from the DB below; nothing from the cache here.
	}
	s.mu.RUnlock()

	if scope == PropagateSubtree || scope == PropagateDomains {
		var companyIDs []string
		if scope == PropagateDomains {
			companyIDs = []string{companyID}
		} else {
			s.mu.RLock()
			companyIDs = s.collectSubtree(companyID)
			s.mu.RUnlock()
		}
		for _, cID := range companyIDs {
			domains, err := s.queryDomainsByCompany(ctx, cID)
			if err != nil {
				return fmt.Errorf("propagate: query domains for company %s: %w", cID, err)
			}
			for _, dID := range domains {
				targets = append(targets, target{ScopeDomain, dID})
			}
		}
	}

	for _, t := range targets {
		if _, err := s.Set(ctx, t.scopeType, t.scopeID, key, value, locked, -1); err != nil {
			return fmt.Errorf("propagate to %s %s: %w", t.scopeType, t.scopeID, err)
		}
	}
	return nil
}

func (s *PostgresConfigStore) collectSubtree(companyID string) []string {
	var result []string
	queue := []string{companyID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		queue = append(queue, s.companyTree[current]...)
	}
	return result
}

// PropagateFromParent copies unlocked entries from parent to child on creation.
func (s *PostgresConfigStore) PropagateFromParent(ctx context.Context, scopeType ScopeType, scopeID string, parentScopeType ScopeType, parentScopeID string) error {
	s.mu.RLock()
	parentKey := string(parentScopeType) + ":" + parentScopeID
	parentEntries, ok := s.cache[parentKey]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	for key, parentEntry := range parentEntries {
		if parentEntry.Locked {
			continue
		}
		if _, err := s.Set(ctx, scopeType, scopeID, key, parentEntry.Value, false, -1); err != nil {
			return fmt.Errorf("propagate from parent %s/%s key %s: %w", parentScopeType, parentScopeID, key, err)
		}
	}
	return nil
}

func (s *PostgresConfigStore) queryDomainsByCompany(ctx context.Context, companyID string) ([]string, error) {
	const q = `SELECT id::text FROM domains WHERE company_id = $1::uuid AND status = 'active'`
	rows, err := s.db.QueryContext(ctx, q, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// writeChangeLog writes a config_change_log row. Errors are swallowed to keep
// the main write path from failing on audit side effects.
func (s *PostgresConfigStore) writeChangeLog(ctx context.Context, scopeType ScopeType, scopeID, key string, oldValue, newValue json.RawMessage, action, changedBy string) {
	const q = `
		INSERT INTO config_change_log (scope_type, scope_id, key, old_value, new_value, changed_by, action)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, $7)`
	var oldV, newV interface{}
	if oldValue != nil {
		oldV = []byte(oldValue)
	}
	if newValue != nil {
		newV = []byte(newValue)
	}
	var by interface{}
	if changedBy != "" {
		by = changedBy
	}
	_, _ = s.db.ExecContext(ctx, q, string(scopeType), scopeID, key, oldV, newV, by, action)
}

func (s *PostgresConfigStore) Notify(ctx context.Context, event ConfigChangeEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *PostgresConfigStore) Subscribe() chan ConfigChangeEvent {
	ch := make(chan ConfigChangeEvent, 100)
	s.subMu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.subMu.Unlock()
	return ch
}

func (s *PostgresConfigStore) Unsubscribe(ch chan ConfigChangeEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

func (s *PostgresConfigStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	s.closeWg.Wait()
	close(s.notifyChan)

	s.subMu.Lock()
	for _, ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = nil
	s.subMu.Unlock()

	return nil
}

// Ensure *PostgresConfigStore satisfies both ConfigStore and Notifier.
var _ ConfigStore = (*PostgresConfigStore)(nil)
var _ Notifier = (*PostgresConfigStore)(nil)
