package configstore

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

// --- Resolve tests ---

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
		parentOf: map[string]string{},
	}

	ctx := context.Background()

	// Domain overrides company when neither is locked.
	val, err := store.Resolve(ctx, "alice", "example", "root", "auth.mfa.mode")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if string(val) != `"required"` {
		t.Fatalf("Resolve = %s, want %q", val, "required")
	}

	// Company wins when domain has no entry.
	val, err = store.Resolve(ctx, "", "", "root", "auth.mfa.mode")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if string(val) != `"optional"` {
		t.Fatalf("Resolve = %s, want %q", val, "optional")
	}

	// Nothing in hierarchy → ErrConfigNotFound.
	_, err = store.Resolve(ctx, "", "", "", "auth.mfa.mode")
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("Resolve error = %v, want ErrConfigNotFound", err)
	}
}

func TestResolveLocked_CompanyBlocksDomain(t *testing.T) {
	t.Parallel()

	// Company locks the value; domain and user must NOT override it.
	store := &PostgresConfigStore{
		cache: map[string]map[string]*ConfigEntry{
			"company:c1": {
				"auth.mfa.mode": {Key: "auth.mfa.mode", Value: json.RawMessage(`"required"`), Locked: true},
			},
			"domain:d1": {
				"auth.mfa.mode": {Key: "auth.mfa.mode", Value: json.RawMessage(`"disabled"`), Locked: false},
			},
			"user:u1": {
				"auth.mfa.mode": {Key: "auth.mfa.mode", Value: json.RawMessage(`"optional"`), Locked: false},
			},
		},
		parentOf: map[string]string{},
	}

	val, err := store.Resolve(context.Background(), "u1", "d1", "c1", "auth.mfa.mode")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if string(val) != `"required"` {
		t.Fatalf("Resolve = %s, want %q (company locked should win)", val, "required")
	}
}

func TestResolveLocked_DomainBlocksUser(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{
		cache: map[string]map[string]*ConfigEntry{
			"company:c1": {
				"auth.mfa.mode": {Key: "auth.mfa.mode", Value: json.RawMessage(`"optional"`), Locked: false},
			},
			"domain:d1": {
				"auth.mfa.mode": {Key: "auth.mfa.mode", Value: json.RawMessage(`"required"`), Locked: true},
			},
			"user:u1": {
				"auth.mfa.mode": {Key: "auth.mfa.mode", Value: json.RawMessage(`"disabled"`), Locked: false},
			},
		},
		parentOf: map[string]string{},
	}

	val, err := store.Resolve(context.Background(), "u1", "d1", "c1", "auth.mfa.mode")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if string(val) != `"required"` {
		t.Fatalf("Resolve = %s, want %q (domain locked should block user)", val, "required")
	}
}

func TestResolveLocked_RootCompanyBlocksAll(t *testing.T) {
	t.Parallel()

	// Root is locked. Sub-company, domain, user should all be blocked.
	store := &PostgresConfigStore{
		cache: map[string]map[string]*ConfigEntry{
			"company:root": {
				"policy": {Key: "policy", Value: json.RawMessage(`"strict"`), Locked: true},
			},
			"company:sub": {
				"policy": {Key: "policy", Value: json.RawMessage(`"loose"`), Locked: false},
			},
			"domain:d1": {
				"policy": {Key: "policy", Value: json.RawMessage(`"none"`), Locked: false},
			},
		},
		parentOf:    map[string]string{"sub": "root"},
		companyTree: map[string][]string{"root": {"sub"}},
	}

	val, err := store.Resolve(context.Background(), "", "d1", "sub", "policy")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if string(val) != `"strict"` {
		t.Fatalf("Resolve = %s, want %q (root locked should win)", val, "strict")
	}
}

func TestResolveCompanyTree(t *testing.T) {
	t.Parallel()

	// root → sub → grandchild. Only root has a value; it should be found.
	store := &PostgresConfigStore{
		cache: map[string]map[string]*ConfigEntry{
			"company:root": {
				"k": {Key: "k", Value: json.RawMessage(`"from-root"`), Locked: false},
			},
		},
		parentOf:    map[string]string{"sub": "root", "gc": "sub"},
		companyTree: map[string][]string{"root": {"sub"}, "sub": {"gc"}},
	}

	val, err := store.Resolve(context.Background(), "", "", "gc", "k")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if string(val) != `"from-root"` {
		t.Fatalf("Resolve = %s, want %q", val, "from-root")
	}
}

func TestResolveUserOverridesDomain(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{
		cache: map[string]map[string]*ConfigEntry{
			"domain:d1": {
				"theme": {Key: "theme", Value: json.RawMessage(`"light"`), Locked: false},
			},
			"user:u1": {
				"theme": {Key: "theme", Value: json.RawMessage(`"dark"`), Locked: false},
			},
		},
		parentOf: map[string]string{},
	}

	val, err := store.Resolve(context.Background(), "u1", "d1", "", "theme")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if string(val) != `"dark"` {
		t.Fatalf("Resolve = %s, want %q (user should override domain)", val, "dark")
	}
}

// --- buildCompanyChain tests ---

func TestBuildCompanyChain_Single(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{parentOf: map[string]string{}}
	chain := store.buildCompanyChain("c1")
	if len(chain) != 1 || chain[0] != "c1" {
		t.Fatalf("chain = %v, want [c1]", chain)
	}
}

func TestBuildCompanyChain_Ancestry(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{
		parentOf: map[string]string{"child": "parent", "parent": "root"},
	}
	chain := store.buildCompanyChain("child")
	if len(chain) != 3 {
		t.Fatalf("chain len = %d, want 3: %v", len(chain), chain)
	}
	if chain[0] != "child" || chain[1] != "parent" || chain[2] != "root" {
		t.Fatalf("chain = %v, want [child parent root]", chain)
	}
}

func TestBuildCompanyChain_NoCycle(t *testing.T) {
	t.Parallel()

	// Artificial cycle — should not loop forever.
	store := &PostgresConfigStore{
		parentOf: map[string]string{"a": "b", "b": "a"},
	}
	chain := store.buildCompanyChain("a")
	if len(chain) > 10 {
		t.Fatalf("cycle not detected, chain len = %d", len(chain))
	}
}

// --- collectSubtree tests ---

func TestCollectSubtree(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{
		companyTree: map[string][]string{
			"root":   {"child1", "child2"},
			"child1": {"grandchild1"},
			"child2": {},
		},
		parentOf: map[string]string{},
	}

	result := store.collectSubtree("root")
	if len(result) != 4 {
		t.Fatalf("collectSubtree = %v, want 4 items", result)
	}
	found := make(map[string]bool)
	for _, id := range result {
		found[id] = true
	}
	for _, want := range []string{"root", "child1", "child2", "grandchild1"} {
		if !found[want] {
			t.Errorf("collectSubtree missing %q: %v", want, result)
		}
	}
}

// --- Subscribe / Unsubscribe tests ---

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
		t.Errorf("after unsubscribe(ch1) subscribers = %d, want 1", len(store.subscribers))
	}

	store.Unsubscribe(ch2)
	if len(store.subscribers) != 0 {
		t.Errorf("after unsubscribe(ch2) subscribers = %d, want 0", len(store.subscribers))
	}
}

func TestNotifyFansOut(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{}
	ch1 := store.Subscribe()
	ch2 := store.Subscribe()

	event := ConfigChangeEvent{ScopeType: ScopeDomain, ScopeID: "d1", Key: "k", Action: "updated"}
	store.Notify(context.Background(), event)

	for _, ch := range []chan ConfigChangeEvent{ch1, ch2} {
		select {
		case got := <-ch:
			if got != event {
				t.Errorf("got %+v, want %+v", got, event)
			}
		default:
			t.Error("expected event on channel, got none")
		}
	}
}

func TestNotifyConcurrentSubscribers(t *testing.T) {
	t.Parallel()

	store := &PostgresConfigStore{}
	const n = 10
	channels := make([]chan ConfigChangeEvent, n)
	for i := range channels {
		channels[i] = store.Subscribe()
	}

	event := ConfigChangeEvent{ScopeType: ScopeCompany, ScopeID: "c1", Key: "x", Action: "created"}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.Notify(context.Background(), event)
		}(i)
	}
	wg.Wait()

	for i, ch := range channels {
		if len(ch) == 0 {
			t.Errorf("channel %d received nothing", i)
		}
		for len(ch) > 0 {
			<-ch
		}
	}

	for _, ch := range channels {
		store.Unsubscribe(ch)
	}
}

// --- PropagateScope / ScopeType validity ---

func TestScopeTypeIsValid(t *testing.T) {
	t.Parallel()

	for _, s := range []ScopeType{ScopeCompany, ScopeDomain, ScopeUser} {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if ScopeType("global").IsValid() {
		t.Error(`"global" should not be valid`)
	}
}

func TestPropagateScopeIsValid(t *testing.T) {
	t.Parallel()

	for _, s := range []PropagateScope{PropagateSubtree, PropagateChildren, PropagateDomains} {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []PropagateScope{"invalid", ""} {
		if s.IsValid() {
			t.Errorf("%q should not be valid", s)
		}
	}
}

// --- Interface compliance ---

func TestConfigStoreInterface(t *testing.T) {
	t.Parallel()
	var _ ConfigStore = (*PostgresConfigStore)(nil)
	var _ Notifier = (*PostgresConfigStore)(nil)
}
