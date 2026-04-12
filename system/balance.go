package system

import (
	"context"
	"fmt"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
)

func registerBalance(b *elumbot.Bot, deps Dependencies) {
	b.Event("balance", func(ctx context.Context, _ ...string) error {
		store, err := requireStore(deps)
		if err != nil {
			return err
		}
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		user, err := store.Users.Ensure(ctx, upd.UserID)
		if err != nil {
			return err
		}
		return elumbot.Reply(ctx, fmt.Sprintf("Ваш баланс: %d", user.Coins))
	})
}

func registerBalanceAdd(b *elumbot.Bot, deps Dependencies) {
	handler := func(ctx context.Context, args ...string) error {
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
		if len(args) < 2 {
			return elumbot.Reply(ctx, "Использование: /balance_add <user_id> <amount>")
		}
		amount, ok := parsePositiveAmount(args[1])
		if !ok {
			return elumbot.Reply(ctx, "Некорректное значение amount")
		}
		if _, err := store.Users.Ensure(ctx, args[0]); err != nil {
			return err
		}
		if err := store.Users.AddCoins(ctx, args[0], amount); err != nil {
			return err
		}
		_ = store.Metrics.Record(ctx, db.MetricBalanceAdded, args[0], 0, amount)
		return elumbot.Reply(ctx, "Монеты начислены")
	}

	b.Event("balance_add", handler)
}
