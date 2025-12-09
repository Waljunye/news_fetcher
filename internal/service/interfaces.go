package service

//go:generate mockgen -source=interfaces.go -destination=mocks/mocks.go -package=mocks

import (
	"context"
	"time"

	"news_fetcher/internal/domain"
)

type ArticleStore interface {
	Upsert(ctx context.Context, article *domain.Article) (int64, error)
	GetExistingBySourceAndExternalIDs(ctx context.Context, sourceID string, ids []int64) (map[int64]time.Time, error)
}

type TagStore interface {
	UpsertBatch(ctx context.Context, tags []domain.Tag) error
	LinkToArticle(ctx context.Context, articleID int64, tagIDs []int64) error
}

type SyncStateStore interface {
	Get(ctx context.Context, sourceID string) (*domain.SyncState, error)
	Update(ctx context.Context, state *domain.SyncState) error
}

type Source interface {
	ID() string
	Name() string
	FetchArticles(ctx context.Context, maxPages int) ([]domain.Article, error)
}

type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type Publisher interface {
	Publish(ctx context.Context, article *domain.Article, isNew bool) error
	Close() error
}
