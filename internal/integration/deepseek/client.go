package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sharedhttp "github.com/elum-bots/core/internal/integration/httpclient"
)

const defaultBaseURL = "https://api.deepseek.com"
const defaultModel = "deepseek-chat"

var ErrRateLimited = errors.New("deepseek rate limited")

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewClient(apiKey, model, baseURL string, timeout time.Duration, proxyURL string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("deepseek api key is empty")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultModel
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	httpClient, err := sharedhttp.New(timeout, proxyURL)
	if err != nil {
		return nil, err
	}

	return &Client{
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(apiKey),
		model:      strings.TrimSpace(model),
		httpClient: httpClient,
	}, nil
}

func (c *Client) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.ChatCompletion(ctx, []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, 0.8)
}

func (c *Client) ChatCompletion(ctx context.Context, messages []Message, temperature float64) (string, error) {
	if c == nil {
		return "", errors.New("deepseek client is nil")
	}
	if len(messages) == 0 {
		return "", errors.New("deepseek messages are empty")
	}

	reqBody := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
	}
	if strings.TrimSpace(reqBody.Model) == "" {
		reqBody.Model = defaultModel
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var parsed chatResponse
		if json.Unmarshal(body, &parsed) == nil && parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			if isRateLimitedMessage(parsed.Error.Message) {
				return "", ErrRateLimited
			}
			return "", fmt.Errorf("deepseek api error: %s", parsed.Error.Message)
		}
		return "", fmt.Errorf("deepseek api status: %d", resp.StatusCode)
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		if isRateLimitedMessage(parsed.Error.Message) {
			return "", ErrRateLimited
		}
		return "", fmt.Errorf("deepseek api error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("deepseek api returned empty choices")
	}

	out := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if out == "" {
		return "", errors.New("deepseek api returned empty content")
	}
	return out, nil
}

func isRateLimitedMessage(in string) bool {
	s := strings.ToLower(strings.TrimSpace(in))
	return strings.Contains(s, "rate limit") ||
		strings.Contains(s, "too many requests") ||
		strings.Contains(s, "429")
}
