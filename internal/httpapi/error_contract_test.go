package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorIncludesStableStatusText(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	writeError(rec, http.StatusUnauthorized, "bearer token is required")

	var body struct {
		Error struct {
			Code       string `json:"code"`
			Message    string `json:"message"`
			Status     int    `json:"status"`
			StatusText string `json:"status_text"`
		} `json:"error"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Error.Code != "unauthorized" || body.Error.Status != http.StatusUnauthorized {
		t.Fatalf("error = %+v", body.Error)
	}
	if body.Error.StatusText != "Unauthorized" {
		t.Fatalf("status_text = %q", body.Error.StatusText)
	}
	if body.ErrorMessage != body.Error.Message {
		t.Fatalf("error_message = %q, want %q", body.ErrorMessage, body.Error.Message)
	}
}

func TestConstantTimeTokenEqualUsesTrimmedTokenValues(t *testing.T) {
	t.Parallel()

	if !constantTimeTokenEqual(" secret-token ", "secret-token") {
		t.Fatal("constantTimeTokenEqual rejected matching trimmed tokens")
	}
	for _, got := range []string{"", "secret-token-extra", "secret"} {
		if constantTimeTokenEqual(got, "secret-token") {
			t.Fatalf("constantTimeTokenEqual(%q) = true, want false", got)
		}
	}
}
