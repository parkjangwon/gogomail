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
