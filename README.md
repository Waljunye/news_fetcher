# News Fetcher

A service that synchronizes news articles from external APIs. Periodically polls data sources, stores articles in PostgreSQL, and publishes to RabbitMQ.

## Quick Start

### Linux / macOS

```bash
# Start infrastructure and wait for healthy
docker-compose up -d
until docker-compose exec postgres pg_isready -U postgres; do sleep 1; done
until docker-compose exec rabbitmq rabbitmq-diagnostics is_running; do sleep 1; done
until docker-compose exec rabbitmq rabbitmq-diagnostics is_booting; do sleep 1; done
until docker-compose exec rabbitmq rabbitmq-diagnostics check_port_listener 5672; do sleep 1; done

# Apply migrations
migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/news_fetcher?sslmode=disable" up

# Run service
go run cmd/syncer/main.go -config config.yaml
```

### Windows (PowerShell)

```powershell
# Start infrastructure and wait for healthy
docker-compose up -d
do { Start-Sleep 1 } until (docker-compose exec postgres pg_isready -U postgres)
do { Start-Sleep 1 } until (docker-compose exec rabbitmq rabbitmq-diagnostics is_running)
do { Start-Sleep 1 } until (docker-compose exec rabbitmq rabbitmq-diagnostics is_booting)
do { Start-Sleep 1 } until (docker-compose exec rabbitmq rabbitmq-diagnostics check_port_listener 5672)

# Apply migrations
migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/news_fetcher?sslmode=disable" up

# Run service
go run cmd/syncer/main.go -config config.yaml
```

## Docker Compose

### Start

```bash
docker-compose up -d
```

### Services

| Service | Port | Description |
|---------|------|-------------|
| PostgreSQL | 5432 | Database |
| RabbitMQ | 5672 | AMQP |
| RabbitMQ UI | 15672 | Management UI (guest/guest) |

### Environment Variables

```bash
# PostgreSQL
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=news_fetcher
DB_PORT=5432

# RabbitMQ
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest
RABBITMQ_PORT=5672
RABBITMQ_MGMT_PORT=15672
```

### Management

```bash
# Stop
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Logs
docker-compose logs -f

# Status
docker-compose ps
```

## Project Structure

```
news_fetcher/
├── cmd/syncer/              # Entry point
├── internal/
│   ├── config/              # Configuration
│   ├── domain/              # Domain models
│   ├── source/ecb/          # ECB API client
│   ├── publisher/           # RabbitMQ publisher
│   ├── storage/postgres/    # PostgreSQL storage
│   ├── service/             # Business logic
│   └── scheduler/           # Scheduler
├── migrations/              # SQL migrations
├── testdata/utils/          # Test utilities
├── config.yaml              # Configuration
└── docker-compose.yaml      # Docker Compose
```

## Configuration

```yaml
database:
  host: localhost
  port: 5432
  user: ${DB_USER}
  password: ${DB_PASSWORD}
  dbname: news_fetcher
  sslmode: disable

rabbitmq:
  url: amqp://${RABBITMQ_USER}:${RABBITMQ_PASSWORD}@localhost:5672/
  exchange: news_fetcher
  routing_key: articles
  queue_name: cms_articles

api:
  base_url: https://content-ecb.pulselive.com/content/ecb/text/EN/
  page_size: 20
  timeout: 30s
  retry:
    max_attempts: 3
    initial_backoff: 1s
    max_backoff: 30s

sync:
  interval: 5m
  max_pages_per_sync: 5
  max_historical_days: 30

log_level: info
```

## Testing

```bash
# Unit tests
go test ./...

# Integration tests (require Docker)
go test ./... -tags=integration

# Generate mocks
go generate ./...
```

## RabbitMQ Message Format

```json
{
  "action": "create",
  "article": {
    "source_id": "ecb",
    "external_id": 67890,
    "title": "Article Title",
    "description": "Article description",
    "body": "Full article body...",
    "author": "John Doe",
    "canonical_url": "https://example.com/article",
    "image_url": "https://example.com/image.jpg",
    "published_at": "2025-01-15T10:00:00Z",
    "last_modified": "2025-01-15T12:00:00Z",
    "tags": [
      {"id": 1, "label": "Cricket"},
      {"id": 2, "label": "News"}
    ]
  },
  "timestamp": "2025-01-15T14:30:00Z"
}
```

- `action`: `"create"` for new articles, `"update"` for updated articles

## Architecture

### Deduplication

1. Articles are identified by `(source_id, external_id)`
2. Before sync, query existing `external_id` and their `last_modified`
3. Only sync new or updated articles
4. UPSERT with condition `WHERE last_modified < EXCLUDED.last_modified`

### Multi-source

Architecture supports multiple data sources via `Source` interface:

```go
type Source interface {
    ID() string
    Name() string
    FetchArticles(ctx context.Context, maxPages int) ([]domain.Article, error)
}
```