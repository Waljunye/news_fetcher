CREATE TABLE IF NOT EXISTS articles (
    id              BIGSERIAL PRIMARY KEY,
    external_id     BIGINT NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    description     TEXT,
    summary         TEXT,
    body            TEXT,
    author          VARCHAR(255),
    canonical_url   TEXT NOT NULL,
    image_url       TEXT,
    published_at    TIMESTAMP WITH TIME ZONE NOT NULL,
    last_modified   TIMESTAMP WITH TIME ZONE NOT NULL,
    duration        INTEGER DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_articles_external_id ON articles(external_id);
CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_last_modified ON articles(last_modified DESC);

CREATE TABLE IF NOT EXISTS tags (
    id      BIGINT PRIMARY KEY,
    label   VARCHAR(255) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS article_tags (
    article_id  BIGINT REFERENCES articles(id) ON DELETE CASCADE,
    tag_id      BIGINT REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (article_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_article_tags_article_id ON article_tags(article_id);
CREATE INDEX IF NOT EXISTS idx_article_tags_tag_id ON article_tags(tag_id);

CREATE TABLE IF NOT EXISTS sync_state (
    id              SERIAL PRIMARY KEY,
    last_synced_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_article_id BIGINT DEFAULT 0,
    total_synced    BIGINT DEFAULT 0
);

INSERT INTO sync_state (id, total_synced) VALUES (1, 0)
ON CONFLICT (id) DO NOTHING;

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_articles_updated_at
    BEFORE UPDATE ON articles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();