package system

import (
	"context"
	"fmt"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerPostCreate(b *elumbot.Bot, deps Dependencies) {
	b.Event("post_create", func(ctx context.Context, args ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		if len(args) != 0 {
			return elumbot.Reply(ctx, "Использование: /post_create")
		}
		return b.StartDialog(ctx, dialogPostCreate, nil)
	})
}

func registerPostList(b *elumbot.Bot, deps Dependencies) {
	b.Event("post_list", func(ctx context.Context, _ ...string) error {
		store, err := requireStore(deps)
		if err != nil {
			return err
		}
		items, err := store.Posts.List(ctx)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return elumbot.Reply(ctx, "Постов нет")
		}
		lines := make([]string, 0, len(items)+1)
		lines = append(lines, "Посты:")
		for _, item := range items {
			preview := strings.TrimSpace(item.Title)
			if preview == "" {
				preview = strings.TrimSpace(item.Text)
			}
			if idx := strings.Index(preview, "\n"); idx >= 0 {
				preview = strings.TrimSpace(preview[:idx])
			}
			lines = append(lines, fmt.Sprintf("#%d %s", item.ID, preview))
		}
		return elumbot.Reply(ctx, strings.Join(lines, "\n\n"))
	})
}

func registerPostPreview(b *elumbot.Bot, deps Dependencies) {
	b.Event("post_preview", func(ctx context.Context, args ...string) error {
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
		postID, ok := parseSingleArgInt(args)
		if !ok {
			return elumbot.Reply(ctx, "Использование: /post_preview <post_id>")
		}
		post, err := store.Posts.Get(ctx, postID)
		if err != nil {
			if sqlNotFound(err) {
				return elumbot.Reply(ctx, "Ничего не найдено")
			}
			return err
		}
		personalChatID := upd.UserID
		if strings.TrimSpace(personalChatID) == "" {
			personalChatID = upd.ChatID
		}
		if err := b.Send(ctx, personalChatID, post.Text, makePostSendOptions(post)...); err != nil {
			return err
		}
		return elumbot.Reply(ctx, "Превью отправлено в личный чат.")
	})
}

func registerPostSend(b *elumbot.Bot, deps Dependencies) {
	b.Event("post_send", func(ctx context.Context, args ...string) error {
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
		postID, ok := parseSingleArgInt(args)
		if !ok {
			return elumbot.Reply(ctx, "Использование: /post_send <post_id>")
		}
		if _, err := store.Posts.Get(ctx, postID); err != nil {
			if sqlNotFound(err) {
				return elumbot.Reply(ctx, "Ничего не найдено")
			}
			return err
		}
		userIDs, err := store.Users.ListUserIDs(ctx)
		if err != nil {
			return err
		}
		if err := elumbot.Reply(ctx, "Запущена рассылка поста"); err != nil {
			return err
		}
		if deps.Broadcasts == nil {
			return elumbot.Reply(ctx, "Сервис рассылок не инициализирован")
		}
		if _, err := deps.Broadcasts.StartPost(ctx, upd.ChatID, postID, userIDs); err != nil {
			return err
		}
		return nil
	})
}
