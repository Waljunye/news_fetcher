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
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	DBName          string        `yaml:"dbname"`
	SSLMode         string        `yaml:"sslmode"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

func (d DatabaseConfig) DSN() string {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
	if d.ConnectTimeout > 0 {
		dsn += fmt.Sprintf(" connect_timeout=%d", int(d.ConnectTimeout.Seconds()))
	}
	return dsn
}

type APIConfig struct {
	BaseURL   string        `yaml:"base_url"`
	PageSize  int           `yaml:"page_size"`
	PageDelay time.Duration `yaml:"page_delay"`
	Timeout   time.Duration `yaml:"timeout"`
	Retry     RetryConfig   `yaml:"retry"`
}

type RetryConfig struct {
	MaxAttempts    int           `yaml:"max_attempts"`
	InitialBackoff time.Duration `yaml:"initial_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff"`
}

type SyncConfig struct {
	Interval          time.Duration `yaml:"interval"`
	Timeout           time.Duration `yaml:"timeout"`
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
	if c.API.PageDelay == 0 {
		c.API.PageDelay = 500 * time.Millisecond
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
	if c.Sync.Timeout == 0 {
		c.Sync.Timeout = 5 * time.Minute
	}
	if c.Sync.MaxPagesPerSync == 0 {
		c.Sync.MaxPagesPerSync = 5
	}
	if c.Sync.MaxHistoricalDays == 0 {
		c.Sync.MaxHistoricalDays = 30
	}
	if c.Database.Host == "" {
		c.Database.Host = "localhost"
	}
	if c.Database.DBName == "" {
		c.Database.DBName = "news_fetcher"
	}
	if c.Database.ConnectTimeout == 0 {
		c.Database.ConnectTimeout = 10 * time.Second
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 25
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 5
	}
	if c.Database.ConnMaxLifetime == 0 {
		c.Database.ConnMaxLifetime = 5 * time.Minute
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
}
