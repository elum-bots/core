package system

import (
	"context"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerHelp(b *elumbot.Bot, deps Dependencies) {
	b.Event("help", func(ctx context.Context, _ ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		return elumbot.Reply(ctx, helpText(isAdmin(upd), deps))
	})
}
