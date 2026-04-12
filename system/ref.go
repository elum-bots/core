package system

import (
	"context"
	"fmt"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerRef(b *elumbot.Bot, deps Dependencies) {
	b.Event("ref", func(ctx context.Context, _ ...string) error {
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

		rewardPerRef := referralRewardPerRef()
		return elumbot.Reply(ctx, fmt.Sprintf(
			"Реферальная ссылка:\n%s\n\nПриглашено друзей: %d\nНаграда за одного реферала: %s\nНакопленный прогресс к следующей 1 единице баланса: %s",
			referralLink(upd.Platform, upd.UserID),
			user.ReferralCnt,
			formatDecimal(rewardPerRef),
			formatDecimal(user.ReferralRewardProgress),
		))
	})
}
