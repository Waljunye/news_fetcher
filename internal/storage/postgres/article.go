package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"news_fetcher/internal/domain"
)

type ArticleStore struct {
	db *sqlx.DB
}

func NewArticleStore(db *sqlx.DB) *ArticleStore {
	return &ArticleStore{db: db}
}

func (s *ArticleStore) Upsert(ctx context.Context, article *domain.Article) (int64, error) {
	query := `
		INSERT INTO articles (
			source_id, external_id, title, description, summary, body, author,
			canonical_url, image_url, published_at, last_modified, duration
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		ON CONFLICT (source_id, external_id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			summary = EXCLUDED.summary,
			body = EXCLUDED.body,
			author = EXCLUDED.author,
			canonical_url = EXCLUDED.canonical_url,
			image_url = EXCLUDED.image_url,
			last_modified = EXCLUDED.last_modified,
			duration = EXCLUDED.duration
		WHERE articles.last_modified < EXCLUDED.last_modified
		RETURNING id`

	var id int64
	err := s.db.QueryRowContext(ctx, query,
		article.SourceID,
		article.ExternalID,
		article.Title,
		article.Description,
		article.Summary,
		article.Body,
		article.Author,
		article.CanonicalURL,
		article.ImageURL,
		article.PublishedAt,
		article.LastModified,
		article.Duration,
	).Scan(&id)

	if err == sql.ErrNoRows {
		err = s.db.QueryRowContext(ctx,
			"SELECT id FROM articles WHERE source_id = $1 AND external_id = $2",
			article.SourceID, article.ExternalID,
		).Scan(&id)
	}

	if err != nil {
		return 0, err
	}

	return id, nil
}

func (s *ArticleStore) GetExistingBySourceAndExternalIDs(ctx context.Context, sourceID string, ids []int64) (map[int64]time.Time, error) {
	if len(ids) == 0 {
		return make(map[int64]time.Time), nil
	}

	query := `SELECT external_id, last_modified FROM articles WHERE source_id = $1 AND external_id = ANY($2)`

	rows, err := s.db.QueryContext(ctx, query, sourceID, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]time.Time)
	for rows.Next() {
		var extID int64
		var lastMod time.Time
		if err := rows.Scan(&extID, &lastMod); err != nil {
			return nil, err
		}
		result[extID] = lastMod
	}

	return result, rows.Err()
}