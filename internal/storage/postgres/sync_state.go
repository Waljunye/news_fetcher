package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	"news_fetcher/internal/domain"
)

type SyncStateStore struct {
	db *sqlx.DB
}

func NewSyncStateStore(db *sqlx.DB) *SyncStateStore {
	return &SyncStateStore{db: db}
}

func (s *SyncStateStore) Get(ctx context.Context, sourceID string) (*domain.SyncState, error) {
	var state domain.SyncState
	query := `
		SELECT id, source_id, last_synced_at, last_article_id, total_synced
		FROM sync_state
		WHERE source_id = $1`

	err := s.db.GetContext(ctx, &state, query, sourceID)
	if err == sql.ErrNoRows {
		// Return empty state for new sources
		return &domain.SyncState{
			SourceID:     sourceID,
			LastSyncedAt: time.Time{},
			TotalSynced:  0,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *SyncStateStore) Update(ctx context.Context, state *domain.SyncState) error {
	query := `
		INSERT INTO sync_state (source_id, last_synced_at, last_article_id, total_synced)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source_id) DO UPDATE SET
			last_synced_at = EXCLUDED.last_synced_at,
			last_article_id = EXCLUDED.last_article_id,
			total_synced = EXCLUDED.total_synced`

	_, err := s.db.ExecContext(ctx, query,
		state.SourceID,
		state.LastSyncedAt,
		state.LastArticleID,
		state.TotalSynced,
	)
	return err
}