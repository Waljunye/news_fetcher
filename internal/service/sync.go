package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"news_fetcher/internal/config"
	"news_fetcher/internal/domain"
)

type SyncService struct {
	source    Source
	articles  ArticleStore
	tags      TagStore
	syncState SyncStateStore
	txManager TransactionManager
	publisher Publisher
	logger    *slog.Logger
	config    config.SyncConfig
}

func NewSyncService(
	source Source,
	articles ArticleStore,
	tags TagStore,
	syncState SyncStateStore,
	txManager TransactionManager,
	publisher Publisher,
	logger *slog.Logger,
	cfg config.SyncConfig,
) *SyncService {
	return &SyncService{
		source:    source,
		articles:  articles,
		tags:      tags,
		syncState: syncState,
		txManager: txManager,
		publisher: publisher,
		logger:    logger.With("source", source.ID()),
		config:    cfg,
	}
}

func (s *SyncService) Sync(ctx context.Context) (*domain.SyncStats, error) {
	startTime := time.Now()
	s.logger.Info("starting sync",
		"source_name", s.source.Name(),
		"max_pages", s.config.MaxPagesPerSync,
		"max_historical_days", s.config.MaxHistoricalDays,
	)

	// Fetch articles from source (already transformed to domain)
	articles, err := s.source.FetchArticles(ctx, s.config.MaxPagesPerSync)
	if err != nil {
		return nil, fmt.Errorf("fetch articles: %w", err)
	}

	s.logger.Info("fetched articles from source", "count", len(articles))

	// Filter by date
	cutoffDate := time.Now().AddDate(0, 0, -s.config.MaxHistoricalDays)
	articles = s.filterByDate(articles, cutoffDate)
	s.logger.Debug("filtered by date", "remaining", len(articles))

	// Filter for sync (new or updated)
	toSync, err := s.filterForSync(ctx, articles)
	if err != nil {
		return nil, fmt.Errorf("filter for sync: %w", err)
	}

	s.logger.Info("articles to sync", "count", len(toSync))

	stats := &domain.SyncStats{
		SourceID: s.source.ID(),
		Fetched:  len(articles),
		Skipped:  len(articles) - len(toSync),
	}

	for i := range toSync {
		article := &toSync[i]
		isNew, err := s.saveArticle(ctx, article)
		if err != nil {
			stats.Errors++
			continue
		}

		if s.publisher != nil {
			if err := s.publisher.Publish(ctx, article, isNew); err != nil {
				stats.Errors++
			} else {
				stats.Published++
			}
		}

		if isNew {
			stats.New++
		} else {
			stats.Updated++
		}
	}

	if err := s.updateSyncState(ctx, stats); err != nil {
		return stats, fmt.Errorf("update sync state: %w", err)
	}

	stats.Duration = time.Since(startTime)

	s.logger.Info("sync completed",
		"new", stats.New,
		"updated", stats.Updated,
		"skipped", stats.Skipped,
		"errors", stats.Errors,
		"published", stats.Published,
		"duration", stats.Duration,
	)

	return stats, nil
}

func (s *SyncService) filterByDate(articles []domain.Article, cutoff time.Time) []domain.Article {
	var filtered []domain.Article
	for _, a := range articles {
		if a.PublishedAt.After(cutoff) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func (s *SyncService) filterForSync(ctx context.Context, articles []domain.Article) ([]domain.Article, error) {
	if len(articles) == 0 {
		return nil, nil
	}

	externalIDs := make([]int64, len(articles))
	for i, a := range articles {
		externalIDs[i] = a.ExternalID
	}

	existing, err := s.articles.GetExistingBySourceAndExternalIDs(ctx, s.source.ID(), externalIDs)
	if err != nil {
		return nil, err
	}

	var toSync []domain.Article
	for _, article := range articles {
		existingLastMod, exists := existing[article.ExternalID]

		if !exists {
			toSync = append(toSync, article)
		} else if article.LastModified.After(existingLastMod) {
			toSync = append(toSync, article)
		}
	}

	return toSync, nil
}

func (s *SyncService) saveArticle(ctx context.Context, article *domain.Article) (bool, error) {
	existing, err := s.articles.GetExistingBySourceAndExternalIDs(ctx, s.source.ID(), []int64{article.ExternalID})
	if err != nil {
		return false, err
	}
	isNew := len(existing) == 0

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		articleID, err := s.articles.Upsert(txCtx, article)
		if err != nil {
			return fmt.Errorf("upsert article: %w", err)
		}

		if len(article.Tags) > 0 {
			if err := s.tags.UpsertBatch(txCtx, article.Tags); err != nil {
				return fmt.Errorf("upsert tags: %w", err)
			}

			tagIDs := make([]int64, len(article.Tags))
			for i, tag := range article.Tags {
				tagIDs[i] = tag.ID
			}

			if err := s.tags.LinkToArticle(txCtx, articleID, tagIDs); err != nil {
				return fmt.Errorf("link tags: %w", err)
			}
		}

		return nil
	})

	return isNew, err
}

func (s *SyncService) updateSyncState(ctx context.Context, stats *domain.SyncStats) error {
	state, err := s.syncState.Get(ctx, s.source.ID())
	if err != nil {
		return err
	}

	state.SourceID = s.source.ID()
	state.LastSyncedAt = time.Now()
	state.TotalSynced += int64(stats.New + stats.Updated)

	return s.syncState.Update(ctx, state)
}
