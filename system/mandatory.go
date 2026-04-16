package system

import (
	"context"
	"fmt"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerMandatoryAdd(b *elumbot.Bot, deps Dependencies) {
	b.Event("mandatory_add", func(ctx context.Context, args ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		if len(args) != 0 {
			return elumbot.Reply(ctx, "Использование: /mandatory_add")
		}
		return b.StartDialog(ctx, dialogMandatoryAdd, nil)
	})
}

func registerMandatoryList(b *elumbot.Bot, deps Dependencies) {
	b.Event("mandatory_list", func(ctx context.Context, _ ...string) error {
		store, err := requireStore(deps)
		if err != nil {
			return err
		}
		verifiedCount, err := store.Mandatory.CountVerifiedUsers(ctx)
		if err != nil {
			return err
		}
		items, err := store.Mandatory.List(ctx)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return elumbot.Reply(ctx, fmt.Sprintf("Обязательные каналы не найдены.\n\nПрошли проверку обязательной подписки: %d", verifiedCount))
		}
		lines := make([]string, 0, len(items)+1)
		lines = append(lines, "Обязательные каналы:")
		lines = append(lines, fmt.Sprintf("Прошли проверку обязательной подписки: %d", verifiedCount))
		for _, item := range items {
			check := "1"
			if !item.RequiresCheck {
				check = "0"
			}
			lines = append(lines, fmt.Sprintf("%d) %s (%s) | проверка=%s", item.ID, item.Title, item.ChannelID, check))
		}
		return elumbot.Reply(ctx, strings.Join(lines, "\n"))
	})
}

func registerMandatoryDel(b *elumbot.Bot, deps Dependencies) {
	b.Event("mandatory_del", func(ctx context.Context, args ...string) error {
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
		id, ok := parseSingleArgInt(args)
		if !ok {
			return elumbot.Reply(ctx, "Использование: /mandatory_del <id>")
		}
		if err := store.Mandatory.Delete(ctx, id); err != nil {
			return err
		}
		return elumbot.Reply(ctx, "Mandatory удален")
	})
}

func registerMandatoryReward(b *elumbot.Bot, deps Dependencies) {
	b.Event("mandatory_reward", func(ctx context.Context, args ...string) error {
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

		if len(args) == 1 {
			rowID, ok := parseSingleArgInt(args)
			if !ok {
				return elumbot.Reply(ctx, "Использование: /mandatory_reward <id>\nИли запустите /mandatory_reward без аргументов для выбора канала.")
			}
			return b.StartDialog(ctx, dialogMandatoryRewardReset, map[string]any{
				keyMandatoryRewardRowID: rowID,
			})
		}

		items, err := store.Mandatory.List(ctx)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return elumbot.Reply(ctx, "Нет обязательных каналов для запуска рассылки.")
		}
		lines := make([]string, 0, len(items)+1)
		lines = append(lines, "Mandatory каналы:")
		for _, item := range items {
			check := "1"
			if !item.RequiresCheck {
				check = "0"
			}
			lines = append(lines, fmt.Sprintf("%d) %s (%s) | проверка=%s\n%s", item.ID, item.Title, item.ChannelID, check, item.URL))
		}
		if err := elumbot.Reply(ctx, strings.Join(lines, "\n\n")); err != nil {
			return err
		}
		return b.StartDialog(ctx, dialogMandatoryRewardSelect, nil)
	})
}

func launchMandatoryReward(dc elumbot.DialogContext, b *elumbot.Bot, deps Dependencies, rowID int64, reset bool) error {
	store, err := requireStore(deps)
	if err != nil {
		return err
	}
	upd, err := currentUpdate(dc.Context())
	if err != nil {
		return dc.Reply("context error")
	}

	channel, found, err := findMandatoryChannel(dc.Context(), store, rowID)
	if err != nil {
		return err
	}
	if !found {
		return dc.Reply("Введите корректный row_id канала (целое число > 0).")
	}
	if reset {
		if err := store.Mandatory.ResetRewardProgress(dc.Context(), rowID); err != nil {
			return err
		}
	}
	if err := dc.Reply("Запустил mandatory_reward в фоне. Итог отправлю отдельным сообщением."); err != nil {
		return err
	}
	if deps.Broadcasts == nil {
		return dc.Reply("Сервис рассылок не инициализирован")
	}
	targets, err := store.Mandatory.ListUsersPendingReward(dc.Context(), channel.ID)
	if err != nil {
		return err
	}
	if _, err := deps.Broadcasts.StartMandatoryReward(dc.Context(), upd.ChatID, channel, reset, targets); err != nil {
		return err
	}
	return nil
}
