package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	RabbitMQ RabbitMQConfig `yaml:"rabbitmq"`
	API      APIConfig      `yaml:"api"`
	Sync     SyncConfig     `yaml:"sync"`
	LogLevel string         `yaml:"log_level"`
}

type RabbitMQConfig struct {
	URL        string `yaml:"url"`
	Exchange   string `yaml:"exchange"`
	RoutingKey string `yaml:"routing_key"`
	QueueName  string `yaml:"queue_name"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

type APIConfig struct {
	BaseURL  string        `yaml:"base_url"`
	PageSize int           `yaml:"page_size"`
	Timeout  time.Duration `yaml:"timeout"`
	Retry    RetryConfig   `yaml:"retry"`
}

type RetryConfig struct {
	MaxAttempts    int           `yaml:"max_attempts"`
	InitialBackoff time.Duration `yaml:"initial_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff"`
}

type SyncConfig struct {
	Interval          time.Duration `yaml:"interval"`
	MaxPagesPerSync   int           `yaml:"max_pages_per_sync"`
	MaxHistoricalDays int           `yaml:"max_historical_days"`
}

func Load(path string) (*Config, error) {
	_ = godotenv.Load()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.setDefaults()

	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.RabbitMQ.URL == "" {
		c.RabbitMQ.URL = "amqp://guest:guest@localhost:5672/"
	}
	if c.RabbitMQ.Exchange == "" {
		c.RabbitMQ.Exchange = "news_fetcher"
	}
	if c.RabbitMQ.RoutingKey == "" {
		c.RabbitMQ.RoutingKey = "articles"
	}
	if c.RabbitMQ.QueueName == "" {
		c.RabbitMQ.QueueName = "cms_articles"
	}
	if c.API.PageSize == 0 {
		c.API.PageSize = 20
	}
	if c.API.Timeout == 0 {
		c.API.Timeout = 30 * time.Second
	}
	if c.API.Retry.MaxAttempts == 0 {
		c.API.Retry.MaxAttempts = 3
	}
	if c.API.Retry.InitialBackoff == 0 {
		c.API.Retry.InitialBackoff = 1 * time.Second
	}
	if c.API.Retry.MaxBackoff == 0 {
		c.API.Retry.MaxBackoff = 30 * time.Second
	}
	if c.Sync.Interval == 0 {
		c.Sync.Interval = 5 * time.Minute
	}
	if c.Sync.MaxPagesPerSync == 0 {
		c.Sync.MaxPagesPerSync = 5
	}
	if c.Sync.MaxHistoricalDays == 0 {
		c.Sync.MaxHistoricalDays = 30
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
}