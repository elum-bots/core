package system

import (
	"context"
	"fmt"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerTrackCreate(b *elumbot.Bot, deps Dependencies) {
	b.Event("track_create", func(ctx context.Context, args ...string) error {
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

		label := strings.TrimSpace(strings.Join(args, " "))
		if label == "" {
			return elumbot.Reply(ctx, "Использование: /track_create <метка>")
		}

		var createdErr error
		var linkURL string
		for range 10 {
			item, err := store.Track.Create(ctx, randomTrackCode(8), label, upd.UserID)
			if err != nil {
				createdErr = err
				if strings.Contains(strings.ToLower(err.Error()), "unique") {
					continue
				}
				return err
			}
			linkURL = trackLink(upd.Platform, item.Code)
			return elumbot.Reply(ctx, fmt.Sprintf(
				"Track создан:\nID: %d\nМетка: %s\nCode: %s\nURL: %s",
				item.ID, item.Label, item.Code, linkURL,
			))
		}
		if createdErr != nil {
			return createdErr
		}
		return elumbot.Reply(ctx, "Не удалось создать track-ссылку")
	})
}

func registerTrackList(b *elumbot.Bot, deps Dependencies) {
	b.Event("track_list", func(ctx context.Context, _ ...string) error {
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
		items, err := store.Track.List(ctx)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return elumbot.Reply(ctx, "Track-ссылок пока нет")
		}
		lines := make([]string, 0, len(items)+1)
		lines = append(lines, "Track ссылки:")
		for _, item := range items {
			lines = append(lines, fmt.Sprintf(
				"ID=%d | %s | code=%s | пришло=%d | сделали целевое действие=%d\n%s",
				item.ID, item.Label, item.Code, item.ArrivalsCount, item.GeneratedUsersCount, trackLink(upd.Platform, item.Code),
			))
		}
		return replyLongText(ctx, b, upd.ChatID, strings.Join(lines, "\n\n"))
	})
}

func registerTrackGet(b *elumbot.Bot, deps Dependencies) {
	b.Event("track_get", func(ctx context.Context, args ...string) error {
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
			return elumbot.Reply(ctx, "Использование: /track_get <id>")
		}
		item, err := store.Track.Get(ctx, id)
		if err != nil {
			if sqlNotFound(err) {
				return elumbot.Reply(ctx, "Track-ссылка не найдена")
			}
			return err
		}
		return elumbot.Reply(ctx, fmt.Sprintf(
			"Track:\nID: %d\nМетка: %s\nCode: %s\nПришло: %d\nСделали целевое действие: %d\nURL: %s",
			item.ID, item.Label, item.Code, item.ArrivalsCount, item.GeneratedUsersCount, trackLink(upd.Platform, item.Code),
		))
	})
}

func registerTrackDel(b *elumbot.Bot, deps Dependencies) {
	b.Event("track_del", func(ctx context.Context, args ...string) error {
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
			return elumbot.Reply(ctx, "Использование: /track_del <id>")
		}
		if err := store.Track.Delete(ctx, id); err != nil {
			return err
		}
		return elumbot.Reply(ctx, "Track-ссылка удалена")
	})
}
