package system

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerBroadcastStats(b *elumbot.Bot, deps Dependencies) {
	b.Event("broadcast_stats", func(ctx context.Context, args ...string) error {
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

		limit := int64(20)
		if len(args) > 0 {
			v, convErr := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if convErr == nil && v > 0 && v <= 100 {
				limit = v
			}
		}

		items, err := store.Broadcasts.List(ctx, limit)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return elumbot.Reply(ctx, "Статистики рассылок пока нет")
		}

		lines := make([]string, 0, len(items)+1)
		lines = append(lines, "Последние рассылки:")
		for _, item := range items {
			lines = append(lines, broadcastStatText(item))
		}
		return replyLongText(ctx, b, upd.ChatID, strings.Join(lines, "\n"))
	})
}

func registerBroadcastActive(b *elumbot.Bot, deps Dependencies) {
	b.Event("broadcast_active", func(ctx context.Context, _ ...string) error {
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

		items, err := store.Broadcasts.Active(ctx)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return elumbot.Reply(ctx, "Активных рассылок сейчас нет")
		}

		lines := make([]string, 0, len(items)+1)
		lines = append(lines, "Активные рассылки:")
		for _, item := range items {
			lines = append(lines, broadcastStatText(item))
		}
		return replyLongText(ctx, b, upd.ChatID, strings.Join(lines, "\n"))
	})
}

func registerBroadcastStop(b *elumbot.Bot, deps Dependencies) {
	b.Event("broadcast_stop", func(ctx context.Context, args ...string) error {
		if _, err := requireStore(deps); err != nil {
			return err
		}
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		if deps.Broadcasts == nil {
			return elumbot.Reply(ctx, "Сервис рассылок не инициализирован")
		}

		id, ok := parseSingleArgInt(args)
		if !ok {
			return elumbot.Reply(ctx, "Использование: /broadcast_stop <id>")
		}

		item, requested, err := deps.Broadcasts.RequestStop(ctx, id)
		if err != nil {
			if sqlNotFound(err) {
				return elumbot.Reply(ctx, "Ничего не найдено")
			}
			return err
		}
		if !item.Active {
			return elumbot.Reply(ctx, fmt.Sprintf("Рассылка #%d уже не активна. Текущий статус: %s", item.ID, item.Status))
		}
		if !requested {
			return elumbot.Reply(ctx, fmt.Sprintf("Остановка для рассылки #%d уже запрошена.", item.ID))
		}
		return elumbot.Reply(ctx, fmt.Sprintf("Запросил остановку рассылки #%d.", item.ID))
	})
}
