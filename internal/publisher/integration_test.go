//go:build integration

package publisher

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"

	"news_fetcher/internal/domain"
	"news_fetcher/testdata/utils"
)

type RabbitMQIntegrationSuite struct {
	suite.Suite
	ctx       context.Context
	container *rabbitmq.RabbitMQContainer
	amqpURL   string
	logger    *slog.Logger
}

func (s *RabbitMQIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	container, err := rabbitmq.Run(s.ctx,
		"rabbitmq:3.13-management-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Server startup complete").
				WithStartupTimeout(60*time.Second),
		),
	)
	s.Require().NoError(err)
	s.container = container

	amqpURL, err := container.AmqpURL(s.ctx)
	s.Require().NoError(err)
	s.amqpURL = amqpURL
}

func (s *RabbitMQIntegrationSuite) TearDownSuite() {
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func TestRabbitMQIntegrationSuite(t *testing.T) {
	suite.Run(t, new(RabbitMQIntegrationSuite))
}

func (s *RabbitMQIntegrationSuite) TestPublisher_Connection() {
	cfg := Config{
		URL:        s.amqpURL,
		Exchange:   "test-exchange",
		RoutingKey: "test-routing-key",
		QueueName:  "test-queue",
	}

	pub, err := NewRabbitMQ(cfg, s.logger)
	s.NoError(err)
	s.NotNil(pub)

	err = pub.Close()
	s.NoError(err)
}

func (s *RabbitMQIntegrationSuite) TestPublisher_PublishCreate() {
	cfg := Config{
		URL:        s.amqpURL,
		Exchange:   "test-exchange-create",
		RoutingKey: "test-routing-key-create",
		QueueName:  "test-queue-create",
	}

	pub, err := NewRabbitMQ(cfg, s.logger)
	s.Require().NoError(err)
	defer pub.Close()

	now := time.Now().Truncate(time.Millisecond)
	article := &domain.Article{
		ID:           1,
		SourceID:     "test-source",
		ExternalID:   123,
		Title:        "Test Article",
		Description:  utils.Ptr("Test Description"),
		CanonicalURL: "https://example.com/article",
		PublishedAt:  now,
		LastModified: now,
	}

	err = pub.Publish(s.ctx, article, true)
	s.NoError(err)

	msg := s.consumeMessage(cfg)
	s.NotNil(msg)

	var received ArticleMessage
	err = json.Unmarshal(msg.Body, &received)
	s.NoError(err)
	s.Equal("create", received.Action)
	s.Equal(int64(123), received.Article.ExternalID)
	s.Equal("Test Article", received.Article.Title)
}

func (s *RabbitMQIntegrationSuite) TestPublisher_PublishUpdate() {
	cfg := Config{
		URL:        s.amqpURL,
		Exchange:   "test-exchange-update",
		RoutingKey: "test-routing-key-update",
		QueueName:  "test-queue-update",
	}

	pub, err := NewRabbitMQ(cfg, s.logger)
	s.Require().NoError(err)
	defer pub.Close()

	now := time.Now().Truncate(time.Millisecond)
	article := &domain.Article{
		ID:           2,
		SourceID:     "test-source",
		ExternalID:   456,
		Title:        "Updated Article",
		CanonicalURL: "https://example.com/updated",
		PublishedAt:  now,
		LastModified: now,
	}

	err = pub.Publish(s.ctx, article, false)
	s.NoError(err)

	msg := s.consumeMessage(cfg)
	s.NotNil(msg)

	var received ArticleMessage
	err = json.Unmarshal(msg.Body, &received)
	s.NoError(err)
	s.Equal("update", received.Action)
	s.Equal(int64(456), received.Article.ExternalID)
}

func (s *RabbitMQIntegrationSuite) TestPublisher_MessageFormat() {
	cfg := Config{
		URL:        s.amqpURL,
		Exchange:   "test-exchange-format",
		RoutingKey: "test-routing-key-format",
		QueueName:  "test-queue-format",
	}

	pub, err := NewRabbitMQ(cfg, s.logger)
	s.Require().NoError(err)
	defer pub.Close()

	now := time.Now().Truncate(time.Millisecond)
	article := &domain.Article{
		ID:           3,
		SourceID:     "ecb",
		ExternalID:   789,
		Title:        "Full Article",
		Description:  utils.Ptr("Full Description"),
		Summary:      utils.Ptr("Full Summary"),
		Body:         utils.Ptr("Full Body"),
		Author:       utils.Ptr("Test Author"),
		CanonicalURL: "https://example.com/full",
		ImageURL:     utils.Ptr("https://example.com/image.jpg"),
		PublishedAt:  now,
		LastModified: now,
		Duration:     300,
		Tags: []domain.Tag{
			{ID: 1, Label: "tag1"},
			{ID: 2, Label: "tag2"},
		},
	}

	err = pub.Publish(s.ctx, article, true)
	s.NoError(err)

	msg := s.consumeMessage(cfg)
	s.NotNil(msg)

	s.Equal("application/json", msg.ContentType)

	var received ArticleMessage
	err = json.Unmarshal(msg.Body, &received)
	s.NoError(err)

	s.Equal("create", received.Action)
	s.Equal("ecb", received.Article.SourceID)
	s.Equal(int64(789), received.Article.ExternalID)
	s.Equal("Full Article", received.Article.Title)
	s.NotNil(received.Article.Description)
	s.Equal("Full Description", *received.Article.Description)
	s.NotNil(received.Article.Summary)
	s.Equal("Full Summary", *received.Article.Summary)
	s.NotNil(received.Article.Author)
	s.Equal("Test Author", *received.Article.Author)
	s.Equal(300, received.Article.Duration)
	s.Len(received.Article.Tags, 2)
	s.False(received.Timestamp.IsZero())
}

func (s *RabbitMQIntegrationSuite) TestPublisher_MessagePersistence() {
	cfg := Config{
		URL:        s.amqpURL,
		Exchange:   "test-exchange-persist",
		RoutingKey: "test-routing-key-persist",
		QueueName:  "test-queue-persist",
	}

	pub, err := NewRabbitMQ(cfg, s.logger)
	s.Require().NoError(err)
	defer pub.Close()

	now := time.Now().Truncate(time.Millisecond)
	article := &domain.Article{
		SourceID:     "test",
		ExternalID:   999,
		Title:        "Persistent Article",
		CanonicalURL: "https://example.com/persist",
		PublishedAt:  now,
		LastModified: now,
	}

	err = pub.Publish(s.ctx, article, true)
	s.NoError(err)

	msg := s.consumeMessage(cfg)
	s.NotNil(msg)

	s.Equal(uint8(amqp.Persistent), msg.DeliveryMode)
}

func (s *RabbitMQIntegrationSuite) consumeMessage(cfg Config) *amqp.Delivery {
	conn, err := amqp.Dial(s.amqpURL)
	s.Require().NoError(err)
	defer conn.Close()

	ch, err := conn.Channel()
	s.Require().NoError(err)
	defer ch.Close()

	msgs, err := ch.Consume(cfg.QueueName, "", true, false, false, false, nil)
	s.Require().NoError(err)

	select {
	case msg := <-msgs:
		return &msg
	case <-time.After(5 * time.Second):
		s.Fail("Timeout waiting for message")
		return nil
	}
}