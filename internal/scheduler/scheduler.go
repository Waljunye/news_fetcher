package scheduler

import (
	"context"
	"log/slog"
	"time"

	"news_fetcher/internal/domain"
)

// Syncer defines the interface for sync operations.
type Syncer interface {
	Sync(ctx context.Context) (*domain.SyncStats, error)
}

type Scheduler struct {
	syncer   Syncer
	interval time.Duration
	logger   *slog.Logger
}

func NewScheduler(syncer Syncer, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		syncer:   syncer,
		interval: interval,
		logger:   logger,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info("scheduler started", "interval", s.interval)

	s.runSync(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return ctx.Err()
		case <-ticker.C:
			s.runSync(ctx)
		}
	}
}

func (s *Scheduler) runSync(ctx context.Context) {
	syncCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, err := s.syncer.Sync(syncCtx); err != nil {
		s.logger.Error("sync failed", "error", err)
	}
}