package deepseek

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elum-bots/core/internal/db"
)

var ErrNoTokensConfigured = errors.New("deepseek tokens are not configured")

type TokenSource interface {
	ValuesByProvider(ctx context.Context, provider string) ([]string, error)
}

type Service struct {
	source  TokenSource
	model   string
	baseURL string
	timeout time.Duration
	ttl     time.Duration

	mu       sync.RWMutex
	cachedAt time.Time
	clients  []*Client

	next atomic.Uint64
}

func NewService(source TokenSource, model, baseURL string, timeout, ttl time.Duration) (*Service, error) {
	if source == nil {
		return nil, errors.New("deepseek token source is nil")
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &Service{
		source:  source,
		model:   strings.TrimSpace(model),
		baseURL: strings.TrimSpace(baseURL),
		timeout: timeout,
		ttl:     ttl,
	}, nil
}

func (s *Service) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedAt = time.Time{}
	s.clients = nil
}

func (s *Service) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return s.ChatCompletion(ctx, []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.8)
}

func (s *Service) ChatCompletion(ctx context.Context, messages []Message, temperature float64) (string, error) {
	clients, err := s.clientsFor(ctx)
	if err != nil {
		return "", err
	}

	start := int(s.next.Add(1)-1) % len(clients)
	allRateLimited := true
	for i := 0; i < len(clients); i++ {
		client := clients[(start+i)%len(clients)]
		out, callErr := client.ChatCompletion(ctx, messages, temperature)
		if callErr == nil {
			return out, nil
		}
		if errors.Is(callErr, ErrRateLimited) {
			continue
		}
		allRateLimited = false
		return "", callErr
	}
	if allRateLimited {
		return "", ErrRateLimited
	}
	return "", ErrRateLimited
}

func (s *Service) clientsFor(ctx context.Context) ([]*Client, error) {
	if s == nil {
		return nil, errors.New("deepseek service is nil")
	}

	s.mu.RLock()
	if len(s.clients) > 0 && time.Since(s.cachedAt) < s.ttl {
		out := append([]*Client(nil), s.clients...)
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.clients) > 0 && time.Since(s.cachedAt) < s.ttl {
		return append([]*Client(nil), s.clients...), nil
	}

	tokens, err := s.source.ValuesByProvider(ctx, db.IntegrationProviderDeepSeek)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		s.cachedAt = time.Now()
		s.clients = nil
		return nil, ErrNoTokensConfigured
	}

	clients := make([]*Client, 0, len(tokens))
	for _, token := range tokens {
		client, err := NewClient(token, s.model, s.baseURL, s.timeout)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}

	s.cachedAt = time.Now()
	s.clients = clients
	return append([]*Client(nil), clients...), nil
}
