package maildb

import (
	"encoding/json"
	"testing"

	"github.com/gogomail/gogomail/internal/message"
)

func TestAddressesJSONEncodesNameAndAddress(t *testing.T) {
	t.Parallel()

	raw, err := addressesJSON([]message.Address{
		{Name: "Admin", Address: "admin@example.com"},
		{Name: "User", Address: "user@example.com"},
	})
	if err != nil {
		t.Fatalf("addressesJSON returned error: %v", err)
	}

	var got []map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("decoded addresses = %d, want 2", len(got))
	}
	if got[0]["name"] != "Admin" || got[0]["address"] != "admin@example.com" {
		t.Fatalf("first address = %+v", got[0])
	}
}
