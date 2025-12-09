//go:build integration

package postgres

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"news_fetcher/internal/domain"
	"news_fetcher/testdata/utils"
)

type PostgresIntegrationSuite struct {
	suite.Suite
	ctx       context.Context
	container *postgres.PostgresContainer
	db        *sqlx.DB
}

func (s *PostgresIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	migrationsPath, err := filepath.Abs("../../../migrations")
	s.Require().NoError(err)

	container, err := postgres.Run(s.ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("test_db"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithInitScripts(
			filepath.Join(migrationsPath, "001_create_articles.up.sql"),
			filepath.Join(migrationsPath, "002_add_source_id.up.sql"),
		),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	s.Require().NoError(err)
	s.container = container

	connStr, err := container.ConnectionString(s.ctx, "sslmode=disable")
	s.Require().NoError(err)

	db, err := sqlx.Connect("postgres", connStr)
	s.Require().NoError(err)
	s.db = db
}

func (s *PostgresIntegrationSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *PostgresIntegrationSuite) SetupTest() {
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM article_tags")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM tags")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM articles")
	_, _ = s.db.ExecContext(s.ctx, "DELETE FROM sync_state")
}

func TestPostgresIntegrationSuite(t *testing.T) {
	suite.Run(t, new(PostgresIntegrationSuite))
}


func (s *PostgresIntegrationSuite) TestArticleStore_Upsert_Insert() {
	store := NewArticleStore(s.db)
	now := time.Now().Truncate(time.Microsecond)

	article := &domain.Article{
		SourceID:     "test-source",
		ExternalID:   123,
		Title:        "Test Article",
		Description:  utils.Ptr("Test Description"),
		Summary:      utils.Ptr("Test Summary"),
		Body:         utils.Ptr("Test Body"),
		Author:       utils.Ptr("Test Author"),
		CanonicalURL: "https://example.com/article",
		ImageURL:     utils.Ptr("https://example.com/image.jpg"),
		PublishedAt:  now,
		LastModified: now,
		Duration:     300,
	}

	id, err := store.Upsert(s.ctx, article)
	s.NoError(err)
	s.Greater(id, int64(0))

	var count int
	err = s.db.GetContext(s.ctx, &count, "SELECT COUNT(*) FROM articles WHERE external_id = $1 AND source_id = $2", 123, "test-source")
	s.NoError(err)
	s.Equal(1, count)
}

func (s *PostgresIntegrationSuite) TestArticleStore_Upsert_UpdateWhenNewer() {
	store := NewArticleStore(s.db)
	now := time.Now().Truncate(time.Microsecond)
	older := now.Add(-1 * time.Hour)

	article := &domain.Article{
		SourceID:     "test-source",
		ExternalID:   123,
		Title:        "Original Title",
		CanonicalURL: "https://example.com/article",
		PublishedAt:  older,
		LastModified: older,
	}
	id1, err := store.Upsert(s.ctx, article)
	s.NoError(err)

	article.Title = "Updated Title"
	article.LastModified = now
	id2, err := store.Upsert(s.ctx, article)
	s.NoError(err)
	s.Equal(id1, id2)

	var title string
	err = s.db.GetContext(s.ctx, &title, "SELECT title FROM articles WHERE id = $1", id1)
	s.NoError(err)
	s.Equal("Updated Title", title)
}

func (s *PostgresIntegrationSuite) TestArticleStore_Upsert_SkipWhenOlder() {
	store := NewArticleStore(s.db)
	now := time.Now().Truncate(time.Microsecond)
	older := now.Add(-1 * time.Hour)

	article := &domain.Article{
		SourceID:     "test-source",
		ExternalID:   123,
		Title:        "Newer Title",
		CanonicalURL: "https://example.com/article",
		PublishedAt:  now,
		LastModified: now,
	}
	id1, err := store.Upsert(s.ctx, article)
	s.NoError(err)

	article.Title = "Older Title"
	article.LastModified = older
	id2, err := store.Upsert(s.ctx, article)
	s.NoError(err)
	s.Equal(id1, id2)

	var title string
	err = s.db.GetContext(s.ctx, &title, "SELECT title FROM articles WHERE id = $1", id1)
	s.NoError(err)
	s.Equal("Newer Title", title)
}

func (s *PostgresIntegrationSuite) TestArticleStore_GetExisting_ReturnsCorrectMap() {
	store := NewArticleStore(s.db)
	now := time.Now().Truncate(time.Microsecond)

	for i := int64(1); i <= 3; i++ {
		article := &domain.Article{
			SourceID:     "test-source",
			ExternalID:   i * 100,
			Title:        "Article",
			CanonicalURL: "https://example.com/article",
			PublishedAt:  now,
			LastModified: now.Add(time.Duration(i) * time.Hour),
		}
		_, err := store.Upsert(s.ctx, article)
		s.NoError(err)
	}

	result, err := store.GetExistingBySourceAndExternalIDs(s.ctx, "test-source", []int64{100, 200, 999})
	s.NoError(err)
	s.Len(result, 2)

	s.Contains(result, int64(100))
	s.Contains(result, int64(200))
	s.NotContains(result, int64(999))
}

func (s *PostgresIntegrationSuite) TestArticleStore_GetExisting_DifferentSources() {
	store := NewArticleStore(s.db)
	now := time.Now().Truncate(time.Microsecond)

	article1 := &domain.Article{
		SourceID:     "source1",
		ExternalID:   100,
		Title:        "Article Source 1",
		CanonicalURL: "https://example.com/article1",
		PublishedAt:  now,
		LastModified: now,
	}
	_, err := store.Upsert(s.ctx, article1)
	s.NoError(err)

	article2 := &domain.Article{
		SourceID:     "source2",
		ExternalID:   100,
		Title:        "Article Source 2",
		CanonicalURL: "https://example.com/article2",
		PublishedAt:  now,
		LastModified: now,
	}
	_, err = store.Upsert(s.ctx, article2)
	s.NoError(err)

	result, err := store.GetExistingBySourceAndExternalIDs(s.ctx, "source1", []int64{100})
	s.NoError(err)
	s.Len(result, 1)

	result, err = store.GetExistingBySourceAndExternalIDs(s.ctx, "source2", []int64{100})
	s.NoError(err)
	s.Len(result, 1)

	result, err = store.GetExistingBySourceAndExternalIDs(s.ctx, "source3", []int64{100})
	s.NoError(err)
	s.Len(result, 0)
}


func (s *PostgresIntegrationSuite) TestTagStore_UpsertBatch() {
	store := NewTagStore(s.db)

	tags := []domain.Tag{
		{ID: 1, Label: "tag1"},
		{ID: 2, Label: "tag2"},
		{ID: 3, Label: "tag3"},
	}

	err := store.UpsertBatch(s.ctx, tags)
	s.NoError(err)

	var count int
	err = s.db.GetContext(s.ctx, &count, "SELECT COUNT(*) FROM tags")
	s.NoError(err)
	s.Equal(3, count)
}

func (s *PostgresIntegrationSuite) TestTagStore_UpsertBatch_UpdatesExisting() {
	store := NewTagStore(s.db)

	tags := []domain.Tag{
		{ID: 1, Label: "old-label"},
	}
	err := store.UpsertBatch(s.ctx, tags)
	s.NoError(err)

	tags = []domain.Tag{
		{ID: 1, Label: "new-label"},
	}
	err = store.UpsertBatch(s.ctx, tags)
	s.NoError(err)

	var label string
	err = s.db.GetContext(s.ctx, &label, "SELECT label FROM tags WHERE id = $1", 1)
	s.NoError(err)
	s.Equal("new-label", label)
}

func (s *PostgresIntegrationSuite) TestTagStore_LinkToArticle() {
	tagStore := NewTagStore(s.db)
	articleStore := NewArticleStore(s.db)
	now := time.Now().Truncate(time.Microsecond)

	article := &domain.Article{
		SourceID:     "test-source",
		ExternalID:   123,
		Title:        "Test Article",
		CanonicalURL: "https://example.com/article",
		PublishedAt:  now,
		LastModified: now,
	}
	articleID, err := articleStore.Upsert(s.ctx, article)
	s.NoError(err)

	tags := []domain.Tag{
		{ID: 1, Label: "tag1"},
		{ID: 2, Label: "tag2"},
	}
	err = tagStore.UpsertBatch(s.ctx, tags)
	s.NoError(err)

	err = tagStore.LinkToArticle(s.ctx, articleID, []int64{1, 2})
	s.NoError(err)

	var count int
	err = s.db.GetContext(s.ctx, &count, "SELECT COUNT(*) FROM article_tags WHERE article_id = $1", articleID)
	s.NoError(err)
	s.Equal(2, count)
}

func (s *PostgresIntegrationSuite) TestTagStore_LinkToArticle_ReplacesOld() {
	tagStore := NewTagStore(s.db)
	articleStore := NewArticleStore(s.db)
	now := time.Now().Truncate(time.Microsecond)

	article := &domain.Article{
		SourceID:     "test-source",
		ExternalID:   123,
		Title:        "Test Article",
		CanonicalURL: "https://example.com/article",
		PublishedAt:  now,
		LastModified: now,
	}
	articleID, err := articleStore.Upsert(s.ctx, article)
	s.NoError(err)

	tags := []domain.Tag{
		{ID: 1, Label: "tag1"},
		{ID: 2, Label: "tag2"},
		{ID: 3, Label: "tag3"},
	}
	err = tagStore.UpsertBatch(s.ctx, tags)
	s.NoError(err)

	err = tagStore.LinkToArticle(s.ctx, articleID, []int64{1, 2})
	s.NoError(err)

	err = tagStore.LinkToArticle(s.ctx, articleID, []int64{3})
	s.NoError(err)

	linkedTags, err := tagStore.GetByArticleID(s.ctx, articleID)
	s.NoError(err)
	s.Len(linkedTags, 1)
	s.Equal(int64(3), linkedTags[0].ID)
}


func (s *PostgresIntegrationSuite) TestSyncStateStore_GetNew() {
	store := NewSyncStateStore(s.db)

	state, err := store.Get(s.ctx, "new-source")
	s.NoError(err)
	s.NotNil(state)
	s.Equal("new-source", state.SourceID)
	s.True(state.LastSyncedAt.IsZero())
	s.Equal(int64(0), state.TotalSynced)
}

func (s *PostgresIntegrationSuite) TestSyncStateStore_UpdateAndGet() {
	store := NewSyncStateStore(s.db)
	now := time.Now().Truncate(time.Microsecond)

	state := &domain.SyncState{
		SourceID:      "test-source",
		LastSyncedAt:  now,
		LastArticleID: 12345,
		TotalSynced:   100,
	}
	err := store.Update(s.ctx, state)
	s.NoError(err)

	retrieved, err := store.Get(s.ctx, "test-source")
	s.NoError(err)
	s.Equal("test-source", retrieved.SourceID)
	s.Equal(int64(12345), retrieved.LastArticleID)
	s.Equal(int64(100), retrieved.TotalSynced)
	s.WithinDuration(now, retrieved.LastSyncedAt, time.Second)
}

func (s *PostgresIntegrationSuite) TestSyncStateStore_UpdateExisting() {
	store := NewSyncStateStore(s.db)
	now := time.Now().Truncate(time.Microsecond)

	state := &domain.SyncState{
		SourceID:      "test-source",
		LastSyncedAt:  now,
		LastArticleID: 100,
		TotalSynced:   10,
	}
	err := store.Update(s.ctx, state)
	s.NoError(err)

	state.LastArticleID = 200
	state.TotalSynced = 20
	err = store.Update(s.ctx, state)
	s.NoError(err)

	retrieved, err := store.Get(s.ctx, "test-source")
	s.NoError(err)
	s.Equal(int64(200), retrieved.LastArticleID)
	s.Equal(int64(20), retrieved.TotalSynced)
}

func (s *PostgresIntegrationSuite) TestTransaction_Commit() {
	tm := NewTransactionManager(s.db)
	articleStore := NewArticleStore(s.db)
	now := time.Now().Truncate(time.Microsecond)

	err := tm.WithTransaction(s.ctx, func(ctx context.Context) error {
		article := &domain.Article{
			SourceID:     "test-source",
			ExternalID:   999,
			Title:        "Transaction Article",
			CanonicalURL: "https://example.com/tx-article",
			PublishedAt:  now,
			LastModified: now,
		}
		_, err := articleStore.Upsert(ctx, article)
		return err
	})
	s.NoError(err)

	var count int
	err = s.db.GetContext(s.ctx, &count, "SELECT COUNT(*) FROM articles WHERE external_id = $1", 999)
	s.NoError(err)
	s.Equal(1, count)
}

func (s *PostgresIntegrationSuite) TestTransaction_Rollback() {
	tm := NewTransactionManager(s.db)
	now := time.Now().Truncate(time.Microsecond)

	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO articles (source_id, external_id, title, canonical_url, published_at, last_modified)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "test-source", 888, "Pre-existing", "https://example.com", now, now)
	s.NoError(err)

	err = tm.WithTransaction(s.ctx, func(ctx context.Context) error {
		exec := GetExecutor(ctx, s.db)

		_, err := exec.ExecContext(ctx, `
			INSERT INTO articles (source_id, external_id, title, canonical_url, published_at, last_modified)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, "test-source", 777, "Should Rollback", "https://example.com", now, now)
		if err != nil {
			return err
		}

		return context.Canceled
	})
	s.Error(err)

	var count int
	err = s.db.GetContext(s.ctx, &count, "SELECT COUNT(*) FROM articles WHERE external_id = $1", 777)
	s.NoError(err)
	s.Equal(0, count)

	err = s.db.GetContext(s.ctx, &count, "SELECT COUNT(*) FROM articles WHERE external_id = $1", 888)
	s.NoError(err)
	s.Equal(1, count)
}