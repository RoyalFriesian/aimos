package postgres

import (
	"context"
	"fmt"

	"github.com/Sarnga/agent-platform/pkg/execution"
	"github.com/Sarnga/agent-platform/pkg/feedback"
	"github.com/Sarnga/agent-platform/pkg/missions"
	"github.com/Sarnga/agent-platform/pkg/missionstate"
	"github.com/Sarnga/agent-platform/pkg/threads"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Stores struct {
	Pool         *pgxpool.Pool
	Missions     missions.Store
	Threads      threads.Store
	MissionState missionstate.Store
	Execution    execution.Store
	Feedback     feedback.Store
}

func OpenStores(ctx context.Context, config Config) (*Stores, error) {
	pool, err := OpenPool(ctx, config)
	if err != nil {
		return nil, err
	}
	threadStore, err := threads.NewPostgresStore(pool)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create thread store: %w", err)
	}
	missionStore, err := missions.NewPostgresStore(pool)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create mission store: %w", err)
	}
	missionStateStore, err := missionstate.NewPostgresStore(pool)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create mission state store: %w", err)
	}
	executionStore, err := execution.NewPostgresStore(pool)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create execution store: %w", err)
	}
	feedbackStore, err := feedback.NewPostgresStore(pool)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create feedback store: %w", err)
	}
	return &Stores{
		Pool:         pool,
		Missions:     missionStore,
		Threads:      threadStore,
		MissionState: missionStateStore,
		Execution:    executionStore,
		Feedback:     feedbackStore,
	}, nil
}

func (s *Stores) Close() {
	if s == nil || s.Pool == nil {
		return
	}
	s.Pool.Close()
}
