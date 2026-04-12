package system

import elumbot "github.com/elum-bots/core/internal/bot"

func Register(b *elumbot.Bot, deps Dependencies) {
	if b == nil {
		return
	}

	registerMandatoryDialogs(b, deps)
	registerPostDialogs(b, deps)
	registerTaskDialogs(b, deps)

	registerStart(b, deps)
	registerClose(b)
	registerBalance(b, deps)
	registerRef(b, deps)
	registerMyID(b)
	registerGetChatID(b)
	registerHelp(b, deps)
	registerStats(b, deps)
	registerBalanceAdd(b, deps)
	registerBroadcastActive(b, deps)
	registerBroadcastStats(b, deps)
	registerBroadcastStop(b, deps)

	registerPostCreate(b, deps)
	registerPostList(b, deps)
	registerPostPreview(b, deps)
	registerPostSend(b, deps)

	registerMandatoryAdd(b, deps)
	registerMandatoryList(b, deps)
	registerMandatoryDel(b, deps)
	registerMandatoryReward(b, deps)
	registerEarnMore(b, deps)
	registerTaskDone(b, deps)
	registerTaskCreate(b, deps)
	registerTaskEdit(b, deps)
	registerTaskList(b, deps)

	registerTrackCreate(b, deps)
	registerTrackList(b, deps)
	registerTrackGet(b, deps)
	registerTrackDel(b, deps)

	registerIntegrationTokenDialogs(b, deps)
	registerIntegrationTokenCommands(b, deps)
}
