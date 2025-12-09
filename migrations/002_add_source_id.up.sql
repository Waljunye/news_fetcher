-- Add source_id column to articles
ALTER TABLE articles ADD COLUMN source_id VARCHAR(50) NOT NULL DEFAULT 'ecb';

-- Drop old unique constraint and create new composite one
ALTER TABLE articles DROP CONSTRAINT IF EXISTS articles_external_id_key;
ALTER TABLE articles ADD CONSTRAINT articles_source_external_unique UNIQUE (source_id, external_id);

-- Update index
DROP INDEX IF EXISTS idx_articles_external_id;
CREATE INDEX IF NOT EXISTS idx_articles_source_external ON articles(source_id, external_id);

-- Add source_id to sync_state
ALTER TABLE sync_state ADD COLUMN source_id VARCHAR(50) NOT NULL DEFAULT 'ecb';
ALTER TABLE sync_state DROP CONSTRAINT IF EXISTS sync_state_pkey;
ALTER TABLE sync_state ADD CONSTRAINT sync_state_source_unique UNIQUE (source_id);

-- Migrate existing row
UPDATE sync_state SET source_id = 'ecb' WHERE id = 1;