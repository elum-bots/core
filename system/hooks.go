package system

import (
	"context"

	elumbot "github.com/elum-bots/core/internal/bot"
)

type ReferralSuccess struct {
	InviterUserID  string
	InvitedUserID  string
	GrantedCoins   int64
	RewardProgress float64
}

type ReferralSuccessHook func(context.Context, *elumbot.Bot, Dependencies, ReferralSuccess) error
