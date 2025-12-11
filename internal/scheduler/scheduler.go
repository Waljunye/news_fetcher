package scheduler

import (
	"context"
	"log/slog"
	"time"

	"news_fetcher/internal/config"
	"news_fetcher/internal/domain"
)

// Syncer defines the interface for sync operations.
type Syncer interface {
	Sync(ctx context.Context) (*domain.SyncStats, error)
}

type Scheduler struct {
	syncer Syncer
	cfg    config.SyncConfig
	logger *slog.Logger
}

func NewScheduler(syncer Syncer, cfg config.SyncConfig, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		syncer: syncer,
		cfg:    cfg,
		logger: logger,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info("scheduler started", "interval", s.cfg.Interval)

	s.runSync(ctx)

	ticker := time.NewTicker(s.cfg.Interval)
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
	syncCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	if _, err := s.syncer.Sync(syncCtx); err != nil {
		s.logger.Error("sync failed", "error", err)
	}
}
