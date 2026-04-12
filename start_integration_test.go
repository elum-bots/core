package bot

import (
	"context"
	"path/filepath"
	"testing"

	runtimebot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
	"github.com/elum-bots/core/system"
)

type captureAdapter struct {
	dispatch runtimebot.Dispatcher
	sent     []runtimebot.OutMessage
	chatIDs  []string
}

func (a *captureAdapter) Name() string { return "capture" }

func (a *captureAdapter) Start(_ context.Context, d runtimebot.Dispatcher) error {
	a.dispatch = d
	return nil
}

func (a *captureAdapter) Stop(context.Context) error { return nil }

func (a *captureAdapter) Send(_ context.Context, chatID string, msg runtimebot.OutMessage) error {
	a.chatIDs = append(a.chatIDs, chatID)
	a.sent = append(a.sent, msg)
	return nil
}

func TestCustomStartKeepsReferralAndRegistration(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "1")
	t.Setenv("REFERRAL_REWARD_PER_REF", "0.333334")

	ctx := context.Background()
	store := openTestStore(t, ctx)
	defer store.Close()

	if _, err := store.Users.Ensure(ctx, "100"); err != nil {
		t.Fatalf("ensure inviter: %v", err)
	}

	b, adapter := newCustomStartBot(store)
	if err := b.Dispatch(ctx, runtimebot.Update{
		Platform: "tg",
		ChatID:   "chat-1",
		UserID:   "200",
		Message:  runtimebot.Message{Text: "/start ref_100"},
	}); err != nil {
		t.Fatalf("dispatch start: %v", err)
	}

	if len(adapter.sent) != 1 || adapter.sent[0].Text != "custom start" {
		t.Fatalf("unexpected outbound messages: %#v", adapter.sent)
	}

	user, err := store.Users.Get(ctx, "200")
	if err != nil {
		t.Fatalf("get invited user: %v", err)
	}
	if user.UserID != "200" {
		t.Fatalf("unexpected invited user: %#v", user)
	}

	inviter, err := store.Users.Get(ctx, "100")
	if err != nil {
		t.Fatalf("get inviter: %v", err)
	}
	if inviter.ReferralCnt != 1 {
		t.Fatalf("expected referral_cnt=1, got %d", inviter.ReferralCnt)
	}

	starts, err := store.Metrics.CountTotal(ctx, db.MetricStart)
	if err != nil {
		t.Fatalf("count starts: %v", err)
	}
	if starts != 1 {
		t.Fatalf("expected 1 start metric, got %d", starts)
	}
}

func TestCustomStartKeepsTrackVisit(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "1")

	ctx := context.Background()
	store := openTestStore(t, ctx)
	defer store.Close()

	link, err := store.Track.Create(ctx, "promo123", "Promo", "100")
	if err != nil {
		t.Fatalf("create track link: %v", err)
	}

	b, adapter := newCustomStartBot(store)
	if err := b.Dispatch(ctx, runtimebot.Update{
		Platform: "tg",
		ChatID:   "chat-2",
		UserID:   "201",
		Message:  runtimebot.Message{Text: "/start trk_promo123"},
	}); err != nil {
		t.Fatalf("dispatch start: %v", err)
	}

	if len(adapter.sent) != 1 || adapter.sent[0].Text != "custom start" {
		t.Fatalf("unexpected outbound messages: %#v", adapter.sent)
	}

	updated, err := store.Track.Get(ctx, link.ID)
	if err != nil {
		t.Fatalf("get track link: %v", err)
	}
	if updated.ArrivalsCount != 1 {
		t.Fatalf("expected arrivals_count=1, got %d", updated.ArrivalsCount)
	}
}

func TestCoreStartWithoutBusinessHandlerSendsNothing(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "1")

	ctx := context.Background()
	store := openTestStore(t, ctx)
	defer store.Close()

	adapter := &captureAdapter{}
	b := runtimebot.New(adapter)
	system.Register(b, system.Dependencies{Store: store})

	if err := b.Dispatch(ctx, runtimebot.Update{
		Platform: "tg",
		ChatID:   "chat-3",
		UserID:   "202",
		Message:  runtimebot.Message{Text: "/start ref_100"},
	}); err != nil {
		t.Fatalf("dispatch start: %v", err)
	}

	if len(adapter.sent) != 0 {
		t.Fatalf("unexpected outbound messages: %#v", adapter.sent)
	}
	if _, err := store.Users.Get(ctx, "202"); err != nil {
		t.Fatalf("user must be registered: %v", err)
	}
}

func TestCustomStartCallsReferralSuccessHook(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "0")
	t.Setenv("REFERRAL_REWARD_PER_REF", "1")

	ctx := context.Background()
	store := openTestStore(t, ctx)
	defer store.Close()

	if _, err := store.Users.Ensure(ctx, "100"); err != nil {
		t.Fatalf("ensure inviter: %v", err)
	}

	adapter := &captureAdapter{}
	b := runtimebot.New(adapter)

	var (
		called bool
		event  system.ReferralSuccess
	)
	system.Register(b, system.Dependencies{
		Store: store,
		ReferralHooks: []system.ReferralSuccessHook{
			func(_ context.Context, _ *runtimebot.Bot, _ system.Dependencies, in system.ReferralSuccess) error {
				called = true
				event = in
				return nil
			},
		},
	})
	b.Event("start", func(ctx context.Context, _ ...string) error {
		return runtimebot.Reply(ctx, "custom start")
	})

	if err := b.Dispatch(ctx, runtimebot.Update{
		Platform: "tg",
		ChatID:   "chat-4",
		UserID:   "200",
		Message:  runtimebot.Message{Text: "/start ref_100"},
	}); err != nil {
		t.Fatalf("dispatch start: %v", err)
	}

	if !called {
		t.Fatalf("expected referral hook to be called")
	}
	if got, want := event.InviterUserID, "100"; got != want {
		t.Fatalf("InviterUserID = %q, want %q", got, want)
	}
	if got, want := event.InvitedUserID, "200"; got != want {
		t.Fatalf("InvitedUserID = %q, want %q", got, want)
	}
	if got, want := event.GrantedCoins, int64(1); got != want {
		t.Fatalf("GrantedCoins = %d, want %d", got, want)
	}
	if got, want := event.RewardProgress, 0.0; got != want {
		t.Fatalf("RewardProgress = %v, want %v", got, want)
	}
}

func newCustomStartBot(store *db.Store) (*runtimebot.Bot, *captureAdapter) {
	adapter := &captureAdapter{}
	b := runtimebot.New(adapter)
	system.Register(b, system.Dependencies{Store: store})
	b.Event("start", func(ctx context.Context, _ ...string) error {
		return runtimebot.Reply(ctx, "custom start")
	})
	return b, adapter
}

func openTestStore(t *testing.T, ctx context.Context) *db.Store {
	t.Helper()

	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}
