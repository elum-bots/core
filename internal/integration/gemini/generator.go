package gemini

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/genai"
)

const defaultModel = "gemini-2.5-flash-image"

type Generator struct {
	clients    []*genai.Client
	httpClient *http.Client
	model      string
	timeout    time.Duration
	nextClient atomic.Uint64
}

func NewGenerator(ctx context.Context, apiKey string, model string, timeout time.Duration) (*Generator, error) {
	keys := parseAPIKeys(apiKey)
	if len(keys) == 0 {
		return nil, errors.New("gemini api key is empty")
	}
	if strings.TrimSpace(model) == "" {
		model = defaultModel
	}
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}
	clients := make([]*genai.Client, 0, len(keys))
	for _, key := range keys {
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  key,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}
	return &Generator{
		clients:    clients,
		httpClient: &http.Client{},
		model:      model,
		timeout:    timeout,
	}, nil
}

func (g *Generator) GenerateImage(ctx context.Context, photo []byte, mimeType string, prompt string) ([]byte, error) {
	generationCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	if len(g.clients) == 0 {
		return nil, errors.New("gemini clients are not configured")
	}
	if len(photo) == 0 {
		return nil, errors.New("photo is empty")
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/jpeg"
	}

	start := int(g.nextClient.Add(1)-1) % len(g.clients)
	allRateLimited := true
	for i := 0; i < len(g.clients); i++ {
		client := g.clients[(start+i)%len(g.clients)]
		img, err := g.generateWithClient(generationCtx, client, photo, mimeType, prompt)
		if err == nil {
			return img, nil
		}
		if isRateLimitedErr(err) {
			continue
		}
		allRateLimited = false
		return nil, err
	}
	if allRateLimited {
		return nil, ErrGenerationRateLimited
	}
	return nil, ErrGenerationRateLimited
}

func (g *Generator) generateWithClient(ctx context.Context, client *genai.Client, photo []byte, mimeType string, prompt string) ([]byte, error) {
	parts := []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{Data: photo, MIMEType: mimeType}},
	}
	contents := []*genai.Content{{Parts: parts, Role: genai.RoleUser}}
	cfg := &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE", "TEXT"},
	}

	resp, err := client.Models.GenerateContent(ctx, g.model, contents, cfg)
	if err != nil {
		if isRateLimitedErr(err) {
			return nil, err
		}
		if isLocationUnsupportedErr(err) {
			return nil, fmt.Errorf("%w: %v", ErrGenerationUnavailable, err)
		}
		return nil, err
	}
	// logGeminiResponse(resp)

	if imageBytes, imageErr := g.extractImageBytes(ctx, resp); imageErr == nil {
		return imageBytes, nil
	}

	// Some responses return text or IMAGE_* finish reasons without actual image parts.
	// Retry once with IMAGE-only modality and explicit instruction to return only image bytes.
	if shouldRetryImageOnly(resp) {
		retryParts := []*genai.Part{
			{Text: prompt + " Return only the final generated image. Do not return explanation, markdown, or any text."},
			{InlineData: &genai.Blob{Data: photo, MIMEType: mimeType}},
		}
		retryContents := []*genai.Content{{Parts: retryParts, Role: genai.RoleUser}}
		retryCfg := &genai.GenerateContentConfig{ResponseModalities: []string{"IMAGE"}}
		retryResp, retryErr := client.Models.GenerateContent(ctx, g.model, retryContents, retryCfg)
		if retryErr != nil && isRateLimitedErr(retryErr) {
			return nil, retryErr
		}
		if retryErr == nil {
			if imageBytes, imageErr := g.extractImageBytes(ctx, retryResp); imageErr == nil {
				return imageBytes, nil
			}
			resp = retryResp
		}
	}

	if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != "" {
		return nil, fmt.Errorf("%w: prompt blocked (%s)", ErrGenerationRejected, resp.PromptFeedback.BlockReason)
	}

	var reasons []string
	for i, c := range resp.Candidates {
		if c == nil {
			continue
		}
		if c.FinishReason != "" {
			reasons = append(reasons, fmt.Sprintf("candidate[%d].finish_reason=%s", i, c.FinishReason))
		}
		if c.FinishMessage != "" {
			reasons = append(reasons, fmt.Sprintf("candidate[%d].finish_message=%s", i, strings.TrimSpace(c.FinishMessage)))
		}
	}

	if txt := strings.TrimSpace(resp.Text()); txt != "" {
		return nil, fmt.Errorf("%w: gemini returned text without image: %s", ErrGenerationRejected, txt)
	}
	if len(reasons) > 0 {
		return nil, fmt.Errorf("%w: gemini did not return image data (%s)", ErrGenerationRejected, strings.Join(reasons, "; "))
	}
	return nil, fmt.Errorf("%w: gemini did not return image data", ErrGenerationRejected)
}

func parseAPIKeys(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func isRateLimitedErr(err error) bool {
	if err == nil {
		return false
	}
	var rl RateLimitError
	if errors.As(err, &rl) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "429") ||
		strings.Contains(s, "too many requests") ||
		strings.Contains(s, "resource_exhausted") ||
		strings.Contains(s, "rate limit")
}

func isLocationUnsupportedErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "user location is not supported for the api use") ||
		(strings.Contains(s, "failed_precondition") && strings.Contains(s, "location"))
}

func shouldRetryImageOnly(resp *genai.GenerateContentResponse) bool {
	if resp == nil {
		return false
	}
	if strings.TrimSpace(resp.Text()) != "" {
		return true
	}
	for _, c := range resp.Candidates {
		if c == nil {
			continue
		}
		switch c.FinishReason {
		case genai.FinishReasonImageOther, genai.FinishReasonNoImage:
			return true
		}
	}
	return false
}

func (g *Generator) extractImageBytes(ctx context.Context, resp *genai.GenerateContentResponse) ([]byte, error) {
	for _, c := range resp.Candidates {
		if c == nil || c.Content == nil {
			continue
		}
		for _, p := range c.Content.Parts {
			if p == nil {
				continue
			}
			if p.InlineData != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.InlineData.MIMEType)), "image/") && len(p.InlineData.Data) > 0 {
				return p.InlineData.Data, nil
			}
			if p.FileData != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.FileData.MIMEType)), "image/") {
				img, err := g.downloadImageFromFileURI(ctx, p.FileData.FileURI)
				if err == nil && len(img) > 0 {
					return img, nil
				}
			}
		}
	}
	return nil, errors.New("image not found in candidates")
}

func (g *Generator) downloadImageFromFileURI(ctx context.Context, fileURI string) ([]byte, error) {
	fileURI = strings.TrimSpace(fileURI)
	if fileURI == "" {
		return nil, errors.New("empty file uri")
	}
	u, err := url.Parse(fileURI)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURI, nil)
		if err != nil {
			return nil, err
		}
		resp, err := g.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("bad status %d for %s", resp.StatusCode, fileURI)
		}
		return io.ReadAll(resp.Body)
	default:
		return nil, fmt.Errorf("unsupported file uri scheme: %s", u.Scheme)
	}
}
