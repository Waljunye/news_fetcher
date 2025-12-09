package postgres

import (
	"context"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"news_fetcher/internal/domain"
)

type TagStore struct {
	db *sqlx.DB
}

func NewTagStore(db *sqlx.DB) *TagStore {
	return &TagStore{db: db}
}

func (s *TagStore) UpsertBatch(ctx context.Context, tags []domain.Tag) error {
	if len(tags) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("INSERT INTO tags (id, label) VALUES ")
	valueArgs := make([]interface{}, 0, len(tags)*2)

	for i, tag := range tags {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("($")
		sb.WriteString(itoa(i*2 + 1))
		sb.WriteString(", $")
		sb.WriteString(itoa(i*2 + 2))
		sb.WriteString(")")
		valueArgs = append(valueArgs, tag.ID, tag.Label)
	}
	sb.WriteString(" ON CONFLICT (id) DO UPDATE SET label = EXCLUDED.label")

	_, err := s.db.ExecContext(ctx, sb.String(), valueArgs...)
	return err
}

func (s *TagStore) LinkToArticle(ctx context.Context, articleID int64, tagIDs []int64) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM article_tags WHERE article_id = $1",
		articleID,
	)
	if err != nil {
		return err
	}

	if len(tagIDs) == 0 {
		return nil
	}
	
	var sb strings.Builder
	sb.WriteString("INSERT INTO article_tags (article_id, tag_id) VALUES ")
	valueArgs := make([]interface{}, 0, len(tagIDs)+1)
	valueArgs = append(valueArgs, articleID)

	for i, tagID := range tagIDs {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("($1, $")
		sb.WriteString(itoa(i + 2))
		sb.WriteString(")")
		valueArgs = append(valueArgs, tagID)
	}
	sb.WriteString(" ON CONFLICT DO NOTHING")

	_, err = s.db.ExecContext(ctx, sb.String(), valueArgs...)
	return err
}

func (s *TagStore) GetByArticleID(ctx context.Context, articleID int64) ([]domain.Tag, error) {
	query := `
		SELECT t.id, t.label
		FROM tags t
		INNER JOIN article_tags at ON at.tag_id = t.id
		WHERE at.article_id = $1`

	var tags []domain.Tag
	err := s.db.SelectContext(ctx, &tags, query, articleID)
	return tags, err
}

func (s *TagStore) GetTagIDsByExternalIDs(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `SELECT id FROM tags WHERE id = ANY($1)`
	var result []int64
	err := s.db.SelectContext(ctx, &result, query, pq.Array(ids))
	return result, err
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}