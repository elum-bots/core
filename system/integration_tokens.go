package system

import (
	"context"
	"fmt"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
)

func registerIntegrationTokenCommands(b *elumbot.Bot, deps Dependencies) {
	registerDeepSeekTokenCommands(b, deps)
	registerGeminiTokenCommands(b, deps)
}

func registerDeepSeekTokenCommands(b *elumbot.Bot, deps Dependencies) {
	b.Event("deepseek_token_add", func(ctx context.Context, _ ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		if !deepSeekFeatureEnabled(deps) {
			return startFeatureDisabled(ctx)
		}
		return b.StartDialog(ctx, dialogDeepSeekTokenAdd, nil)
	})

	b.Event("deepseek_token_list", func(ctx context.Context, _ ...string) error {
		return listIntegrationTokens(ctx, b, deps, db.IntegrationProviderDeepSeek, "DeepSeek")
	})

	b.Event("deepseek_token_edit", func(ctx context.Context, args ...string) error {
		return startIntegrationTokenEditDialog(ctx, b, deps, db.IntegrationProviderDeepSeek, dialogDeepSeekTokenEdit, args)
	})

	b.Event("deepseek_token_del", func(ctx context.Context, args ...string) error {
		return deleteIntegrationToken(ctx, deps, db.IntegrationProviderDeepSeek, args, "DeepSeek")
	})
}

func registerGeminiTokenCommands(b *elumbot.Bot, deps Dependencies) {
	b.Event("gemini_token_add", func(ctx context.Context, _ ...string) error {
		upd, err := currentUpdate(ctx)
		if err != nil {
			return elumbot.Reply(ctx, "context error")
		}
		if !isAdmin(upd) {
			return startAdminOnly(ctx)
		}
		if !geminiFeatureEnabled(deps) {
			return startFeatureDisabled(ctx)
		}
		return b.StartDialog(ctx, dialogGeminiTokenAdd, nil)
	})

	b.Event("gemini_token_list", func(ctx context.Context, _ ...string) error {
		return listIntegrationTokens(ctx, b, deps, db.IntegrationProviderGemini, "Gemini")
	})

	b.Event("gemini_token_edit", func(ctx context.Context, args ...string) error {
		return startIntegrationTokenEditDialog(ctx, b, deps, db.IntegrationProviderGemini, dialogGeminiTokenEdit, args)
	})

	b.Event("gemini_token_del", func(ctx context.Context, args ...string) error {
		return deleteIntegrationToken(ctx, deps, db.IntegrationProviderGemini, args, "Gemini")
	})
}

func startIntegrationTokenEditDialog(ctx context.Context, b *elumbot.Bot, deps Dependencies, provider, dialogName string, args []string) error {
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
	if !integrationFeatureEnabled(deps, provider) {
		return startFeatureDisabled(ctx)
	}
	id, ok := parseSingleArgInt(args)
	if !ok {
		return elumbot.Reply(ctx, fmt.Sprintf("Использование: /%s_token_edit <id>", provider))
	}
	item, err := store.IntegrationTokens.Get(ctx, id)
	if err != nil {
		if sqlNotFound(err) {
			return elumbot.Reply(ctx, "Токен не найден")
		}
		return err
	}
	if item.Provider != provider {
		return elumbot.Reply(ctx, "Токен не найден")
	}
	return b.StartDialog(ctx, dialogName, map[string]any{
		keyIntegrationTokenID: id,
	})
}

func deleteIntegrationToken(ctx context.Context, deps Dependencies, provider string, args []string, title string) error {
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
	if !integrationFeatureEnabled(deps, provider) {
		return startFeatureDisabled(ctx)
	}
	id, ok := parseSingleArgInt(args)
	if !ok {
		return elumbot.Reply(ctx, fmt.Sprintf("Использование: /%s_token_del <id>", provider))
	}
	deleted, err := store.IntegrationTokens.Delete(ctx, provider, id)
	if err != nil {
		return err
	}
	if !deleted {
		return elumbot.Reply(ctx, "Токен не найден")
	}
	invalidateIntegrationCache(deps, provider)
	return elumbot.Reply(ctx, fmt.Sprintf("%s токен удален.", title))
}

func listIntegrationTokens(ctx context.Context, b *elumbot.Bot, deps Dependencies, provider string, title string) error {
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
	if !integrationFeatureEnabled(deps, provider) {
		return startFeatureDisabled(ctx)
	}
	items, err := store.IntegrationTokens.ListByProvider(ctx, provider)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return elumbot.Reply(ctx, fmt.Sprintf("%s токенов пока нет.", title))
	}
	lines := make([]string, 0, len(items)+1)
	lines = append(lines, title+" токены:")
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("ID=%d | %s | updated=%s", item.ID, maskToken(item.Token), item.UpdatedAt.UTC().Format("2006-01-02 15:04:05")))
	}
	return replyLongText(ctx, b, upd.ChatID, strings.Join(lines, "\n"))
}

func integrationFeatureEnabled(deps Dependencies, provider string) bool {
	switch provider {
	case db.IntegrationProviderDeepSeek:
		return deepSeekFeatureEnabled(deps)
	case db.IntegrationProviderGemini:
		return geminiFeatureEnabled(deps)
	default:
		return false
	}
}

func invalidateIntegrationCache(deps Dependencies, provider string) {
	if deps.Integrations == nil {
		return
	}
	switch provider {
	case db.IntegrationProviderDeepSeek:
		if deps.Integrations.DeepSeek != nil {
			deps.Integrations.DeepSeek.Invalidate()
		}
	case db.IntegrationProviderGemini:
		if deps.Integrations.Gemini != nil {
			deps.Integrations.Gemini.Invalidate()
		}
	}
}
