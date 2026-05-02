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
	source  TokenSource
	metrics *db.MetricsRepository
	baseURL string
	nodeID  string
	ttl     time.Duration

	mu       sync.RWMutex
	cachedAt time.Time
	client   *aimini.Client
	http     *http.Client
}

type QueuedImage struct {
	ID        string
	UserID    string
	NodeID    string
	OutputURL string
	Status    string
}

func NewService(source TokenSource, metrics *db.MetricsRepository, baseURL, nodeID string, timeout, ttl time.Duration, proxyURL string) (*Service, error) {
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
	if ttl <= 0 {
		ttl = time.Minute
	}
	httpClient, err := sharedhttp.New(timeout, proxyURL)
	if err != nil {
		return nil, err
	}
	return &Service{
		source:  source,
		metrics: metrics,
		baseURL: baseURL,
		nodeID:  nodeID,
		ttl:     ttl,
		http:    httpClient,
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

func (s *Service) QueueImageForUser(ctx context.Context, userID string, photo []byte, mimeType string, prompt string) (string, error) {
	if s == nil {
		return "", errors.New("aimini service is nil")
	}
	if len(photo) == 0 {
		return "", errors.New("photo is empty")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", errors.New("aimini user id is empty")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt is empty")
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/jpeg"
	}

	client, err := s.clientFor(ctx)
	if err != nil {
		return "", err
	}

	item, err := client.Queue.Add(ctx, &aimini.AddQueueItemRequest{
		Image:   aimini.ImageFromBytes(photo, imageFilename(mimeType), mimeType),
		UserID:  userID,
		NodeID:  s.nodeID,
		Prompts: []string{prompt},
	})
	if err != nil {
		return "", err
	}
	if item == nil || strings.TrimSpace(item.ID) == "" {
		return "", errors.New("aimini returned empty queue item id")
	}
	return item.ID, nil
}

func (s *Service) ListProcessedImages(ctx context.Context, limit int) ([]QueuedImage, error) {
	if s == nil {
		return nil, errors.New("aimini service is nil")
	}
	client, err := s.clientFor(ctx)
	if err != nil {
		return nil, err
	}

	items, err := client.Queue.List(ctx, &aimini.ListQueueItemsRequest{
		NodeID: s.nodeID,
		Status: aimini.StatusProcessed,
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]QueuedImage, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, QueuedImage{
			ID:        item.ID,
			UserID:    item.UserID,
			NodeID:    item.NodeID,
			OutputURL: item.OutputS3URL,
			Status:    item.Status,
		})
	}
	return out, nil
}

func (s *Service) DownloadImage(ctx context.Context, outputURL string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("aimini service is nil")
	}
	return s.downloadImage(ctx, outputURL)
}

func (s *Service) DeleteQueueItem(ctx context.Context, id string) error {
	if s == nil {
		return errors.New("aimini service is nil")
	}
	client, err := s.clientFor(ctx)
	if err != nil {
		return err
	}
	return client.Queue.Delete(ctx, &aimini.DeleteQueueItemRequest{ID: id})
}

func (s *Service) RecordGeneration(ctx context.Context) {
	if s != nil && s.metrics != nil {
		_ = s.metrics.Record(ctx, db.MetricAiminiGeneration, "", 0, 1)
	}
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
		BaseURL:    s.baseURL,
		Token:      strings.TrimSpace(tokens[0]),
		NodeID:     s.nodeID,
		HTTPClient: s.http,
	})
	if err != nil {
		return nil, err
	}
	s.cachedAt = time.Now()
	s.client = client
	return client, nil
}

func (s *Service) downloadImage(ctx context.Context, rawURL string) ([]byte, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("aimini output url is empty")
	}
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
