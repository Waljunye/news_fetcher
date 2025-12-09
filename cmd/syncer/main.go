package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"news_fetcher/internal/config"
	"news_fetcher/internal/publisher"
	"news_fetcher/internal/scheduler"
	"news_fetcher/internal/service"
	"news_fetcher/internal/source/ecb"
	"news_fetcher/internal/storage/postgres"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Setup logger
	logger := setupLogger("info")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger = setupLogger(cfg.LogLevel)
	logger.Info(cfg.RabbitMQ.URL)

	db, err := sqlx.Connect("postgres", cfg.Database.DSN())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to database")

	// Initialize RabbitMQ publisher
	rabbitMQ, err := publisher.NewRabbitMQ(publisher.Config{
		URL:        cfg.RabbitMQ.URL,
		Exchange:   cfg.RabbitMQ.Exchange,
		RoutingKey: cfg.RabbitMQ.RoutingKey,
		QueueName:  cfg.RabbitMQ.QueueName,
	}, logger)
	if err != nil {
		logger.Error("failed to connect to rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rabbitMQ.Close()

	// Initialize stores
	articleStore := postgres.NewArticleStore(db)
	tagStore := postgres.NewTagStore(db)
	syncStateStore := postgres.NewSyncStateStore(db)
	txManager := postgres.NewTransactionManager(db)

	// Initialize ECB source
	ecbSource := ecb.New(ecb.Config{
		BaseURL:        cfg.API.BaseURL,
		PageSize:       cfg.API.PageSize,
		Timeout:        cfg.API.Timeout,
		MaxAttempts:    cfg.API.Retry.MaxAttempts,
		InitialBackoff: cfg.API.Retry.InitialBackoff,
		MaxBackoff:     cfg.API.Retry.MaxBackoff,
	}, logger)

	// Create sync service for ECB source
	syncService := service.NewSyncService(
		ecbSource,
		articleStore,
		tagStore,
		syncStateStore,
		txManager,
		rabbitMQ,
		logger,
		cfg.Sync,
	)

	sched := scheduler.NewScheduler(syncService, cfg.Sync.Interval, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	logger.Info("starting news syncer",
		"source", ecbSource.Name(),
		"interval", cfg.Sync.Interval,
		"max_pages", cfg.Sync.MaxPagesPerSync,
	)

	if err := sched.Start(ctx); err != nil && err != context.Canceled {
		logger.Error("scheduler error", "error", err)
		os.Exit(1)
	}
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: logLevel}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}
