package configstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrConfigNotFound = errors.New("config not found")
	ErrConfigLocked   = errors.New("config is locked")
	ErrVersionConflict = errors.New("version conflict")
)

type ScopeType string

const (
	ScopeCompany ScopeType = "company"
	ScopeDomain ScopeType = "domain"
	ScopeUser   ScopeType = "user"
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
	db           *sql.DB
	mu           sync.RWMutex
	cache        map[string]map[string]*ConfigEntry
	companyTree  map[string][]string
	subscribers  []chan ConfigChangeEvent
	subMu        sync.Mutex
	notifyChan   chan ConfigChangeEvent
	closed       bool
	closeWg      sync.WaitGroup
}

func NewPostgresConfigStore(db *sql.DB) *PostgresConfigStore {
	store := &PostgresConfigStore{
		db:          db,
		cache:       make(map[string]map[string]*ConfigEntry),
		companyTree: make(map[string][]string),
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

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache = make(map[string]map[string]*ConfigEntry)
	s.companyTree = make(map[string][]string)

	for rows.Next() {
		var entry ConfigEntry
		if err := rows.Scan(&entry.ID, &entry.ScopeType, &entry.ScopeID, &entry.Key, &entry.Value, &entry.Locked, &entry.Version, &entry.UpdatedAt); err != nil {
			return fmt.Errorf("scan config row: %w", err)
		}
		scopeKey := string(entry.ScopeType) + ":" + entry.ScopeID
		if s.cache[scopeKey] == nil {
			s.cache[scopeKey] = make(map[string]*ConfigEntry)
		}
		s.cache[scopeKey][entry.Key] = &entry
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}

	if err := s.loadCompanyTree(ctx); err != nil {
		return fmt.Errorf("load company tree: %w", err)
	}

	return nil
}

func (s *PostgresConfigStore) loadCompanyTree(ctx context.Context) error {
	const query = `
		SELECT id::text, parent_id::text
		FROM companies
		WHERE parent_id IS NOT NULL`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("load company tree: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, parentID string
		if err := rows.Scan(&id, &parentID); err != nil {
			return fmt.Errorf("scan company row: %w", err)
		}
		s.companyTree[parentID] = append(s.companyTree[parentID], id)
	}

	return rows.Err()
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

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "LISTEN config_changed"); err != nil {
		return
	}

	notificationChan := make(chan *sql.NullString, 10)
	go func() {
		for {
			var msg sql.NullString
			if err := conn.QueryRowContext(ctx, "SELECT pg_notify('config_changed', '')").Scan(&msg); err != nil {
				return
			}
			notificationChan <- &msg
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-notificationChan:
			if msg == nil {
				continue
			}
			s.loadAll(ctx)
		}
	}
}

func (s *PostgresConfigStore) Resolve(ctx context.Context, userID, domainID, companyID, key string) (json.RawMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resolveOrder := []struct {
		scopeType ScopeType
		scopeID   string
	}{
		{ScopeUser, userID},
		{ScopeDomain, domainID},
		{ScopeCompany, companyID},
	}

	for _, scope := range resolveOrder {
		if scope.scopeID == "" {
			continue
		}
		scopeKey := string(scope.scopeType) + ":" + scope.scopeID
		if entries, ok := s.cache[scopeKey]; ok {
			if entry, ok := entries[key]; ok {
				return entry.Value, nil
			}
		}
	}

	return nil, ErrConfigNotFound
}

func (s *PostgresConfigStore) Get(ctx context.Context, scopeType ScopeType, scopeID, key string) (*ConfigEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scopeKey := string(scopeType) + ":" + scopeID
	if entries, ok := s.cache[scopeKey]; ok {
		if entry, ok := entries[key]; ok {
			return entry, nil
		}
	}
	return nil, ErrConfigNotFound
}

func (s *PostgresConfigStore) Set(ctx context.Context, scopeType ScopeType, scopeID, key string, value json.RawMessage, locked bool, expectedVersion int64) (*ConfigEntry, error) {
	const query = `
		INSERT INTO runtime_config (scope_type, scope_id, key, value, locked, version)
		VALUES ($1, $2::uuid, $3, $4, $5, 1)
		ON CONFLICT (scope_type, scope_id, key) DO UPDATE SET
			value = EXCLUDED.value,
			locked = EXCLUDED.locked,
			version = runtime_config.version + 1,
			updated_at = now()
		WHERE runtime_config.version = $6 OR $6 = -1
		RETURNING id::text, scope_type, scope_id::text, key, value, locked, version, updated_at::text`

	var entry ConfigEntry
	err := s.db.QueryRowContext(ctx, query, string(scopeType), scopeID, key, value, locked, expectedVersion).Scan(
		&entry.ID, &entry.ScopeType, &entry.ScopeID, &entry.Key, &entry.Value, &entry.Locked, &entry.Version, &entry.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) && expectedVersion != -1 {
			return nil, ErrVersionConflict
		}
		return nil, fmt.Errorf("set config: %w", err)
	}

	s.mu.Lock()
	scopeKey := string(scopeType) + ":" + scopeID
	if s.cache[scopeKey] == nil {
		s.cache[scopeKey] = make(map[string]*ConfigEntry)
	}
	s.cache[scopeKey][key] = &entry
	s.mu.Unlock()

	s.Notify(ctx, ConfigChangeEvent{
		ScopeType: scopeType,
		ScopeID:   scopeID,
		Key:       key,
		Action:    "updated",
	})

	return &entry, nil
}

func (s *PostgresConfigStore) Delete(ctx context.Context, scopeType ScopeType, scopeID, key string, expectedVersion int64) error {
	const checkQuery = `
		SELECT locked FROM runtime_config
		WHERE scope_type = $1 AND scope_id = $2::uuid AND key = $3`

	var locked bool
	if err := s.db.QueryRowContext(ctx, checkQuery, string(scopeType), scopeID, key).Scan(&locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConfigNotFound
		}
		return fmt.Errorf("check config: %w", err)
	}

	if locked {
		return ErrConfigLocked
	}

	const query = `
		DELETE FROM runtime_config
		WHERE scope_type = $1 AND scope_id = $2::uuid AND key = $3 AND version = $4
		RETURNING id`

	var id string
	err := s.db.QueryRowContext(ctx, query, string(scopeType), scopeID, key, expectedVersion).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if expectedVersion == -1 {
				return fmt.Errorf("delete config: not found")
			}
			return ErrVersionConflict
		}
		return fmt.Errorf("delete config: %w", err)
	}

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

func (s *PostgresConfigStore) Propagate(ctx context.Context, companyID string, scope PropagateScope, key string, value json.RawMessage, locked bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetScopeIDs := s.collectPropagateTargets(companyID, scope)

	for _, targetID := range targetScopeIDs {
		if _, err := s.Set(ctx, ScopeCompany, targetID, key, value, locked, -1); err != nil {
			return fmt.Errorf("propagate to company %s: %w", targetID, err)
		}
	}

	return nil
}

func (s *PostgresConfigStore) collectPropagateTargets(companyID string, scope PropagateScope) []string {
	var targets []string

	switch scope {
	case PropagateSubtree:
		targets = s.collectSubtree(companyID)
	case PropagateChildren:
		targets = s.companyTree[companyID]
	case PropagateDomains:
		targets = []string{companyID}
	}

	return targets
}

func (s *PostgresConfigStore) collectSubtree(companyID string) []string {
	var result []string
	queue := []string{companyID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		if children, ok := s.companyTree[current]; ok {
			queue = append(queue, children...)
		}
	}

	return result
}

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
			return fmt.Errorf("propagate from parent %s/%s: %w", parentScopeType, parentScopeID, err)
		}
	}

	return nil
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
