package system

import (
	"context"
	"fmt"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
	integration "github.com/elum-bots/core/internal/integration"
)

func registerEarnMore(b *elumbot.Bot, deps Dependencies) {
	b.Event("earn_more", func(ctx context.Context, _ ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if err := touchUserForDialog(ctx, deps, upd.UserID); err != nil {
			return err
		}
		return replyNextTask(ctx, deps)
	})

	b.Event("button:"+buttonTaskCheck, func(ctx context.Context, _ ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if err := touchUserForDialog(ctx, deps, upd.UserID); err != nil {
			return err
		}
		return checkNextTask(ctx, deps, upd)
	})
}

func registerTaskDone(b *elumbot.Bot, deps Dependencies) {
	b.Event("task_done", func(ctx context.Context, args ...string) error {
		store, err := requireStore(deps)
		if err != nil {
			return err
		}
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if err := touchUserForDialog(ctx, deps, upd.UserID); err != nil {
			return err
		}
		taskID, ok := parseSingleArgInt(args)
		if !ok {
			return elumbot.Reply(ctx, "Использование: /task_done <task_id>")
		}
		task, err := store.Tasks.Get(ctx, taskID)
		if err != nil {
			if sqlNotFound(err) {
				return elumbot.Reply(ctx, "Задание не найдено")
			}
			return err
		}
		return checkAndRewardTask(ctx, deps, upd, task)
	})
}

func registerTaskCreate(b *elumbot.Bot, deps Dependencies) {
	b.Event("task_create", func(ctx context.Context, args ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		if len(args) != 0 {
			return elumbot.Reply(ctx, "Использование: /task_create")
		}
		return b.StartDialog(ctx, dialogTaskCreate, nil)
	})
}

func registerTaskEdit(b *elumbot.Bot, deps Dependencies) {
	b.Event("task_edit", func(ctx context.Context, args ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		taskID, ok := parseSingleArgInt(args)
		if !ok {
			return elumbot.Reply(ctx, "Использование: /task_edit <id>")
		}
		return b.StartDialog(ctx, dialogTaskEdit, map[string]any{
			keyTaskID: taskID,
		})
	})
}

func registerTaskList(b *elumbot.Bot, deps Dependencies) {
	b.Event("task_list", func(ctx context.Context, _ ...string) error {
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
		tasks, err := store.Tasks.List(ctx)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			return elumbot.Reply(ctx, "Активных заданий нет")
		}
		lines := make([]string, 0, len(tasks))
		for _, task := range tasks {
			lines = append(lines, taskAdminText(task))
		}
		return elumbot.Reply(ctx, "Задания:\n\n"+strings.Join(lines, "\n\n"))
	})
}

func replyNextTask(ctx context.Context, deps Dependencies) error {
	store, err := requireStore(deps)
	if err != nil {
		return err
	}
	upd, err := currentUpdate(ctx)
	if err != nil {
		return elumbot.Reply(ctx, "context error")
	}
	task, ok, err := store.Tasks.NextPending(ctx, upd.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return elumbot.Reply(ctx, "Сейчас доступных заданий нет")
	}
	return elumbot.Reply(ctx, taskCardText(task), elumbot.WithButtons(taskButtons(task.Channels)...))
}

func checkNextTask(ctx context.Context, deps Dependencies, upd elumbot.Update) error {
	store, err := requireStore(deps)
	if err != nil {
		return err
	}
	task, ok, err := store.Tasks.NextPending(ctx, upd.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return elumbot.Reply(ctx, "Сейчас доступных заданий нет")
	}
	return checkAndRewardTask(ctx, deps, upd, task)
}

func checkAndRewardTask(ctx context.Context, deps Dependencies, upd elumbot.Update, task db.Task) error {
	store, err := requireStore(deps)
	if err != nil {
		return err
	}
	missing, err := missingTaskChannels(ctx, deps, upd, task)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return elumbot.Reply(ctx, taskMissingText(task, missing), elumbot.WithButtons(taskButtons(missing)...))
	}
	reward, granted, err := store.Tasks.GrantReward(ctx, upd.UserID, task.ID)
	if err != nil {
		return err
	}
	if !granted {
		return elumbot.Reply(ctx, "Награда по заданию уже получена")
	}
	if err := elumbot.Reply(ctx, fmt.Sprintf("Задание выполнено. Начислено %d монет", reward)); err != nil {
		return err
	}
	return replyNextTask(ctx, deps)
}

func missingTaskChannels(ctx context.Context, deps Dependencies, upd elumbot.Update, task db.Task) ([]db.TaskChannel, error) {
	if len(task.Channels) == 0 {
		return nil, nil
	}
	missing := make([]db.TaskChannel, 0, len(task.Channels))
	for _, channel := range task.Channels {
		if !channel.RequiresCheck {
			continue
		}
		ok, err := integration.IsSubscribedToChannel(ctx, deps.Integrations, upd.Platform, upd.UserID, channel.ChannelID)
		if err != nil {
			return nil, err
		}
		if !ok {
			missing = append(missing, channel)
		}
	}
	return missing, nil
}
