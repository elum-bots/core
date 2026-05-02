package aimini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elum-bots/core/internal/db"
	"github.com/elum-utils/aimini"
)

type fakeTokenSource struct {
	tokens []string
}

func (f fakeTokenSource) ValuesByProvider(_ context.Context, provider string) ([]string, error) {
	if provider != db.IntegrationProviderAimini {
		return nil, errors.New("unexpected provider")
	}
	return append([]string(nil), f.tokens...), nil
}

func TestGenerateImageForUserQueuesPollsAndDownloadsOutput(t *testing.T) {
	t.Parallel()

	wantImage := []byte("generated-image")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/queue.add":
			if got := r.Header.Get("Authorization"); got != "secret-token" {
				t.Fatalf("Authorization = %q", got)
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm: %v", err)
			}
			if got := r.FormValue("user_id"); got != "user-1" {
				t.Fatalf("user_id = %q", got)
			}
			if got := r.FormValue("node_id"); got != "node-1" {
				t.Fatalf("node_id = %q", got)
			}
			if got := r.FormValue("prompts"); got != `["make portrait"]` {
				t.Fatalf("prompts = %q", got)
			}
			file, header, err := r.FormFile("image")
			if err != nil {
				t.Fatalf("FormFile: %v", err)
			}
			defer file.Close()
			if header.Filename != "input.png" {
				t.Fatalf("filename = %q", header.Filename)
			}
			raw, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if string(raw) != "source-image" {
				t.Fatalf("image = %q", raw)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"item-1","status":"queued"}`))
		case "/queue.list":
			var req aimini.ListQueueItemsRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if req.NodeID != "node-1" || req.Status != aimini.StatusProcessed {
				t.Fatalf("list request = %+v", req)
			}
			_, _ = w.Write([]byte(`[{"id":"item-1","status":"processed","output_s3_url":"` + serverURL(r) + `/result.png"}]`))
		case "/result.png":
			_, _ = w.Write(wantImage)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	service, err := NewService(
		fakeTokenSource{tokens: []string{"secret-token"}},
		nil,
		server.URL,
		"node-1",
		time.Second,
		time.Millisecond,
		time.Minute,
		"",
	)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	got, err := service.GenerateImageForUser(context.Background(), "user-1", []byte("source-image"), "image/png", "make portrait")
	if err != nil {
		t.Fatalf("GenerateImageForUser() error: %v", err)
	}
	if string(got) != string(wantImage) {
		t.Fatalf("image = %q, want %q", got, wantImage)
	}
}

func TestGenerateImageReturnsNoTokensError(t *testing.T) {
	t.Parallel()

	service, err := NewService(fakeTokenSource{}, nil, "https://example.test", "node-1", time.Second, time.Millisecond, time.Minute, "")
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	_, err = service.GenerateImageForUser(context.Background(), "user-1", []byte("source-image"), "image/jpeg", "prompt")
	if !errors.Is(err, ErrNoTokensConfigured) {
		t.Fatalf("error = %v, want ErrNoTokensConfigured", err)
	}
}

func serverURL(r *http.Request) string {
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}
