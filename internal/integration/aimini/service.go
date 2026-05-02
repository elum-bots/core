package aimini

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/elum-bots/core/internal/db"
	sharedhttp "github.com/elum-bots/core/internal/integration/httpclient"
	"github.com/elum-utils/aimini"
)

var ErrNoTokensConfigured = errors.New("aimini tokens are not configured")

type TokenSource interface {
	ValuesByProvider(ctx context.Context, provider string) ([]string, error)
}

type Service struct {
	source       TokenSource
	metrics      *db.MetricsRepository
	baseURL      string
	nodeID       string
	timeout      time.Duration
	pollInterval time.Duration
	ttl          time.Duration

	mu       sync.RWMutex
	cachedAt time.Time
	client   *aimini.Client
	http     *http.Client
}

func NewService(source TokenSource, metrics *db.MetricsRepository, baseURL, nodeID string, timeout, pollInterval, ttl time.Duration, proxyURL string) (*Service, error) {
	if source == nil {
		return nil, errors.New("aimini token source is nil")
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("aimini base url is empty")
	}
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, errors.New("aimini node id is empty")
	}
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	httpClient, err := sharedhttp.New(timeout, proxyURL)
	if err != nil {
		return nil, err
	}
	return &Service{
		source:       source,
		metrics:      metrics,
		baseURL:      baseURL,
		nodeID:       nodeID,
		timeout:      timeout,
		pollInterval: pollInterval,
		ttl:          ttl,
		http:         httpClient,
	}, nil
}

func (s *Service) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedAt = time.Time{}
	s.client = nil
}

func (s *Service) GenerateImageForUser(ctx context.Context, userID string, photo []byte, mimeType string, prompt string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("aimini service is nil")
	}
	if len(photo) == 0 {
		return nil, errors.New("photo is empty")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("aimini user id is empty")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("prompt is empty")
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/jpeg"
	}

	client, err := s.clientFor(ctx)
	if err != nil {
		return nil, err
	}

	generationCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	resp, err := client.Models.GenerateContent(
		generationCtx,
		"",
		[]*aimini.Content{aimini.NewContent(
			aimini.Image(aimini.ImageFromBytes(photo, imageFilename(mimeType), mimeType)),
			aimini.Text(prompt),
		)},
		&aimini.GenerateContentConfig{
			UserID:       userID,
			NodeID:       s.nodeID,
			PollInterval: s.pollInterval,
		},
	)
	if err != nil {
		return nil, err
	}
	outputURL := ""
	if resp != nil {
		outputURL = strings.TrimSpace(resp.OutputURL)
	}
	if outputURL == "" {
		return nil, errors.New("aimini returned empty output url")
	}
	image, err := s.downloadImage(generationCtx, outputURL)
	if err != nil {
		return nil, err
	}
	if s.metrics != nil {
		_ = s.metrics.Record(ctx, db.MetricAiminiGeneration, "", 0, 1)
	}
	return image, nil
}

func (s *Service) clientFor(ctx context.Context) (*aimini.Client, error) {
	s.mu.RLock()
	if s.client != nil && time.Since(s.cachedAt) < s.ttl {
		client := s.client
		s.mu.RUnlock()
		return client, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil && time.Since(s.cachedAt) < s.ttl {
		return s.client, nil
	}

	tokens, err := s.source.ValuesByProvider(ctx, db.IntegrationProviderAimini)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		s.cachedAt = time.Now()
		s.client = nil
		return nil, ErrNoTokensConfigured
	}

	client, err := aimini.NewClient(ctx, &aimini.ClientConfig{
		BaseURL:      s.baseURL,
		Token:        strings.TrimSpace(tokens[0]),
		NodeID:       s.nodeID,
		HTTPClient:   s.http,
		PollInterval: s.pollInterval,
	})
	if err != nil {
		return nil, err
	}
	s.cachedAt = time.Now()
	s.client = client
	return client, nil
}

func (s *Service) downloadImage(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("aimini output image status: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func imageFilename(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return "input.png"
	case "image/webp":
		return "input.webp"
	case "image/gif":
		return "input.gif"
	case "image/heic":
		return "input.heic"
	case "image/heif":
		return "input.heif"
	default:
		return "input.jpg"
	}
}
