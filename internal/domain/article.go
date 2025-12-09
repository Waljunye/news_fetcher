package domain

import "time"

type Article struct {
	ID           int64
	SourceID     string // identifies the source (e.g., "ecb", "espn")
	ExternalID   int64
	Title        string
	Description  *string
	Summary      *string
	Body         *string
	Author       *string
	CanonicalURL string
	ImageURL     *string
	PublishedAt  time.Time
	LastModified time.Time
	Duration     int
	Tags         []Tag
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Tag struct {
	ID    int64
	Label string
}

type SyncState struct {
	ID            int64     `db:"id"`
	SourceID      string    `db:"source_id"`
	LastSyncedAt  time.Time `db:"last_synced_at"`
	LastArticleID int64     `db:"last_article_id"`
	TotalSynced   int64     `db:"total_synced"`
}