package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"news_fetcher/internal/config"
	"news_fetcher/internal/domain"
	"news_fetcher/internal/service/mocks"
)

type SyncServiceTestSuite struct {
	suite.Suite
	ctrl *gomock.Controller

	source      *mocks.MockSource
	articles    *mocks.MockArticleStore
	tags        *mocks.MockTagStore
	syncState   *mocks.MockSyncStateStore
	txManager   *mocks.MockTransactionManager
	publisher   *mocks.MockPublisher

	service *SyncService
	cfg     config.SyncConfig
	logger  *slog.Logger
}

func (s *SyncServiceTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())

	s.source = mocks.NewMockSource(s.ctrl)
	s.articles = mocks.NewMockArticleStore(s.ctrl)
	s.tags = mocks.NewMockTagStore(s.ctrl)
	s.syncState = mocks.NewMockSyncStateStore(s.ctrl)
	s.txManager = mocks.NewMockTransactionManager(s.ctrl)
	s.publisher = mocks.NewMockPublisher(s.ctrl)

	s.cfg = config.SyncConfig{
		Interval:          5 * time.Minute,
		MaxPagesPerSync:   5,
		MaxHistoricalDays: 30,
	}

	s.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	s.source.EXPECT().ID().Return("test-source").AnyTimes()
	s.source.EXPECT().Name().Return("Test Source").AnyTimes()

	s.service = NewSyncService(
		s.source,
		s.articles,
		s.tags,
		s.syncState,
		s.txManager,
		s.publisher,
		s.logger,
		s.cfg,
	)
}

func (s *SyncServiceTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestSyncServiceTestSuite(t *testing.T) {
	suite.Run(t, new(SyncServiceTestSuite))
}

func (s *SyncServiceTestSuite) TestSync_NewArticles() {
	ctx := context.Background()
	now := time.Now()

	articles := []domain.Article{
		{
			SourceID:     "test-source",
			ExternalID:   1,
			Title:        "asd",
			PublishedAt:  now,
			LastModified: now,
			Tags:         []domain.Tag{{ID: 1, Label: "test tag1"}},
		},
	}

	s.source.EXPECT().FetchArticles(ctx, s.cfg.MaxPagesPerSync).Return(articles, nil)

	s.articles.EXPECT().GetExistingBySourceAndExternalIDs(ctx, "test-source", []int64{1}).Return(map[int64]time.Time{}, nil)

	s.articles.EXPECT().GetExistingBySourceAndExternalIDs(ctx, "test-source", []int64{1}).Return(map[int64]time.Time{}, nil)

	s.txManager.EXPECT().WithTransaction(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	s.articles.EXPECT().Upsert(ctx, &articles[0]).Return(int64(100), nil)

	s.tags.EXPECT().UpsertBatch(ctx, articles[0].Tags).Return(nil)
	s.tags.EXPECT().LinkToArticle(ctx, int64(100), []int64{1}).Return(nil)

	s.publisher.EXPECT().Publish(ctx, &articles[0], true).Return(nil)

	s.syncState.EXPECT().Get(ctx, "test-source").Return(&domain.SyncState{SourceID: "test-source"}, nil)
	s.syncState.EXPECT().Update(ctx, gomock.Any()).Return(nil)

	stats, err := s.service.Sync(ctx)

	s.NoError(err)
	s.Equal(1, stats.Fetched)
	s.Equal(1, stats.New)
	s.Equal(0, stats.Updated)
	s.Equal(0, stats.Skipped)
	s.Equal(1, stats.Published)
}

func (s *SyncServiceTestSuite) TestSync_UpdatedArticles() {
	ctx := context.Background()
	now := time.Now()
	oldTime := now.Add(-1 * time.Hour)

	articles := []domain.Article{
		{
			SourceID:     "test-source",
			ExternalID:   1,
			Title:        "updated asd",
			PublishedAt:  now,
			LastModified: now,
		},
	}

	s.source.EXPECT().FetchArticles(ctx, s.cfg.MaxPagesPerSync).Return(articles, nil)

	s.articles.EXPECT().GetExistingBySourceAndExternalIDs(ctx, "test-source", []int64{1}).Return(
		map[int64]time.Time{1: oldTime}, nil,
	)

	s.articles.EXPECT().GetExistingBySourceAndExternalIDs(ctx, "test-source", []int64{1}).Return(
		map[int64]time.Time{1: oldTime}, nil,
	)

	s.txManager.EXPECT().WithTransaction(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	s.articles.EXPECT().Upsert(ctx, &articles[0]).Return(int64(100), nil)

	s.publisher.EXPECT().Publish(ctx, &articles[0], false).Return(nil)

	s.syncState.EXPECT().Get(ctx, "test-source").Return(&domain.SyncState{SourceID: "test-source"}, nil)
	s.syncState.EXPECT().Update(ctx, gomock.Any()).Return(nil)

	stats, err := s.service.Sync(ctx)

	s.NoError(err)
	s.Equal(1, stats.Fetched)
	s.Equal(0, stats.New)
	s.Equal(1, stats.Updated)
	s.Equal(0, stats.Skipped)
}

func (s *SyncServiceTestSuite) TestSync_SkipsOldArticles() {
	ctx := context.Background()
	now := time.Now()

	articles := []domain.Article{
		{
			SourceID:     "test-source",
			ExternalID:   1,
			Title:        "old asd",
			PublishedAt:  now,
			LastModified: now.Add(-1 * time.Hour), // older than existing
		},
	}

	s.source.EXPECT().FetchArticles(ctx, s.cfg.MaxPagesPerSync).Return(articles, nil)

	s.articles.EXPECT().GetExistingBySourceAndExternalIDs(ctx, "test-source", []int64{1}).Return(
		map[int64]time.Time{1: now}, nil,
	)

	s.syncState.EXPECT().Get(ctx, "test-source").Return(&domain.SyncState{SourceID: "test-source"}, nil)
	s.syncState.EXPECT().Update(ctx, gomock.Any()).Return(nil)

	stats, err := s.service.Sync(ctx)

	s.NoError(err)
	s.Equal(1, stats.Fetched)
	s.Equal(0, stats.New)
	s.Equal(0, stats.Updated)
	s.Equal(1, stats.Skipped)
}

func (s *SyncServiceTestSuite) TestSync_FiltersOutdatedByDate() {
	ctx := context.Background()
	now := time.Now()
	oldDate := now.AddDate(0, 0, -31)

	articles := []domain.Article{
		{
			SourceID:     "test-source",
			ExternalID:   1,
			Title:        "very old asd",
			PublishedAt:  oldDate,
			LastModified: oldDate,
		},
	}

	s.source.EXPECT().FetchArticles(ctx, s.cfg.MaxPagesPerSync).Return(articles, nil)

	s.syncState.EXPECT().Get(ctx, "test-source").Return(&domain.SyncState{SourceID: "test-source"}, nil)
	s.syncState.EXPECT().Update(ctx, gomock.Any()).Return(nil)

	stats, err := s.service.Sync(ctx)

	s.NoError(err)
	s.Equal(0, stats.Fetched)
	s.Equal(0, stats.New)
}

func (s *SyncServiceTestSuite) TestSync_SourceError() {
	ctx := context.Background()

	s.source.EXPECT().FetchArticles(ctx, s.cfg.MaxPagesPerSync).Return(nil, errors.New("api error"))

	stats, err := s.service.Sync(ctx)

	s.Error(err)
	s.Nil(stats)
	s.Contains(err.Error(), "fetch articles")
}

func (s *SyncServiceTestSuite) TestSync_PublisherNil() {
	ctx := context.Background()
	now := time.Now()

	service := NewSyncService(
		s.source,
		s.articles,
		s.tags,
		s.syncState,
		s.txManager,
		nil,
		s.logger,
		s.cfg,
	)

	articles := []domain.Article{
		{
			SourceID:     "test-source",
			ExternalID:   1,
			Title:        "asd",
			PublishedAt:  now,
			LastModified: now,
		},
	}

	s.source.EXPECT().FetchArticles(ctx, s.cfg.MaxPagesPerSync).Return(articles, nil)
	s.articles.EXPECT().GetExistingBySourceAndExternalIDs(ctx, "test-source", []int64{1}).Return(map[int64]time.Time{}, nil)
	s.articles.EXPECT().GetExistingBySourceAndExternalIDs(ctx, "test-source", []int64{1}).Return(map[int64]time.Time{}, nil)

	s.txManager.EXPECT().WithTransaction(ctx, gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	s.articles.EXPECT().Upsert(ctx, &articles[0]).Return(int64(100), nil)

	s.syncState.EXPECT().Get(ctx, "test-source").Return(&domain.SyncState{SourceID: "test-source"}, nil)
	s.syncState.EXPECT().Update(ctx, gomock.Any()).Return(nil)

	stats, err := service.Sync(ctx)

	s.NoError(err)
	s.Equal(1, stats.New)
	s.Equal(0, stats.Published)
}