package tg

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elum-bots/core/internal/bot"
)

type disp struct {
	mu  sync.Mutex
	got []bot.Update
	err error
}

func (d *disp) Dispatch(_ context.Context, u bot.Update) error {
	d.mu.Lock()
	d.got = append(d.got, u)
	d.mu.Unlock()
	return d.err
}

func (d *disp) len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.got)
}

func (d *disp) at(i int) bot.Update {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.got[i]
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResp(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestAdapterNameAndEmptyToken(t *testing.T) {
	a := NewAdapter("")
	if a.Name() != "tg" {
		t.Fatalf("unexpected adapter name")
	}
	if err := a.Start(context.Background(), &disp{}); !errors.Is(err, bot.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if err := a.Send(context.Background(), "1", bot.OutMessage{Text: "x"}); !errors.Is(err, bot.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestAdapterStartAndSend(t *testing.T) {
	var getCalls atomic.Int32
	var sentBody string

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		switch {
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			sentBody = body
			return jsonResp(`{"ok":true,"result":{}}`), nil
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			c := getCalls.Add(1)
			if c == 1 {
				return jsonResp(`{"ok":true,"result":[{"update_id":1,"message":{"message_id":11,"from":{"id":42},"chat":{"id":100,"type":"private"},"text":"/stats one"}},{"update_id":2,"callback_query":{"id":"x","from":{"id":42},"data":"a","message":{"message_id":12,"chat":{"id":100,"type":"private"}}}}]}`), nil
			}
			return jsonResp(`{"ok":true,"result":[]}`), nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
	})}

	a := NewAdapter("T", WithBaseURL("https://example.test"), WithPolling(1, 10), WithHTTPClient(client))
	if err := a.Send(context.Background(), "100", bot.OutMessage{Text: "hello", Buttons: []bot.ButtonRow{bot.Row(bot.Btn("a", "A"))}}); err != nil {
		t.Fatalf("send err: %v", err)
	}
	if !strings.Contains(sentBody, `"text":"hello"`) || !strings.Contains(sentBody, `"callback_data":"a"`) {
		t.Fatalf("unexpected send body: %s", sentBody)
	}

	d := &disp{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			if d.len() >= 2 {
				cancel()
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	err := a.Start(ctx, d)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
	if d.len() != 2 {
		t.Fatalf("expected 2 updates, got %d", d.len())
	}
	first := d.at(0)
	second := d.at(1)
	if first.Message.Text != "/stats one" && second.Message.Text == "/stats one" {
		first, second = second, first
	}
	if first.Platform != "tg" || first.Message.Text != "/stats one" {
		t.Fatalf("message mapping mismatch: %#v", first)
	}
	if second.ButtonID != "a" {
		t.Fatalf("callback mapping mismatch: %#v", second)
	}
}

func TestSendEscapesAngleArgsWhenUsingHTML(t *testing.T) {
	var payload sendMessageRequest

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/sendMessage") {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return jsonResp(`{"ok":true,"result":{}}`), nil
	})}

	adapter := NewAdapter("T", WithBaseURL("https://example.test"), WithHTTPClient(client))
	err := adapter.Send(context.Background(), "123", bot.OutMessage{
		Text: "Команды:\n/post_preview <id>\n<a href=\"https://example.com\">читать</a>",
	})
	if err != nil {
		t.Fatalf("send err: %v", err)
	}
	if payload.ParseMode != "HTML" {
		t.Fatalf("expected HTML parse mode, got %q", payload.ParseMode)
	}
	if !strings.Contains(payload.Text, "&lt;id&gt;") {
		t.Fatalf("expected escaped placeholder, got %q", payload.Text)
	}
	if !strings.Contains(payload.Text, "<a href=\"https://example.com\">читать</a>") {
		t.Fatalf("expected anchor tag to stay intact, got %q", payload.Text)
	}
}

func TestAdapterDispatchError(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			return jsonResp(`{"ok":true,"result":[{"update_id":1,"message":{"message_id":11,"from":{"id":42},"chat":{"id":100,"type":"private"},"text":"x"}}]}`), nil
		}
		return jsonResp(`{"ok":true,"result":[]}`), nil
	})}

	a := NewAdapter("T", WithBaseURL("https://example.test"), WithPolling(1, 1), WithHTTPClient(client))
	d := &disp{err: errors.New("boom")}
	err := a.Start(context.Background(), d)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestNormalizeUpdateKinds(t *testing.T) {
	u, ok := normalizeUpdate(tgUpdate{Message: &tgMessage{MessageID: 1, Chat: tgChat{ID: 2, Type: "private"}, From: &tgUser{ID: 3}, Photo: []tgPhoto{{FileID: "p", FileSize: 5}}, Caption: "cap"}})
	if !ok {
		t.Fatalf("expected private photo update to be accepted")
	}
	if u.Message.Kind != bot.MessageKindPhoto || len(u.Message.Attachments) != 1 {
		t.Fatalf("photo mapping failed: %#v", u)
	}

	u, ok = normalizeUpdate(tgUpdate{Message: &tgMessage{MessageID: 1, Chat: tgChat{ID: 2, Type: "private"}, From: &tgUser{ID: 3}, Document: &tgDocument{FileID: "d", FileName: "f.txt", MimeType: "text/plain", FileSize: 7}}})
	if !ok {
		t.Fatalf("expected private document update to be accepted")
	}
	if u.Message.Kind != bot.MessageKindFile || u.Message.Attachments[0].Name != "f.txt" {
		t.Fatalf("doc mapping failed: %#v", u)
	}

	u, ok = normalizeUpdate(tgUpdate{Message: &tgMessage{MessageID: 1, Chat: tgChat{ID: 2, Type: "private"}, From: &tgUser{ID: 3}, Voice: &tgFile{FileID: "v", FileSize: 1}}})
	if !ok {
		t.Fatalf("expected private voice update to be accepted")
	}
	if u.Message.Kind != bot.MessageKindVoice {
		t.Fatalf("voice mapping failed: %#v", u)
	}
}

func TestNormalizeUpdateSkipsNonPrivateChats(t *testing.T) {
	if _, ok := normalizeUpdate(tgUpdate{
		Message: &tgMessage{
			MessageID: 1,
			Chat:      tgChat{ID: -100, Type: "supergroup"},
			From:      &tgUser{ID: 3},
			Text:      "hello",
		},
	}); ok {
		t.Fatalf("expected supergroup message to be skipped")
	}

	if _, ok := normalizeUpdate(tgUpdate{
		CallbackQuery: &tgCallbackQuery{
			ID:   "cb",
			From: &tgUser{ID: 3},
			Data: "click",
			Message: &tgMessage{
				MessageID: 2,
				Chat:      tgChat{ID: -100, Type: "channel"},
			},
		},
	}); ok {
		t.Fatalf("expected channel callback to be skipped")
	}
}

func TestSendMedia(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "image.png")
	vid := filepath.Join(dir, "video.mp4")
	aud := filepath.Join(dir, "audio.ogg")
	if err := os.WriteFile(img, []byte("img"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vid, []byte("vid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aud, []byte("aud"), 0o600); err != nil {
		t.Fatal(err)
	}

	var calls []string
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Path)
		return jsonResp(`{"ok":true,"result":{}}`), nil
	})}
	adapter := NewAdapter("T", WithBaseURL("https://example.test"), WithHTTPClient(client))
	err := adapter.Send(context.Background(), "123", bot.OutMessage{
		Text: "media",
		Media: []bot.OutMedia{
			bot.Image(img),
			bot.Video(vid),
			bot.Audio(aud),
		},
	})
	if err != nil {
		t.Fatalf("send media err: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 media call, got %d", len(calls))
	}
	if !strings.HasSuffix(calls[0], "/sendMediaGroup") {
		t.Fatalf("unexpected media endpoints: %#v", calls)
	}
}

func TestSendMediaGroupWithButtons(t *testing.T) {
	dir := t.TempDir()
	img1 := filepath.Join(dir, "image1.png")
	img2 := filepath.Join(dir, "image2.png")
	if err := os.WriteFile(img1, []byte("img1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(img2, []byte("img2"), 0o600); err != nil {
		t.Fatal(err)
	}

	var calls []string
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Path)
		return jsonResp(`{"ok":true,"result":{}}`), nil
	})}

	adapter := NewAdapter("T", WithBaseURL("https://example.test"), WithHTTPClient(client))
	err := adapter.Send(context.Background(), "123", bot.OutMessage{
		Text: "Альбом",
		Media: []bot.OutMedia{
			bot.Image(img1),
			bot.Image(img2),
		},
		Buttons: []bot.ButtonRow{bot.Row(bot.Btn("a", "A"))},
	})
	if err != nil {
		t.Fatalf("send media group with buttons err: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if !strings.HasSuffix(calls[0], "/sendMediaGroup") || !strings.HasSuffix(calls[1], "/sendMessage") {
		t.Fatalf("unexpected call order: %#v", calls)
	}
}
