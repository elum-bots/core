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

func TestQueueImageForUserAddsQueueItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queue.add" {
			t.Fatalf("path = %q", r.URL.Path)
		}
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
	}))
	t.Cleanup(server.Close)

	service := newTestService(t, server.URL, []string{"secret-token"})

	id, err := service.QueueImageForUser(context.Background(), "user-1", []byte("source-image"), "image/png", "make portrait")
	if err != nil {
		t.Fatalf("QueueImageForUser() error: %v", err)
	}
	if id != "item-1" {
		t.Fatalf("id = %q, want item-1", id)
	}
}

func TestListProcessedImages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queue.list" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var req aimini.ListQueueItemsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if req.NodeID != "node-1" || req.Status != aimini.StatusProcessed || req.Limit != 7 {
			t.Fatalf("list request = %+v", req)
		}
		_, _ = w.Write([]byte(`[{"id":"item-1","user_id":"user-1","node_id":"node-1","status":"processed","output_s3_url":"https://cdn/result.png"}]`))
	}))
	t.Cleanup(server.Close)

	service := newTestService(t, server.URL, []string{"secret-token"})

	items, err := service.ListProcessedImages(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListProcessedImages() error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if got := items[0]; got.ID != "item-1" || got.UserID != "user-1" || got.NodeID != "node-1" || got.OutputURL != "https://cdn/result.png" || got.Status != "processed" {
		t.Fatalf("item = %+v", got)
	}
}

func TestDownloadImage(t *testing.T) {
	t.Parallel()

	wantImage := []byte("generated-image")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/result.png" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write(wantImage)
	}))
	t.Cleanup(server.Close)

	service := newTestService(t, server.URL, []string{"secret-token"})

	got, err := service.DownloadImage(context.Background(), server.URL+"/result.png")
	if err != nil {
		t.Fatalf("DownloadImage() error: %v", err)
	}
	if string(got) != string(wantImage) {
		t.Fatalf("image = %q, want %q", got, wantImage)
	}
}

func TestDeleteQueueItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queue.delete" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var req aimini.DeleteQueueItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if req.ID != "item-1" {
			t.Fatalf("id = %q", req.ID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	service := newTestService(t, server.URL, []string{"secret-token"})

	if err := service.DeleteQueueItem(context.Background(), "item-1"); err != nil {
		t.Fatalf("DeleteQueueItem() error: %v", err)
	}
}

func TestQueueImageForUserReturnsNoTokensError(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "https://example.test", nil)

	_, err := service.QueueImageForUser(context.Background(), "user-1", []byte("source-image"), "image/jpeg", "prompt")
	if !errors.Is(err, ErrNoTokensConfigured) {
		t.Fatalf("error = %v, want ErrNoTokensConfigured", err)
	}
}

func newTestService(t *testing.T, baseURL string, tokens []string) *Service {
	t.Helper()
	service, err := NewService(fakeTokenSource{tokens: tokens}, nil, baseURL, "node-1", time.Second, time.Minute, "")
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}
	return service
}
