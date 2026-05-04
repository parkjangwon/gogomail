package app

import (
	"context"
	"fmt"

	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/maildb"
)

type backpressureStore interface {
	State(ctx context.Context) (backpressure.State, error)
	SetState(ctx context.Context, update backpressure.StateUpdate) (backpressure.State, error)
}

type adminService struct {
	*maildb.Repository
	backpressure backpressureStore
}

func (s adminService) GetBackpressure(ctx context.Context) (backpressure.State, error) {
	if s.backpressure == nil {
		return backpressure.State{}, fmt.Errorf("backpressure backend is not configured")
	}
	return s.backpressure.State(ctx)
}

func (s adminService) UpdateBackpressure(ctx context.Context, req backpressure.StateUpdate) (backpressure.State, error) {
	if s.backpressure == nil {
		return backpressure.State{}, fmt.Errorf("backpressure backend is not configured")
	}
	return s.backpressure.SetState(ctx, req)
}
