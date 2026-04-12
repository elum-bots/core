package system

import (
	"fmt"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
)

func registerIntegrationTokenDialogs(b *elumbot.Bot, deps Dependencies) {
	registerDeepSeekTokenDialogs(b, deps)
	registerGeminiTokenDialogs(b, deps)
}

func registerDeepSeekTokenDialogs(b *elumbot.Bot, deps Dependencies) {
	b.Dialog(dialogDeepSeekTokenAdd, func() *elumbot.Dialog {
		return integrationTokenAddDialog(deps, db.IntegrationProviderDeepSeek, "DeepSeek")
	})
	b.Dialog(dialogDeepSeekTokenEdit, func() *elumbot.Dialog {
		return integrationTokenEditDialog(deps, db.IntegrationProviderDeepSeek, "DeepSeek")
	})
}

func registerGeminiTokenDialogs(b *elumbot.Bot, deps Dependencies) {
	b.Dialog(dialogGeminiTokenAdd, func() *elumbot.Dialog {
		return integrationTokenAddDialog(deps, db.IntegrationProviderGemini, "Gemini")
	})
	b.Dialog(dialogGeminiTokenEdit, func() *elumbot.Dialog {
		return integrationTokenEditDialog(deps, db.IntegrationProviderGemini, "Gemini")
	})
}

func integrationTokenAddDialog(deps Dependencies, provider, title string) *elumbot.Dialog {
	return elumbot.NewDialog().
		Ask(keyIntegrationTokenValue, fmt.Sprintf("Введите %s token:", title), elumbot.WithValidator(validateTokenValue)).
		OnFinish(func(dc elumbot.DialogContext) error {
			store, err := requireStore(deps)
			if err != nil {
				return err
			}
			token := strings.TrimSpace(dc.Input(keyIntegrationTokenValue).Text)
			item, err := store.IntegrationTokens.Create(dc.Context(), provider, token)
			if err != nil {
				return err
			}
			invalidateIntegrationCache(deps, provider)
			return dc.Reply(fmt.Sprintf("%s токен добавлен.\nID: %d\nТокен: %s", title, item.ID, maskToken(item.Token)))
		})
}

func integrationTokenEditDialog(deps Dependencies, provider, title string) *elumbot.Dialog {
	return elumbot.NewDialog().
		Ask(keyIntegrationTokenValue, fmt.Sprintf("Введите новый %s token:", title), elumbot.WithValidator(validateTokenValue)).
		OnFinish(func(dc elumbot.DialogContext) error {
			store, err := requireStore(deps)
			if err != nil {
				return err
			}
			id, ok := readMandatoryRewardRowID(dc.Value(keyIntegrationTokenID))
			if !ok {
				return dc.Reply("Некорректный id токена.")
			}
			token := strings.TrimSpace(dc.Input(keyIntegrationTokenValue).Text)
			updated, err := store.IntegrationTokens.Update(dc.Context(), provider, id, token)
			if err != nil {
				return err
			}
			if !updated {
				return dc.Reply("Токен не найден.")
			}
			invalidateIntegrationCache(deps, provider)
			return dc.Reply(fmt.Sprintf("%s токен обновлен.\nID: %d\nТокен: %s", title, id, maskToken(token)))
		})
}
