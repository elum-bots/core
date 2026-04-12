package system

import (
	"context"
	"fmt"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerMyID(b *elumbot.Bot) {
	handler := func(ctx context.Context, _ ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		return elumbot.Reply(ctx, fmt.Sprintf(
			"Ваши идентификаторы:\nuser_id: %s\nchat_id: %s\nplatform: %s",
			upd.UserID,
			upd.ChatID,
			upd.Platform,
		))
	}

	b.Event("my_id", handler)
	b.Event("myid", handler)
}
