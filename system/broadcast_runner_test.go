package system

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
)

type testAdapter struct {
	mu sync.Mutex

	userBlock   chan struct{}
	userStarted chan string
	sends       []testSend
}

type testSend struct {
	chatID string
	text   string
}

func (a *testAdapter) Name() string                                    { return "test" }
func (a *testAdapter) Start(context.Context, elumbot.Dispatcher) error { return nil }
func (a *testAdapter) Stop(context.Context) error                      { return nil }

func (a *testAdapter) Send(ctx context.Context, chatID string, msg elumbot.OutMessage) error {
	if a.userBlock != nil && chatID != "admin-chat" {
		select {
		case a.userStarted <- chatID:
		default:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.userBlock:
		}
	}

	a.mu.Lock()
	a.sends = append(a.sends, testSend{chatID: chatID, text: msg.Text})
	a.mu.Unlock()
	return nil
}

func (a *testAdapter) countSends(chatID string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := 0
	for _, item := range a.sends {
		if item.chatID == chatID {
			count++
		}
	}
	return count
}

func TestBroadcastRunnerResumePost(t *testing.T) {
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	for _, userID := range []string{"u1", "u2"} {
		if _, err := store.Users.Ensure(context.Background(), userID); err != nil {
			t.Fatalf("Ensure(%s) error = %v", userID, err)
		}
	}
	post, err := store.Posts.Create(context.Background(), "Smoke", "Smoke text", "", "", nil, "admin")
	if err != nil {
		t.Fatalf("Posts.Create() error = %v", err)
	}

	payload, err := json.Marshal(postBroadcastPayload{PostID: post.ID})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	stat, err := store.Broadcasts.Create(context.Background(), broadcastTypePost, "admin-chat", string(payload), []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("Broadcasts.Create() error = %v", err)
	}

	adapter := &testAdapter{}
	bot := elumbot.New(adapter)
	runner := NewBroadcastRunner(context.Background(), bot, store)
	if err := runner.Resume(context.Background()); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	waitForBroadcastStatus(t, store, stat.ID, broadcastStatusCompleted)
	if got, want := adapter.countSends("u1"), 1; got != want {
		t.Fatalf("u1 sends = %d, want %d", got, want)
	}
	if got, want := adapter.countSends("u2"), 1; got != want {
		t.Fatalf("u2 sends = %d, want %d", got, want)
	}
	if got, want := adapter.countSends("admin-chat"), 1; got != want {
		t.Fatalf("admin summary sends = %d, want %d", got, want)
	}
}

func TestBroadcastRunnerStopPost(t *testing.T) {
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	for _, userID := range []string{"u1", "u2", "u3"} {
		if _, err := store.Users.Ensure(context.Background(), userID); err != nil {
			t.Fatalf("Ensure(%s) error = %v", userID, err)
		}
	}
	post, err := store.Posts.Create(context.Background(), "Stop", "Stop text", "", "", nil, "admin")
	if err != nil {
		t.Fatalf("Posts.Create() error = %v", err)
	}

	adapter := &testAdapter{
		userBlock:   make(chan struct{}),
		userStarted: make(chan string, 1),
	}
	bot := elumbot.New(adapter)
	runner := NewBroadcastRunner(context.Background(), bot, store)

	stat, err := runner.StartPost(context.Background(), "admin-chat", post.ID, []string{"u1", "u2", "u3"})
	if err != nil {
		t.Fatalf("StartPost() error = %v", err)
	}

	select {
	case <-adapter.userStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first user send")
	}

	item, requested, err := runner.RequestStop(context.Background(), stat.ID)
	if err != nil {
		t.Fatalf("RequestStop() error = %v", err)
	}
	if !requested {
		t.Fatalf("RequestStop() = false, want true")
	}
	if got, want := item.Status, broadcastStatusCancelRequested; got != want {
		t.Fatalf("status after request = %q, want %q", got, want)
	}

	waitForBroadcastStatus(t, store, stat.ID, broadcastStatusCanceled)

	if got, want := adapter.countSends("admin-chat"), 1; got != want {
		t.Fatalf("admin summary sends = %d, want %d", got, want)
	}
	if got := adapter.countSends("u1") + adapter.countSends("u2") + adapter.countSends("u3"); got != 0 {
		t.Fatalf("user sends = %d, want 0 after cancellation", got)
	}
}

func waitForBroadcastStatus(t *testing.T, store *db.Store, id int64, want string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		item, err := store.Broadcasts.Get(context.Background(), id)
		if err == nil && item.Status == want && !item.Active {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	item, err := store.Broadcasts.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Broadcasts.Get() final error = %v", err)
	}
	t.Fatalf("broadcast status = %q active=%t, want status=%q active=false", item.Status, item.Active, want)
}
