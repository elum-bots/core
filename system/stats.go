package system

import (
	"context"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerStats(b *elumbot.Bot, deps Dependencies) {
	b.Event("stats", func(ctx context.Context, _ ...string) error {
		store, err := requireStore(deps)
		if err != nil {
			return err
		}
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		stats, err := store.Stats.GetBotStats(ctx)
		if err != nil {
			return err
		}
		return elumbot.Reply(ctx, statsText(stats, deps))
	})
}
