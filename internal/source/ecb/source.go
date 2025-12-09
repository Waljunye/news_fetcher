package ecb

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"news_fetcher/internal/domain"
)

const (
	SourceID   = "ecb"
	SourceName = "ECB Cricket"
)

// Config holds ECB source configuration.
type Config struct {
	BaseURL        string
	PageSize       int
	Timeout        time.Duration
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// Source implements source.Source for ECB Cricket API.
type Source struct {
	httpClient *http.Client
	baseURL    string
	pageSize   int
	maxAttempts    int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	logger     *slog.Logger
}

// New creates a new ECB source.
func New(cfg Config, logger *slog.Logger) *Source {
	return &Source{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL:        cfg.BaseURL,
		pageSize:       cfg.PageSize,
		maxAttempts:    cfg.MaxAttempts,
		initialBackoff: cfg.InitialBackoff,
		maxBackoff:     cfg.MaxBackoff,
		logger:         logger.With("source", SourceID),
	}
}

// ID returns the source identifier.
func (s *Source) ID() string {
	return SourceID
}

// Name returns human-readable name.
func (s *Source) Name() string {
	return SourceName
}

// FetchArticles fetches articles from ECB API.
func (s *Source) FetchArticles(ctx context.Context, maxPages int) ([]domain.Article, error) {
	var allContent []Content

	for page := 0; page < maxPages; page++ {
		resp, err := s.fetchPage(ctx, page)
		if err != nil {
			return s.transform(allContent), fmt.Errorf("fetch page %d: %w", page, err)
		}

		allContent = append(allContent, resp.Content...)

		s.logger.Debug("fetched page",
			"page", page,
			"articles", len(resp.Content),
			"total", len(allContent),
		)

		if page >= resp.PageInfo.NumPages-1 {
			break
		}
	}

	return s.transform(allContent), nil
}

func (s *Source) fetchPage(ctx context.Context, page int) (*APIResponse, error) {
	url := fmt.Sprintf("%s?pageSize=%d&page=%d", s.baseURL, s.pageSize, page)

	var resp *APIResponse
	var err error

	for attempt := 1; attempt <= s.maxAttempts; attempt++ {
		resp, err = s.doRequest(ctx, url)
		if err == nil {
			return resp, nil
		}

		if attempt == s.maxAttempts {
			break
		}

		backoff := s.calculateBackoff(attempt)
		s.logger.Warn("request failed, retrying",
			"attempt", attempt,
			"backoff", backoff,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("after %d attempts: %w", s.maxAttempts, err)
}

func (s *Source) doRequest(ctx context.Context, url string) (*APIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "NewsFetcher/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &apiResp, nil
}

func (s *Source) calculateBackoff(attempt int) time.Duration {
	backoff := s.initialBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
	}
	if backoff > s.maxBackoff {
		backoff = s.maxBackoff
	}
	return backoff
}

func (s *Source) transform(contents []Content) []domain.Article {
	articles := make([]domain.Article, 0, len(contents))

	for _, c := range contents {
		publishedAt, err := time.Parse(time.RFC3339, c.Date)
		if err != nil {
			s.logger.Warn("failed to parse date",
				"external_id", c.ID,
				"date", c.Date,
			)
			continue
		}

		lastModified := time.UnixMilli(c.LastModified)

		article := domain.Article{
			SourceID:     SourceID,
			ExternalID:   c.ID,
			Title:        c.Title,
			Description:  c.Description,
			Summary:      c.Summary,
			Body:         c.Body,
			Author:       c.Author,
			CanonicalURL: c.CanonicalURL,
			PublishedAt:  publishedAt,
			LastModified: lastModified,
			Duration:     c.Duration,
		}

		if c.LeadMedia != nil && c.LeadMedia.ImageURL != "" {
			article.ImageURL = &c.LeadMedia.ImageURL
		}

		for _, tag := range c.Tags {
			article.Tags = append(article.Tags, domain.Tag{
				ID:    tag.ID,
				Label: tag.Label,
			})
		}

		articles = append(articles, article)
	}

	return articles
}