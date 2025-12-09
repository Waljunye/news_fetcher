package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"news_fetcher/internal/domain"
)

type RabbitMQ struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	exchange   string
	routingKey string
	logger     *slog.Logger
}

type Config struct {
	URL        string
	Exchange   string
	RoutingKey string
	QueueName  string
}

func NewRabbitMQ(cfg Config, logger *slog.Logger) (*RabbitMQ, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		cfg.Exchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	q, err := ch.QueueDeclare(
		cfg.QueueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	err = ch.QueueBind(
		q.Name,
		cfg.RoutingKey,
		cfg.Exchange,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("bind queue: %w", err)
	}

	logger.Info("connected to rabbitmq",
		"exchange", cfg.Exchange,
		"queue", cfg.QueueName,
		"routing_key", cfg.RoutingKey,
	)

	return &RabbitMQ{
		conn:       conn,
		channel:    ch,
		exchange:   cfg.Exchange,
		routingKey: cfg.RoutingKey,
		logger:     logger,
	}, nil
}

type ArticleMessage struct {
	Action    string         `json:"action"` // "create" or "update"
	Article   domain.Article `json:"article"`
	Timestamp time.Time      `json:"timestamp"`
}

func (r *RabbitMQ) Publish(ctx context.Context, article *domain.Article, isNew bool) error {
	action := "update"
	if isNew {
		action = "create"
	}

	msg := ArticleMessage{
		Action:    action,
		Article:   *article,
		Timestamp: time.Now().UTC(),
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		r.exchange,
		r.routingKey,
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	r.logger.Debug("published article",
		"external_id", article.ExternalID,
		"action", action,
	)

	return nil
}

func (r *RabbitMQ) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
