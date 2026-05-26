package jmap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// StateChange represents a JMAP push event (RFC 8620 §7.1).
type StateChange struct {
	Changed map[string]map[string]string `json:"changed"`
}

// StateNotifier allows the application to push state change events to
// SSE subscribers. When nil, the endpoint sends only pings.
type StateNotifier interface {
	Subscribe(userID string) <-chan StateChange
	Unsubscribe(userID string, ch <-chan StateChange)
}

// ServeEventSource handles GET /jmap/eventsource/ — RFC 8620 §7.3.
func (h *Handler) ServeEventSource(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userIDFromBearer(r)
	if !ok {
		writeJSONResponse(w, http.StatusUnauthorized, map[string]string{"type": "unauthorized"})
		return
	}

	// Parse query parameters.
	q := r.URL.Query()
	typesParam := q.Get("types")      // "*" or "Mailbox,Email,..."
	closeAfter := q.Get("closeafter") // "state" or "no"
	pingParam := q.Get("ping")

	pingInterval := 0
	if pingParam != "" {
		if n, err := strconv.Atoi(pingParam); err == nil && n > 0 {
			pingInterval = n
		}
	}
	if closeAfter == "" {
		closeAfter = "no"
	}

	// typesParam is stored for future filtering; currently we push all types.
	_ = typesParam

	// SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	flush()

	// Send initial state event (empty changed map per RFC 8620 §7.3).
	h.sseWriteEvent(w, "state", StateChange{Changed: map[string]map[string]string{}})
	flush()

	// Subscribe to state changes if a notifier is available.
	var changeCh <-chan StateChange
	if h.deps.Notifier != nil {
		changeCh = h.deps.Notifier.Subscribe(userID)
		defer h.deps.Notifier.Unsubscribe(userID, changeCh)
	}

	// Set up ping ticker.
	var ticker *time.Ticker
	var tickC <-chan time.Time
	if pingInterval > 0 {
		ticker = time.NewTicker(time.Duration(pingInterval) * time.Second)
		defer ticker.Stop()
		tickC = ticker.C
	}

	for {
		select {
		case <-r.Context().Done():
			return

		case <-tickC:
			h.ssePing(w, pingInterval)
			flush()

		case change, ok := <-changeCh:
			if !ok {
				return
			}
			h.sseWriteEvent(w, "state", change)
			flush()
			if closeAfter == "state" {
				return
			}
		}
	}
}

// sseWriteEvent writes a named SSE event with a JSON data payload.
func (h *Handler) sseWriteEvent(w http.ResponseWriter, event string, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

// ssePing writes a JMAP SSE ping event.
func (h *Handler) ssePing(w http.ResponseWriter, intervalSecs int) {
	fmt.Fprintf(w, "event: ping\ndata: {\"interval\": %d}\n\n", intervalSecs)
}

