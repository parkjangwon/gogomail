package configstore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestResolveTreeOrder(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{
		cache: map[string]map[string]*ConfigEntry{
			"company:root": {
				"auth.mfa.mode": {Key: "auth.mfa.mode", Value: json.RawMessage(`"optional"`), ScopeType: ScopeCompany, ScopeID: "root"},
			},
			"domain:example": {
				"auth.mfa.mode": {Key: "auth.mfa.mode", Value: json.RawMessage(`"required"`), ScopeType: ScopeDomain, ScopeID: "example"},
			},
			"user:alice": {},
		},
	}

	ctx := context.Background()

	val, err := store.Resolve(ctx, "alice", "example", "root", "auth.mfa.mode")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if string(val) != `"required"` {
		t.Fatalf("Resolve = %s, want %s", string(val), `"required"`)
	}

	val, err = store.Resolve(ctx, "", "", "root", "auth.mfa.mode")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if string(val) != `"optional"` {
		t.Fatalf("Resolve = %s, want %s", string(val), `"optional"`)
	}

	_, err = store.Resolve(ctx, "", "", "", "auth.mfa.mode")
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("Resolve error = %v, want ErrConfigNotFound", err)
	}
}

func TestScopeTypeIsValid(t *testing.T) {
	t.Parallel()

	if !ScopeCompany.IsValid() {
		t.Error("ScopeCompany should be valid")
	}
	if !ScopeDomain.IsValid() {
		t.Error("ScopeDomain should be valid")
	}
	if !ScopeUser.IsValid() {
		t.Error("ScopeUser should be valid")
	}

	var invalid PropagateScope = "invalid"
	if invalid.IsValid() {
		t.Error("invalid should not be valid")
	}

	if !PropagateSubtree.IsValid() {
		t.Error("PropagateSubtree should be valid")
	}
	if !PropagateChildren.IsValid() {
		t.Error("PropagateChildren should be valid")
	}
	if !PropagateDomains.IsValid() {
		t.Error("PropagateDomains should be valid")
	}
}

func TestCollectSubtree(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{
		companyTree: map[string][]string{
			"root": {"child1", "child2"},
			"child1": {"grandchild1"},
			"child2": {},
		},
	}

	result := store.collectSubtree("root")
	if len(result) != 4 {
		t.Fatalf("collectSubtree = %v, want 4 items", result)
	}

	found := make(map[string]bool)
	for _, id := range result {
		found[id] = true
	}
	if !found["root"] || !found["child1"] || !found["child2"] || !found["grandchild1"] {
		t.Errorf("collectSubtree missing items: %v", result)
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{}

	ch1 := store.Subscribe()
	ch2 := store.Subscribe()

	if len(store.subscribers) != 2 {
		t.Errorf("subscribers = %d, want 2", len(store.subscribers))
	}

	store.Unsubscribe(ch1)
	if len(store.subscribers) != 1 {
		t.Errorf("after unsubscribe subscribers = %d, want 1", len(store.subscribers))
	}

	store.Unsubscribe(ch2)
	if len(store.subscribers) != 0 {
		t.Errorf("after unsubscribe subscribers = %d, want 0", len(store.subscribers))
	}
}

func TestConfigStoreInterface(t *testing.T) {
	t.Parallel()

	var _ ConfigStore = (*PostgresConfigStore)(nil)
}

func TestPropagateScopeIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scope PropagateScope
		valid bool
	}{
		{PropagateSubtree, true},
		{PropagateChildren, true},
		{PropagateDomains, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.scope.IsValid(); got != tt.valid {
			t.Errorf("PropagateScope(%q).IsValid() = %v, want %v", tt.scope, got, tt.valid)
		}
	}
}

