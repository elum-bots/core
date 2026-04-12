package system

import (
	"context"
	"log"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
)

func registerStart(b *elumbot.Bot, deps Dependencies) {
	b.Event("start", func(ctx context.Context, args ...string) error {
		store, err := requireStore(deps)
		if err != nil {
			return err
		}
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}

		_, getErr := store.Users.Get(ctx, upd.UserID)
		isNewUser := sqlNotFound(getErr)

		if _, err := store.Users.Ensure(ctx, upd.UserID); err != nil {
			return err
		}
		_ = store.Users.Touch(ctx, upd.UserID)
		_ = store.Metrics.Record(ctx, db.MetricStart, upd.UserID, 0, 1)

		if len(args) > 0 {
			startArg := strings.TrimSpace(args[0])
			if isNewUser {
				if refUserID := parseStartReferralUserID(startArg); refUserID != "" && refUserID != upd.UserID {
					_, _ = store.Users.Ensure(ctx, refUserID)
					linked, granted, remainder, err := store.Users.RegisterReferralWithReward(ctx, upd.UserID, refUserID, referralRewardPerRef())
					if err != nil {
						log.Printf("ref link failed inviter=%s invited=%s err=%v", refUserID, upd.UserID, err)
					} else if linked {
						log.Printf("ref bonus granted inviter=%s invited=%s reward_per_ref=%s granted=%d remainder=%s", refUserID, upd.UserID, formatDecimal(referralRewardPerRef()), granted, formatDecimal(remainder))
						event := ReferralSuccess{
							InviterUserID:  refUserID,
							InvitedUserID:  upd.UserID,
							GrantedCoins:   granted,
							RewardProgress: remainder,
						}
						for _, hook := range deps.ReferralHooks {
							if hook == nil {
								continue
							}
							if err := hook(ctx, b, deps, event); err != nil {
								log.Printf("referral success hook error inviter=%s invited=%s err=%v", refUserID, upd.UserID, err)
							}
						}
					} else {
						log.Printf("ref link skipped inviter=%s invited=%s", refUserID, upd.UserID)
					}
				}
			}
			if code := parseStartTrackingCode(startArg); code != "" {
				if registered, err := store.Track.MarkVisitByCode(ctx, upd.UserID, code); err != nil {
					log.Printf("track visit failed platform=%s user=%s code=%s err=%v", upd.Platform, upd.UserID, code, err)
				} else {
					log.Printf("track visit platform=%s user=%s code=%s registered=%t", upd.Platform, upd.UserID, code, registered)
				}
			}
		}

		return nil
	})
}
