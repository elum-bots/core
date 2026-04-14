package gemini

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/genai"
)

func TestExtractImageBytesFromInlineData(t *testing.T) {
	g := &Generator{httpClient: &http.Client{}}
	want := []byte{1, 2, 3}
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "ok"},
						{InlineData: &genai.Blob{MIMEType: "image/png", Data: want}},
					},
				},
			},
		},
	}

	got, err := g.extractImageBytes(context.Background(), resp)
	if err != nil {
		t.Fatalf("extractImageBytes error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("unexpected bytes: got=%v want=%v", got, want)
	}
}

func TestExtractImageBytesFromFileData(t *testing.T) {
	want := []byte("image-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	g := &Generator{httpClient: srv.Client()}
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{FileData: &genai.FileData{MIMEType: "image/jpeg", FileURI: srv.URL}},
					},
				},
			},
		},
	}

	got, err := g.extractImageBytes(context.Background(), resp)
	if err != nil {
		t.Fatalf("extractImageBytes error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("unexpected bytes: got=%q want=%q", string(got), string(want))
	}
}

func TestParseAPIKeys(t *testing.T) {
	keys := parseAPIKeys(" k1, k2 ,k1,, ")
	if len(keys) != 2 || keys[0] != "k1" || keys[1] != "k2" {
		t.Fatalf("unexpected parsed keys: %+v", keys)
	}
}

func TestIsRateLimitedErr(t *testing.T) {
	if !isRateLimitedErr(errors.New("Error 429, Message: Too Many Requests")) {
		t.Fatal("expected 429 error to be rate limited")
	}
	if !isRateLimitedErr(errors.New("RESOURCE_EXHAUSTED")) {
		t.Fatal("expected RESOURCE_EXHAUSTED to be rate limited")
	}
	if isRateLimitedErr(errors.New("some other error")) {
		t.Fatal("did not expect generic error to be rate limited")
	}
}

func TestShouldRetryImageOnly(t *testing.T) {
	if !shouldRetryImageOnly(&genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "Here is your image!"},
					},
				},
			},
		},
	}) {
		t.Fatal("expected text-only response to trigger image-only retry")
	}

	if !shouldRetryImageOnly(&genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{FinishReason: genai.FinishReasonNoImage},
		},
	}) {
		t.Fatal("expected no-image finish reason to trigger image-only retry")
	}

	if shouldRetryImageOnly(&genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte{1}}},
					},
				},
			},
		},
	}) {
		t.Fatal("did not expect image response to trigger image-only retry")
	}
}

func TestNewGeneratorProxy(t *testing.T) {
	g, err := NewGenerator(context.Background(), "test-key", "", time.Second, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	transport, ok := g.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", g.httpClient.Transport)
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("transport.Proxy() error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:8080" {
		t.Fatalf("proxyURL = %v, want %q", proxyURL, "http://127.0.0.1:8080")
	}
}
