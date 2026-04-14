package httpclient

import (
	"net/http"
	"testing"
	"time"
)

func TestNewWithoutProxy(t *testing.T) {
	client, err := New(5*time.Second, "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if client.Timeout != 5*time.Second {
		t.Fatalf("timeout = %v, want %v", client.Timeout, 5*time.Second)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("proxy func is nil")
	}
}

func TestNewWithProxy(t *testing.T) {
	client, err := New(time.Second, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("transport.Proxy() error = %v", err)
	}
	if proxyURL == nil {
		t.Fatal("proxyURL is nil")
	}
	if got, want := proxyURL.String(), "http://127.0.0.1:8080"; got != want {
		t.Fatalf("proxyURL = %q, want %q", got, want)
	}
}

func TestNewInvalidProxy(t *testing.T) {
	if _, err := New(time.Second, "127.0.0.1:8080"); err == nil {
		t.Fatal("New() error = nil, want error")
	}
}
