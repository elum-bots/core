package system

import (
	"github.com/elum-bots/core/internal/db"
	integration "github.com/elum-bots/core/internal/integration"
)

type Dependencies struct {
	Store         *db.Store
	Integrations  *integration.Services
	HelpInfo      string
	ReferralHooks []ReferralSuccessHook
	Broadcasts    *BroadcastRunner
}
