package deepseek

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientCompleteSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPost; got != want {
			t.Fatalf("method = %s, want %s", got, want)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"answer"}}]}`))
	}))
	defer srv.Close()

	client, err := NewClient("test-key", "", srv.URL, time.Second, "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.Complete(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != "answer" {
		t.Fatalf("Complete() = %q, want %q", got, "answer")
	}
}

func TestClientRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":{"message":"rate limit exceeded"}}`, http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client, err := NewClient("test-key", "", srv.URL, time.Second, "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Complete(context.Background(), "system", "user")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Complete() error = %v, want ErrRateLimited", err)
	}
}

func TestNewClientDefaults(t *testing.T) {
	client, err := NewClient("test-key", "", "", time.Second, "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client.model != defaultModel {
		t.Fatalf("model = %q, want %q", client.model, defaultModel)
	}
	if !strings.HasPrefix(client.baseURL, "https://") {
		t.Fatalf("baseURL = %q, want https://...", client.baseURL)
	}
}

func TestNewClientProxy(t *testing.T) {
	client, err := NewClient("test-key", "", "", time.Second, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.httpClient.Transport)
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
