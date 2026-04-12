package db

import (
	"context"
	"encoding/json"
	"math"
	"path/filepath"
	"testing"
)

func TestStoreUsersFlow(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "3")

	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	user, err := store.Users.Ensure(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if got, want := user.Coins, int64(3); got != want {
		t.Fatalf("coins after ensure = %d, want %d", got, want)
	}

	if err := store.Users.UpdateProfile(context.Background(), "u1", "Alex", "2000-01-01"); err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}

	if err := store.Users.AddCoins(context.Background(), "u1", 5); err != nil {
		t.Fatalf("AddCoins() error = %v", err)
	}

	ok, err := store.Users.SpendCoins(context.Background(), "u1", 4)
	if err != nil {
		t.Fatalf("SpendCoins() error = %v", err)
	}
	if !ok {
		t.Fatalf("SpendCoins() = false, want true")
	}

	user, err = store.Users.Get(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := user.Name, "Alex"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
	if got, want := user.Coins, int64(4); got != want {
		t.Fatalf("coins after updates = %d, want %d", got, want)
	}
}

func TestStoreReferralRewardFlow(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "0")

	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.Users.Ensure(context.Background(), "100"); err != nil {
		t.Fatalf("Ensure inviter error = %v", err)
	}

	for _, invitedID := range []string{"101", "102", "103"} {
		if _, err := store.Users.Ensure(context.Background(), invitedID); err != nil {
			t.Fatalf("Ensure invited error = %v", err)
		}
	}

	linked, granted, progress, err := store.Users.RegisterReferralWithReward(context.Background(), "101", "100", 0.333334)
	if err != nil {
		t.Fatalf("RegisterReferralWithReward(1) error = %v", err)
	}
	if !linked || granted != 0 {
		t.Fatalf("first reward result linked=%t granted=%d", linked, granted)
	}
	if math.Abs(progress-0.333334) > 0.00001 {
		t.Fatalf("first progress = %f", progress)
	}

	linked, granted, progress, err = store.Users.RegisterReferralWithReward(context.Background(), "102", "100", 0.333334)
	if err != nil {
		t.Fatalf("RegisterReferralWithReward(2) error = %v", err)
	}
	if !linked || granted != 0 {
		t.Fatalf("second reward result linked=%t granted=%d", linked, granted)
	}
	if math.Abs(progress-0.666668) > 0.00002 {
		t.Fatalf("second progress = %f", progress)
	}

	linked, granted, progress, err = store.Users.RegisterReferralWithReward(context.Background(), "103", "100", 0.333334)
	if err != nil {
		t.Fatalf("RegisterReferralWithReward(3) error = %v", err)
	}
	if !linked || granted != 1 {
		t.Fatalf("third reward result linked=%t granted=%d", linked, granted)
	}
	if math.Abs(progress) > 0.00001 {
		t.Fatalf("third progress = %f", progress)
	}

	user, err := store.Users.Get(context.Background(), "100")
	if err != nil {
		t.Fatalf("Get inviter error = %v", err)
	}
	if got, want := user.Coins, int64(1); got != want {
		t.Fatalf("inviter coins = %d, want %d", got, want)
	}
	if got, want := user.ReferralCnt, int64(3); got != want {
		t.Fatalf("inviter referral cnt = %d, want %d", got, want)
	}
	if math.Abs(user.ReferralRewardProgress) > 0.00001 {
		t.Fatalf("inviter progress = %f", user.ReferralRewardProgress)
	}
}

func TestStoreBotStatsFlow(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "0")

	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.Users.Ensure(context.Background(), "100"); err != nil {
		t.Fatalf("Ensure inviter error = %v", err)
	}
	if _, err := store.Users.Ensure(context.Background(), "101"); err != nil {
		t.Fatalf("Ensure invited error = %v", err)
	}
	if _, err := store.Users.Ensure(context.Background(), "102"); err != nil {
		t.Fatalf("Ensure user error = %v", err)
	}

	if _, _, _, err := store.Users.RegisterReferralWithReward(context.Background(), "101", "100", 1); err != nil {
		t.Fatalf("RegisterReferralWithReward() error = %v", err)
	}

	track, err := store.Track.Create(context.Background(), "trk12345", "test", "100")
	if err != nil {
		t.Fatalf("Track.Create() error = %v", err)
	}
	if _, err := store.Track.MarkVisitByCode(context.Background(), "102", track.Code); err != nil {
		t.Fatalf("Track.MarkVisitByCode() error = %v", err)
	}

	if err := store.Metrics.Record(context.Background(), MetricPostSent, "102", 1, 1); err != nil {
		t.Fatalf("Metrics.Record(post_sent) error = %v", err)
	}
	if err := store.Metrics.Record(context.Background(), MetricBalanceAdded, "102", 0, 5); err != nil {
		t.Fatalf("Metrics.Record(balance_added) error = %v", err)
	}

	stat, err := store.Broadcasts.Start(context.Background(), "post", 3)
	if err != nil {
		t.Fatalf("Broadcasts.Start() error = %v", err)
	}
	if err := store.Broadcasts.Finish(context.Background(), stat.ID, "completed"); err != nil {
		t.Fatalf("Broadcasts.Finish() error = %v", err)
	}

	stats, err := store.Stats.GetBotStats(context.Background())
	if err != nil {
		t.Fatalf("Stats.GetBotStats() error = %v", err)
	}

	if got, want := stats.UniqueTotal, int64(3); got != want {
		t.Fatalf("UniqueTotal = %d, want %d", got, want)
	}
	if got, want := stats.UniqueToday, int64(3); got != want {
		t.Fatalf("UniqueToday = %d, want %d", got, want)
	}
	if got, want := stats.NewUsersToday, int64(3); got != want {
		t.Fatalf("NewUsersToday = %d, want %d", got, want)
	}
	if got, want := stats.RefTotal, int64(1); got != want {
		t.Fatalf("RefTotal = %d, want %d", got, want)
	}
	if got, want := stats.RefToday, int64(1); got != want {
		t.Fatalf("RefToday = %d, want %d", got, want)
	}
	if got, want := stats.PostSentTotal, int64(1); got != want {
		t.Fatalf("PostSentTotal = %d, want %d", got, want)
	}
	if got, want := stats.TrackVisitsTotal, int64(1); got != want {
		t.Fatalf("TrackVisitsTotal = %d, want %d", got, want)
	}
	if got, want := stats.BalanceAddedTotal, int64(1); got != want {
		t.Fatalf("BalanceAddedTotal = %d, want %d", got, want)
	}
	if got, want := stats.BroadcastsTotal, int64(1); got != want {
		t.Fatalf("BroadcastsTotal = %d, want %d", got, want)
	}
	if got, want := stats.BroadcastsActive, int64(0); got != want {
		t.Fatalf("BroadcastsActive = %d, want %d", got, want)
	}
}

func TestTrackMarkVisitByMissingCode(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	created, err := store.Track.MarkVisitByCode(context.Background(), "u1", "missing-code")
	if err != nil {
		t.Fatalf("Track.MarkVisitByCode() error = %v", err)
	}
	if created {
		t.Fatalf("Track.MarkVisitByCode() = true, want false")
	}
}

func TestOpenTracksAppliedMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bot.sqlite")

	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	var appliedCount int
	if err := store.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&appliedCount); err != nil {
		_ = store.Close()
		t.Fatalf("count schema_migrations error = %v", err)
	}
	if got, want := appliedCount, 1; got != want {
		_ = store.Close()
		t.Fatalf("schema_migrations count = %d, want %d", got, want)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	store, err = Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() second time error = %v", err)
	}
	defer store.Close()

	if err := store.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&appliedCount); err != nil {
		t.Fatalf("count schema_migrations second time error = %v", err)
	}
	if got, want := appliedCount, 1; got != want {
		t.Fatalf("schema_migrations count on reopen = %d, want %d", got, want)
	}
}

func TestStoreBroadcastQueueFlow(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	payload, err := json.Marshal(map[string]any{"post_id": 7})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	stat, err := store.Broadcasts.Create(context.Background(), "post", "admin-chat", string(payload), []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("Broadcasts.Create() error = %v", err)
	}
	if got, want := stat.Total, int64(2); got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got, want := stat.Status, "running"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	pending, err := store.Broadcasts.ListPendingTargets(context.Background(), stat.ID, 10)
	if err != nil {
		t.Fatalf("ListPendingTargets() error = %v", err)
	}
	if got, want := len(pending), 2; got != want {
		t.Fatalf("pending len = %d, want %d", got, want)
	}

	stopped, err := store.Broadcasts.RequestStop(context.Background(), stat.ID)
	if err != nil {
		t.Fatalf("RequestStop() error = %v", err)
	}
	if !stopped {
		t.Fatalf("RequestStop() = false, want true")
	}

	stat, err = store.Broadcasts.Get(context.Background(), stat.ID)
	if err != nil {
		t.Fatalf("Broadcasts.Get() error = %v", err)
	}
	if got, want := stat.Status, "cancel_requested"; got != want {
		t.Fatalf("status after stop = %q, want %q", got, want)
	}
	if !stat.StopRequested {
		t.Fatalf("stop_requested = false, want true")
	}

	marked, err := store.Broadcasts.MarkTargetSent(context.Background(), stat.ID, "u1")
	if err != nil {
		t.Fatalf("MarkTargetSent() error = %v", err)
	}
	if !marked {
		t.Fatalf("MarkTargetSent() = false, want true")
	}
	if err := store.Broadcasts.IncSuccess(context.Background(), stat.ID); err != nil {
		t.Fatalf("IncSuccess() error = %v", err)
	}

	if err := store.Broadcasts.Finish(context.Background(), stat.ID, "canceled"); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	stat, err = store.Broadcasts.Get(context.Background(), stat.ID)
	if err != nil {
		t.Fatalf("Broadcasts.Get() after finish error = %v", err)
	}
	if stat.Active {
		t.Fatalf("active = true, want false")
	}
	if got, want := stat.Status, "canceled"; got != want {
		t.Fatalf("final status = %q, want %q", got, want)
	}
}

func TestStoreTasksFlow(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "0")

	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.Users.Ensure(context.Background(), "u1"); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	task, err := store.Tasks.Create(context.Background(), 3, []TaskChannel{
		{
			ChannelID:     "-100123",
			Title:         "Channel One",
			URL:           "https://t.me/channel_one",
			RequiresCheck: true,
		},
		{
			ChannelID:     "@channel_two",
			Title:         "Channel Two",
			URL:           "https://t.me/channel_two",
			RequiresCheck: false,
		},
	})
	if err != nil {
		t.Fatalf("Tasks.Create() error = %v", err)
	}
	if got, want := task.Reward, int64(3); got != want {
		t.Fatalf("reward = %d, want %d", got, want)
	}
	if got, want := len(task.Channels), 2; got != want {
		t.Fatalf("channels len = %d, want %d", got, want)
	}

	next, ok, err := store.Tasks.NextPending(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Tasks.NextPending() error = %v", err)
	}
	if !ok || next.ID != task.ID {
		t.Fatalf("NextPending() = (%v, %t), want task %d", next.ID, ok, task.ID)
	}

	reward, granted, err := store.Tasks.GrantReward(context.Background(), "u1", task.ID)
	if err != nil {
		t.Fatalf("Tasks.GrantReward() error = %v", err)
	}
	if !granted || reward != 3 {
		t.Fatalf("GrantReward() = (%d, %t), want (3, true)", reward, granted)
	}

	reward, granted, err = store.Tasks.GrantReward(context.Background(), "u1", task.ID)
	if err != nil {
		t.Fatalf("Tasks.GrantReward() second error = %v", err)
	}
	if granted || reward != 3 {
		t.Fatalf("second GrantReward() = (%d, %t), want (3, false)", reward, granted)
	}

	user, err := store.Users.Get(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Users.Get() error = %v", err)
	}
	if got, want := user.Coins, int64(3); got != want {
		t.Fatalf("user coins = %d, want %d", got, want)
	}

	list, err := store.Tasks.List(context.Background())
	if err != nil {
		t.Fatalf("Tasks.List() error = %v", err)
	}
	if got, want := list[0].CompletedTotal, int64(1); got != want {
		t.Fatalf("completed total = %d, want %d", got, want)
	}
}

func TestIntegrationTokensFlow(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	item, err := store.IntegrationTokens.Create(context.Background(), IntegrationProviderDeepSeek, "secret-token-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got, want := item.Provider, IntegrationProviderDeepSeek; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}

	values, err := store.IntegrationTokens.ValuesByProvider(context.Background(), IntegrationProviderDeepSeek)
	if err != nil {
		t.Fatalf("ValuesByProvider() error = %v", err)
	}
	if len(values) != 1 || values[0] != "secret-token-1" {
		t.Fatalf("unexpected values: %#v", values)
	}

	updated, err := store.IntegrationTokens.Update(context.Background(), IntegrationProviderDeepSeek, item.ID, "secret-token-2")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !updated {
		t.Fatalf("Update() = false, want true")
	}

	got, err := store.IntegrationTokens.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Token != "secret-token-2" {
		t.Fatalf("token after update = %q", got.Token)
	}

	deleted, err := store.IntegrationTokens.Delete(context.Background(), IntegrationProviderDeepSeek, item.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatalf("Delete() = false, want true")
	}

	values, err = store.IntegrationTokens.ValuesByProvider(context.Background(), IntegrationProviderDeepSeek)
	if err != nil {
		t.Fatalf("ValuesByProvider() after delete error = %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("values after delete = %#v, want empty", values)
	}
}
