package system

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/elum-bots/core/internal/apiflow"
	elumbot "github.com/elum-bots/core/internal/bot"
	maxbot "github.com/elum-bots/core/internal/max-bot-api-client-go"
	"github.com/elum-bots/core/internal/max-bot-api-client-go/schemes"
	"github.com/elum-utils/env"
)

func registerGetChatID(b *elumbot.Bot) {
	b.Event("get_chat_id", func(ctx context.Context, args ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		if strings.ToLower(strings.TrimSpace(upd.Platform)) != "max" {
			return elumbot.Reply(ctx, "Команда доступна только для MAX-бота.")
		}
		if len(args) < 1 {
			return elumbot.Reply(ctx, "Использование: /get_chat_id <url_канала>")
		}

		targetURL, ok := normalizeMAXChannelURL(args[0])
		if !ok {
			return elumbot.Reply(ctx, "Некорректный URL. Пример: /get_chat_id https://max.ru/channel_name")
		}

		api, err := maxbot.New(strings.TrimSpace(env.GetEnvString("MAX_BOT_TOKEN", "")))
		if err != nil {
			return err
		}

		searchCtx, cancel := context.WithTimeout(apiflow.WithClass(ctx, apiflow.ClassAdmin), 30*time.Second)
		defer cancel()

		found, err := findMAXChatByURL(searchCtx, api, targetURL)
		if err != nil {
			return err
		}
		if found == nil {
			return elumbot.Reply(ctx, "Чат с таким URL не найден среди чатов, где установлен бот.")
		}

		title := strings.TrimSpace(found.Title)
		if title == "" {
			title = "-"
		}
		return elumbot.Reply(ctx, fmt.Sprintf(
			"Найден чат:\nID: %d\nНазвание: %s\nURL: %s",
			found.ChatId,
			title,
			targetURL,
		))
	})
}

func findMAXChatByURL(ctx context.Context, api *maxbot.Api, targetURL string) (*schemes.Chat, error) {
	var (
		found     *schemes.Chat
		next      int64
		seenPages int
	)

	for {
		var list *schemes.ChatList
		callErr := runMAXAPICall(ctx, func(callCtx context.Context) error {
			var err error
			list, err = api.Chats.GetChats(callCtx, 100, next)
			return err
		})
		if callErr != nil {
			return nil, callErr
		}

		for i := range list.Chats {
			link, ok := normalizeMAXChannelURL(list.Chats[i].Link)
			if !ok {
				continue
			}
			if strings.EqualFold(link, targetURL) {
				found = &list.Chats[i]
				break
			}
		}
		if found != nil {
			return found, nil
		}

		seenPages++
		if list.Marker == nil || *list.Marker <= 0 || seenPages >= 1000 {
			return nil, nil
		}
		next = *list.Marker
	}
}

func runMAXAPICall(ctx context.Context, fn func(context.Context) error) error {
	flow := apiflow.MAX()
	if flow == nil {
		return fn(ctx)
	}
	return flow.Do(ctx, apiflow.ClassAdmin, fn)
}

func normalizeMAXChannelURL(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}

	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "www.max.ru" {
		host = "max.ru"
	}
	if host != "max.ru" {
		return "", false
	}

	path := strings.TrimSpace(u.EscapedPath())
	if path == "" {
		path = strings.TrimSpace(u.Path)
	}
	if path == "" || path == "/" {
		return "", false
	}

	for strings.HasSuffix(path, "/") && path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path == "/" {
		return "", false
	}

	return "https://max.ru" + path, true
}
