package system

import (
	"context"
	"fmt"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerMandatoryDialogs(b *elumbot.Bot, deps Dependencies) {
	b.Dialog(dialogMandatoryAdd, func() *elumbot.Dialog {
		return elumbot.NewDialog().
			Ask(keyMandatoryChannelID, "Добавление обязательного канала.\n\nШаг 1/4: введите ID канала:", elumbot.WithValidator(validateNonEmpty)).
			Ask(keyMandatoryChannelTitle, "Шаг 2/4: введите название канала:", elumbot.WithValidator(validateNonEmpty)).
			Ask(keyMandatoryChannelURL, "Шаг 3/4: введите URL канала:", elumbot.WithValidator(validateNonEmpty)).
			Ask(keyMandatoryRequiresCheck, "Шаг 4/4: проводить проверку членства?\n\nВведите:\n- 1 или да — проверять\n- 0 или нет — не проверять", elumbot.WithValidator(validateCheckFlag)).
			OnFinish(func(dc elumbot.DialogContext) error {
				store, err := requireStore(deps)
				if err != nil {
					return err
				}
				channelID := strings.TrimSpace(dc.Input(keyMandatoryChannelID).Text)
				title := strings.TrimSpace(dc.Input(keyMandatoryChannelTitle).Text)
				link := strings.TrimSpace(dc.Input(keyMandatoryChannelURL).Text)
				check, _ := parseCheckFlag(dc.Input(keyMandatoryRequiresCheck).Text)
				item, err := store.Mandatory.Create(dc.Context(), channelID, title, link, check)
				if err != nil {
					return err
				}
				checkText := "да (1)"
				if !item.RequiresCheck {
					checkText = "нет (0)"
				}
				return dc.Reply(fmt.Sprintf("Обязательный канал добавлен.\nrow_id: %d\nID: %s\nНазвание: %s\nURL: %s\nПроверка: %s", item.ID, item.ChannelID, item.Title, item.URL, checkText))
			})
	})

	b.Dialog(dialogMandatoryRewardSelect, func() *elumbot.Dialog {
		return elumbot.NewDialog().
			Ask(keyMandatoryRewardRowID, "Выберите обязательный канал для mandatory_reward.\nВведите row_id из списка:", elumbot.WithValidator(validatePositiveInt64)).
			Ask(keyMandatoryRewardReset, "Сбросить предыдущий прогресс рассылки по этому каналу?\n\nВведите:\n- 1 или да — сбросить\n- 0 или нет — оставить прогресс", elumbot.WithValidator(validateCheckFlag)).
			OnFinish(func(dc elumbot.DialogContext) error {
				rowID, ok := parsePositiveInt64(dc.Input(keyMandatoryRewardRowID).Text)
				if !ok {
					return dc.Reply("Введите корректный row_id канала (целое число > 0).")
				}
				reset, _ := parseCheckFlag(dc.Input(keyMandatoryRewardReset).Text)
				return launchMandatoryReward(dc, b, deps, rowID, reset)
			})
	})

	b.Dialog(dialogMandatoryRewardReset, func() *elumbot.Dialog {
		return elumbot.NewDialog().
			Ask(keyMandatoryRewardReset, "Сбросить предыдущий прогресс рассылки по этому каналу?\n\nВведите:\n- 1 или да — сбросить\n- 0 или нет — оставить прогресс", elumbot.WithValidator(validateCheckFlag)).
			OnFinish(func(dc elumbot.DialogContext) error {
				rowID, ok := readMandatoryRewardRowID(dc.Value(keyMandatoryRewardRowID))
				if !ok {
					return dc.Reply("Введите корректный row_id канала (целое число > 0).")
				}
				reset, _ := parseCheckFlag(dc.Input(keyMandatoryRewardReset).Text)
				return launchMandatoryReward(dc, b, deps, rowID, reset)
			})
	})
}

func registerPostDialogs(b *elumbot.Bot, deps Dependencies) {
	b.Dialog(dialogPostCreate, func() *elumbot.Dialog {
		return elumbot.NewDialog().
			Ask(keyPostTitle, "Введите заголовок поста:", elumbot.WithValidator(validatePostTitle)).
			Ask(keyPostText, "Введите текст поста. Если без текста, отправьте /empty.", elumbot.WithValidator(validatePostText)).
			Ask(keyPostPhoto, "Отправьте фото для поста или /empty без фото.", elumbot.WithValidator(validatePostPhoto)).
			Ask(keyPostButtons, "Введите кнопки (каждая с новой строки):\nТекст|URL\nИли /empty без кнопок.", elumbot.WithValidator(validatePostButtons)).
			OnFinish(func(dc elumbot.DialogContext) error {
				store, err := requireStore(deps)
				if err != nil {
					return err
				}
				upd, err := currentUpdate(dc.Context())
				if err != nil {
					return dc.Reply("context error")
				}

				title := strings.TrimSpace(dc.Input(keyPostTitle).Text)
				body := strings.TrimSpace(dc.Input(keyPostText).Text)
				if isEmptyToken(body) {
					body = ""
				}
				text := postBodyWithTitle(title, body)
				mediaID, mediaKind := parsePostMedia(dc.Input(keyPostPhoto))
				buttons, err := parsePostButtonsRaw(dc.Input(keyPostButtons).Text)
				if err != nil {
					return dc.Reply("Некорректный формат кнопок.\nИспользуйте:\nТекст|URL\nИли /empty без кнопок.")
				}
				post, err := store.Posts.Create(dc.Context(), title, text, mediaID, mediaKind, buttons, upd.UserID)
				if err != nil {
					return err
				}
				return dc.Reply(fmt.Sprintf("Пост создан.\nID: %d\nЗаголовок: %s", post.ID, post.Title))
			})
	})
}

func readMandatoryRewardRowID(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, t > 0
	case int:
		return int64(t), t > 0
	case string:
		return parsePositiveInt64(t)
	default:
		return 0, false
	}
}

func touchUserForDialog(ctx context.Context, deps Dependencies, userID string) error {
	store, err := requireStore(deps)
	if err != nil {
		return err
	}
	if _, err := store.Users.Ensure(ctx, userID); err != nil {
		return err
	}
	return store.Users.Touch(ctx, userID)
}

func _unusedContext(_ context.Context) {}
