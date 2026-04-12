package system

import (
	"context"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerClose(b *elumbot.Bot) {
	b.Event("close", func(ctx context.Context, _ ...string) error {
		return elumbot.Reply(ctx, "Текущий диалог закрыт.\n/start - начать заново.")
	})
}
