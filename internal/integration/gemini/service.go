package gemini

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/elum-bots/core/internal/db"
)

var ErrNoTokensConfigured = errors.New("gemini tokens are not configured")

type TokenSource interface {
	ValuesByProvider(ctx context.Context, provider string) ([]string, error)
}

type Service struct {
	source   TokenSource
	model    string
	proxyURL string
	timeout  time.Duration
	ttl      time.Duration

	mu        sync.RWMutex
	cachedAt  time.Time
	generator *Generator
}

func NewService(ctx context.Context, source TokenSource, model string, timeout, ttl time.Duration, proxyURL string) (*Service, error) {
	if source == nil {
		return nil, errors.New("gemini token source is nil")
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &Service{
		source:   source,
		model:    strings.TrimSpace(model),
		proxyURL: strings.TrimSpace(proxyURL),
		timeout:  timeout,
		ttl:      ttl,
	}, nil
}

func (s *Service) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedAt = time.Time{}
	s.generator = nil
}

func (s *Service) GenerateImage(ctx context.Context, photo []byte, mimeType string, prompt string) ([]byte, error) {
	generator, err := s.generatorFor(ctx)
	if err != nil {
		return nil, err
	}
	return generator.GenerateImage(ctx, photo, mimeType, prompt)
}

func (s *Service) generatorFor(ctx context.Context) (*Generator, error) {
	if s == nil {
		return nil, errors.New("gemini service is nil")
	}

	s.mu.RLock()
	if s.generator != nil && time.Since(s.cachedAt) < s.ttl {
		generator := s.generator
		s.mu.RUnlock()
		return generator, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.generator != nil && time.Since(s.cachedAt) < s.ttl {
		return s.generator, nil
	}

	tokens, err := s.source.ValuesByProvider(ctx, db.IntegrationProviderGemini)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		s.cachedAt = time.Now()
		s.generator = nil
		return nil, ErrNoTokensConfigured
	}

	generator, err := NewGenerator(ctx, strings.Join(tokens, ","), s.model, s.timeout, s.proxyURL)
	if err != nil {
		return nil, err
	}
	s.cachedAt = time.Now()
	s.generator = generator
	return generator, nil
}
