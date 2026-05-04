package maildb

import (
	"context"
	"strings"
	"testing"
)

func TestStreamAPIUsageLedgerRejectsNilDatabase(t *testing.T) {
	t.Parallel()

	err := (&Repository{}).StreamAPIUsageLedger(context.Background(), APIUsageLedgerListRequest{}, nil)
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestAPIUsageLedgerStreamLimit(t *testing.T) {
	t.Parallel()

	limit, unbounded := apiUsageLedgerStreamLimit(APIUsageLedgerNoLimit)
	if limit != 0 || !unbounded {
		t.Fatalf("no-limit = %d/%v", limit, unbounded)
	}

	limit, unbounded = apiUsageLedgerStreamLimit(0)
	if limit != MessageListDefaultLimit || unbounded {
		t.Fatalf("default limit = %d/%v", limit, unbounded)
	}

	limit, unbounded = apiUsageLedgerStreamLimit(MessageListMaxLimit + 1)
	if limit != MessageListMaxLimit || unbounded {
		t.Fatalf("max limit = %d/%v", limit, unbounded)
	}
}
