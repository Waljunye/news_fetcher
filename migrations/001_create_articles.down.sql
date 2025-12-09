DROP TRIGGER IF EXISTS update_articles_updated_at ON articles;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS article_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS sync_state;
DROP TABLE IF EXISTS articles;