package httpapi

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/configstore"
)

// fakeConfigNotifier implements configstore.Notifier for testing.
type fakeConfigNotifier struct {
	mu  sync.Mutex
	chs []chan configstore.ConfigChangeEvent
}

func (n *fakeConfigNotifier) Subscribe() chan configstore.ConfigChangeEvent {
	ch := make(chan configstore.ConfigChangeEvent, 10)
	n.mu.Lock()
	n.chs = append(n.chs, ch)
	n.mu.Unlock()
	return ch
}

func (n *fakeConfigNotifier) Unsubscribe(ch chan configstore.ConfigChangeEvent) {
	n.mu.Lock()
	for i, c := range n.chs {
		if c == ch {
			n.chs = append(n.chs[:i], n.chs[i+1:]...)
			close(ch)
			break
		}
	}
	n.mu.Unlock()
}

func (n *fakeConfigNotifier) emit(event configstore.ConfigChangeEvent) {
	n.mu.Lock()
	for _, ch := range n.chs {
		ch <- event
	}
	n.mu.Unlock()
}

func TestAdminConfigStreamSendsConnectedEvent(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/admin/v1/config/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			if !strings.Contains(line, `"type":"connected"`) {
				t.Errorf("first data event should be connected, got: %s", line)
			}
			return
		}
	}
	t.Fatal("no data event received before context deadline")
}

func TestAdminConfigStreamForwardsNotifierEvents(t *testing.T) {
	t.Parallel()

	notifier := &fakeConfigNotifier{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "", WithConfigNotifier(notifier))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/admin/v1/config/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	// Drain until connected event.
	connected := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"connected"`) {
			connected = true
			break
		}
	}
	if !connected {
		t.Fatal("did not receive connected event")
	}

	// Emit a config change event from the notifier.
	notifier.emit(configstore.ConfigChangeEvent{
		ScopeType: configstore.ScopeCompany,
		ScopeID:   "company-1",
		Key:       "auth.mfa.mode",
		Action:    "set",
	})

	// Expect a config.changed data line.
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			if !strings.Contains(line, "config.changed") {
				t.Errorf("expected config.changed event, got: %s", line)
			}
			if !strings.Contains(line, "auth.mfa.mode") {
				t.Errorf("expected key in event, got: %s", line)
			}
			return
		}
	}
	t.Fatal("no config.changed event received after notifier emit")
}
