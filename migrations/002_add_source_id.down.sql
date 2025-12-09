-- Remove source_id from sync_state
ALTER TABLE sync_state DROP CONSTRAINT IF EXISTS sync_state_source_unique;
ALTER TABLE sync_state DROP COLUMN IF EXISTS source_id;
ALTER TABLE sync_state ADD CONSTRAINT sync_state_pkey PRIMARY KEY (id);

-- Remove source_id from articles
DROP INDEX IF EXISTS idx_articles_source_external;
ALTER TABLE articles DROP CONSTRAINT IF EXISTS articles_source_external_unique;
ALTER TABLE articles ADD CONSTRAINT articles_external_id_key UNIQUE (external_id);
CREATE INDEX IF NOT EXISTS idx_articles_external_id ON articles(external_id);
ALTER TABLE articles DROP COLUMN IF EXISTS source_id;