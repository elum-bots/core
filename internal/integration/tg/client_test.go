package tg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientGetMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/bottest-token/getMe"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"username":"demo_bot","first_name":"Demo"}}`))
	}))
	defer srv.Close()

	client, err := NewClient("test-token", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	me, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v", err)
	}
	if got, want := me.Username, "demo_bot"; got != want {
		t.Fatalf("username = %q, want %q", got, want)
	}
}

func TestClientDeleteWebhook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/bottest-token/deleteWebhook"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	client, err := NewClient("test-token", WithBaseURL(srv.URL), WithHTTPClient(&http.Client{Timeout: time.Second}))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.DeleteWebhook(context.Background(), true); err != nil {
		t.Fatalf("DeleteWebhook() error = %v", err)
	}
}

func TestClientGetChatMember(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/bottest-token/getChatMember"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error = %v", err)
		}
		if got, want := payload["chat_id"], "-100123"; got != want {
			t.Fatalf("chat_id = %#v, want %q", got, want)
		}
		if got, want := int64(payload["user_id"].(float64)), int64(42); got != want {
			t.Fatalf("user_id = %d, want %d", got, want)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"member"}}`))
	}))
	defer srv.Close()

	client, err := NewClient("test-token", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	member, err := client.GetChatMember(context.Background(), "-100123", "42")
	if err != nil {
		t.Fatalf("GetChatMember() error = %v", err)
	}
	if !member.IsMember() {
		t.Fatalf("IsMember() = false, want true")
	}
}

func TestClientDownloadFileByID(t *testing.T) {
	imageBytes := []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest-token/getFile":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"file-1","file_path":"photos/demo.png","file_size":16}}`))
		case "/file/bottest-token/photos/demo.png":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(imageBytes)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewClient("test-token", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, mimeType, err := client.DownloadFileByID(context.Background(), "file-1")
	if err != nil {
		t.Fatalf("DownloadFileByID() error = %v", err)
	}
	if string(got) != string(imageBytes) {
		t.Fatalf("downloaded bytes mismatch")
	}
	if got, want := mimeType, "image/png"; got != want {
		t.Fatalf("mimeType = %q, want %q", got, want)
	}
}
