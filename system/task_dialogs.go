package system

import (
	"database/sql"
	"fmt"
	"strings"

	elumbot "github.com/elum-bots/core/internal/bot"
)

func registerTaskDialogs(b *elumbot.Bot, deps Dependencies) {
	b.Dialog(dialogTaskCreate, func() *elumbot.Dialog {
		return elumbot.NewDialog().
			Ask(keyTaskReward, "Создание задания.\n\nШаг 1/2: введите награду (целое число > 0):", elumbot.WithValidator(validatePositiveInt64)).
			Ask(keyTaskChannelsRaw, "Шаг 2/2: введите каналы задания, каждый с новой строки:\nChannelID|Название|URL|Проверять(опционально 1/0)\nЕсли 4-е поле не указано, используется 1.", elumbot.WithValidator(validateTaskChannels)).
			OnFinish(func(dc elumbot.DialogContext) error {
				store, err := requireStore(deps)
				if err != nil {
					return err
				}
				reward, ok := parsePositiveInt64(dc.Input(keyTaskReward).Text)
				if !ok {
					return dc.Reply("Введите целое число больше 0.")
				}
				channels, err := parseTaskChannelsInput(dc.Input(keyTaskChannelsRaw).Text)
				if err != nil {
					return dc.Reply(fmt.Sprintf("Ошибка формата каналов: %v", err))
				}
				task, err := store.Tasks.Create(dc.Context(), reward, channels)
				if err != nil {
					return err
				}
				return dc.Reply(fmt.Sprintf("Задание создано.\nID: %d\nНаграда: %d\nКаналов: %d", task.ID, task.Reward, len(task.Channels)))
			})
	})

	b.Dialog(dialogTaskEdit, func() *elumbot.Dialog {
		return elumbot.NewDialog().
			Ask(keyTaskReward, "Изменение задания.\n\nШаг 1/2: введите новую награду или /empty, чтобы оставить текущую.", elumbot.WithValidator(validateTaskRewardOptional)).
			Ask(keyTaskChannelsRaw, "Шаг 2/2: введите новый список каналов или /empty, чтобы оставить текущий.\n\nФормат строки:\nChannelID|Название|URL|Проверять(опционально 1/0)", elumbot.WithValidator(validateTaskChannelsOptional)).
			OnFinish(func(dc elumbot.DialogContext) error {
				store, err := requireStore(deps)
				if err != nil {
					return err
				}
				taskID, ok := readMandatoryRewardRowID(dc.Value(keyTaskID))
				if !ok {
					return dc.Reply("Не удалось определить ID задания.")
				}
				current, err := store.Tasks.Get(dc.Context(), taskID)
				if err != nil {
					if sqlNotFound(err) {
						return dc.Reply("Задание не найдено.")
					}
					return err
				}

				reward := current.Reward
				if raw := strings.TrimSpace(dc.Input(keyTaskReward).Text); raw != "" && !isEmptyToken(raw) {
					value, ok := parsePositiveInt64(raw)
					if !ok {
						return dc.Reply("Введите целое число больше 0.")
					}
					reward = value
				}

				channels := current.Channels
				if raw := strings.TrimSpace(dc.Input(keyTaskChannelsRaw).Text); raw != "" && !isEmptyToken(raw) {
					parsed, err := parseTaskChannelsInput(raw)
					if err != nil {
						return dc.Reply(fmt.Sprintf("Ошибка формата каналов: %v", err))
					}
					channels = parsed
				}

				task, err := store.Tasks.Update(dc.Context(), taskID, reward, channels)
				if err != nil {
					if err == sql.ErrNoRows {
						return dc.Reply("Задание не найдено.")
					}
					return err
				}
				return dc.Reply(fmt.Sprintf("Задание обновлено.\nID: %d\nНаграда: %d\nКаналов: %d", task.ID, task.Reward, len(task.Channels)))
			})
	})
}
