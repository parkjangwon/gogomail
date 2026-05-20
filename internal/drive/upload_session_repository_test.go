package drive

import (
	"context"
	"strings"
	"testing"
)

func TestCreateUploadSessionRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.CreateUploadSession(context.Background(), CreateUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("CreateUploadSession err = %v, want database handle rejection first", err)
	}
}

func TestServiceCreateUploadSessionRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).CreateUploadSession(context.Background(), CreateUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("CreateUploadSession err = %v, want repository rejection", err)
	}
}

func TestGetUploadSessionRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.GetUploadSession(context.Background(), GetUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("GetUploadSession err = %v, want database handle rejection first", err)
	}
}

func TestServiceGetUploadSessionRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).GetUploadSession(context.Background(), GetUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("GetUploadSession err = %v, want repository rejection", err)
	}
}

func TestListUploadSessionsRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.ListUploadSessions(context.Background(), ListUploadSessionsRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("ListUploadSessions err = %v, want database handle rejection first", err)
	}
}

func TestServiceListUploadSessionsRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).ListUploadSessions(context.Background(), ListUploadSessionsRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("ListUploadSessions err = %v, want repository rejection", err)
	}
}

func TestListUploadSessionsQueryUsesSargableStatusFilter(t *testing.T) {
	t.Parallel()

	req, err := ValidateListUploadSessionsRequest(ListUploadSessionsRequest{
		UserID: " user-1 ",
		Status: " Uploading ",
		Limit:  25,
	})
	if err != nil {
		t.Fatalf("ValidateListUploadSessionsRequest returned error: %v", err)
	}
	query, args := buildListUploadSessionsQuery(req)
	for _, want := range []string{
		"FROM drive_upload_sessions s",
		"WHERE s.user_id = $1::uuid",
		"AND u.status = 'active'",
		"AND d.status = 'active'",
		"AND s.status = $2",
		"ORDER BY s.updated_at DESC, s.created_at DESC",
		"LIMIT $3",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list upload sessions query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "$2 = '' OR") {
		t.Fatalf("list upload sessions query contains non-sargable status filter:\n%s", query)
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
	if args[0] != "user-1" || args[1] != UploadSessionStatusUploading || args[2] != 25 {
		t.Fatalf("args = %#v", args)
	}

	query, args = buildListUploadSessionsQuery(ListUploadSessionsRequest{
		UserID: "user-1",
		Limit:  50,
	})
	if strings.Contains(query, "s.status = $") {
		t.Fatalf("unfiltered list upload sessions query unexpectedly includes status predicate:\n%s", query)
	}
	if len(args) != 2 {
		t.Fatalf("unfiltered args len = %d, want 2", len(args))
	}
	if args[0] != "user-1" || args[1] != 50 {
		t.Fatalf("unfiltered args = %#v", args)
	}
}

func TestCancelUploadSessionRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.CancelUploadSession(context.Background(), CancelUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("CancelUploadSession err = %v, want database handle rejection first", err)
	}
}

func TestServiceCancelUploadSessionRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).CancelUploadSession(context.Background(), CancelUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("CancelUploadSession err = %v, want repository rejection", err)
	}
}

func TestExpireUploadSessionsRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.ExpireUploadSessions(context.Background(), ExpireUploadSessionsRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("ExpireUploadSessions err = %v, want database handle rejection first", err)
	}
}

func TestCountStaleUploadSessionsRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.CountStaleUploadSessions(context.Background(), ExpireUploadSessionsRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("CountStaleUploadSessions err = %v, want database handle rejection first", err)
	}
}

func TestListStaleUploadSessionsRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.ListStaleUploadSessions(context.Background(), ExpireUploadSessionsRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("ListStaleUploadSessions err = %v, want database handle rejection first", err)
	}
}

func TestServiceExpireUploadSessionsRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).ExpireUploadSessions(context.Background(), ExpireUploadSessionsRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("ExpireUploadSessions err = %v, want repository rejection", err)
	}
}

func TestServiceStaleUploadSessionPreviewRequiresRepository(t *testing.T) {
	t.Parallel()

	if _, err := (*Service)(nil).CountStaleUploadSessions(context.Background(), ExpireUploadSessionsRequest{}); err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("CountStaleUploadSessions err = %v, want repository rejection", err)
	}
	if _, err := (*Service)(nil).ListStaleUploadSessions(context.Background(), ExpireUploadSessionsRequest{}); err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("ListStaleUploadSessions err = %v, want repository rejection", err)
	}
}

func TestStoreUploadSessionBodyRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, _, err := repo.StoreUploadSessionBody(context.Background(), RecordUploadSessionBodyRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("StoreUploadSessionBody err = %v, want database handle rejection first", err)
	}
}

func TestServiceStoreUploadSessionBodyRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).StoreUploadSessionBody(context.Background(), StoreUploadSessionBodyRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("StoreUploadSessionBody err = %v, want repository rejection", err)
	}
}

func TestFinalizeUploadSessionRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.FinalizeUploadSession(context.Background(), fakeStore{}, FinalizeUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("FinalizeUploadSession err = %v, want database handle rejection first", err)
	}
}

func TestServiceFinalizeUploadSessionRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).FinalizeUploadSession(context.Background(), FinalizeUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("FinalizeUploadSession err = %v, want repository rejection", err)
	}
}
