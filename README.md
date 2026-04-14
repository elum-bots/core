# Bot Core Package

`bot` — переиспользуемый core-пакет для новых ботов.
Он содержит рантайм, системные команды, SQLite, integrations, payments и базовые сервисы.

Готовый стартовый проект поверх него лежит в [example](/Volumes/CLOUD/GitHub/GMELUM/elum-bot/bot/example).

## Что экспортирует пакет

- `Run(ctx, opts...)` — запуск бота
- `WithCommands(...)` — подключение своих регистраторов команд
- `WithPaymentProducts(...)` — каталог товаров для платежей
- `WithStartupHooks(...)` — startup hooks для своей инициализации
- `WithPaymentSuccessHooks(...)` — hook-и после успешного начисления по оплате
- `WithReferralSuccessHooks(...)` — hook-и после успешной новой referral-привязки
- `WithHelpInfo(...)` — кастомный информационный блок для `/help`

Также наружу экспортированы типы и helper-ы для команд:

- `Bot`
- `Dependencies`
- `Reply`, `Send`, `CurrentUpdate`
- `ImageBytesFromUpdate`, `ImageBytesFromAttachment`
- `IsSubscribedToChannel`
- `NewDialog`, `WithValidator`, `IntRange`
- `Btn`, `URLBtn`, `Row`, `WithButtons`
- `Image`, `Video`, `Audio`, `File`, `WithMedia`

## Что входит в core

- системные команды из `system/`
- рантайм платформ `tg` и `max`
- локальные adapters `internal/bot/adapters/tg` и `internal/bot/adapters/max`
- SQLite store и migrations
- posts, mandatory, track, referrals, stats, broadcasts
- автопродолжение системных рассылок после рестарта
- досрочная остановка активной рассылки через `/broadcast_stop <id>`
- tasks с каналами и наградами
- payments и Platega callback
- integrations: DeepSeek, Gemini, TG, MAX
- TG helpers: `getFile`, `getChatMember`, download by `file_id`

## DeepSeek

DeepSeek включается не токеном из `env`, а feature-флагом:

- `FEATURE_DEEPSEEK=true`
- `DEEPSEEK_PROXY_URL=http://host:port` при необходимости отдельного proxy только для DeepSeek

После включения токены добавляются уже из самого бота через админские команды.
Токены хранятся в SQLite и кешируются на 1 минуту.

## Gemini

Gemini включается не токеном из `env`, а feature-флагом:

- `FEATURE_GEMINI=true`
- `GEMINI_PROXY_URL=http://host:port` при необходимости отдельного proxy только для Gemini

После включения токены добавляются уже из самого бота через админские команды.
Токены хранятся в SQLite и кешируются на 1 минуту.

## Как использовать core

Пример из [main.go](/Volumes/CLOUD/GitHub/GMELUM/elum-bot/bot/example/main.go):

```go
corebot.Run(
	ctx,
	corebot.WithPaymentProducts(
		payments.MustCoinProduct("10_coins", "10 монет", 100),
	),
	corebot.WithPaymentSuccessHooks(onPaymentSuccess),
	corebot.WithReferralSuccessHooks(onReferralSuccess),
	corebot.WithCommands(commands.Register),
)
```

Регистратор команд выглядит так:

```go
func Register(b *corebot.Bot, deps corebot.Dependencies) {
	b.Event("ping", func(ctx context.Context, _ ...string) error {
		return corebot.Reply(ctx, "pong")
	})
}
```

Пример бизнес-hook после оплаты:

```go
func onPaymentSuccess(ctx context.Context, b *corebot.Bot, deps corebot.Dependencies, tx corebot.PaymentTransaction) error {
	return corebot.Send(ctx, b, tx.PlatformUserID, "Оплата прошла успешно")
}
```

Пример бизнес-hook после referral:

```go
func onReferralSuccess(ctx context.Context, b *corebot.Bot, deps corebot.Dependencies, event corebot.ReferralSuccess) error {
	if event.GrantedCoins <= 0 {
		return nil
	}
	return corebot.Send(ctx, b, event.InviterUserID, "За приглашение начислены монеты")
}
```

Пример получения bytes входящего фото:

```go
upd, _ := corebot.CurrentUpdate(ctx)
photo, mimeType, err := corebot.ImageBytesFromUpdate(ctx, deps, upd)
_ = photo
_ = mimeType
_ = err
```

Пример общей проверки подписки:

```go
ok, err := corebot.IsSubscribedToChannel(ctx, deps, upd.Platform, upd.UserID, channelID)
_ = ok
_ = err
```

Важно для `/start`:

- core не отправляет никакого стартового текста сам
- core только обрабатывает системную часть `/start`: регистрацию пользователя, `ref_*`, `trk_*`, метрику start
- весь пользовательский ответ на `/start` должен регистрироваться в бизнес-боте через `b.Event("start", ...)`

## ENV

Core читает env только через `github.com/elum-utils/env`.
Формат и список переменных смотри в [`.env.example`](/Volumes/CLOUD/GitHub/GMELUM/elum-bot/bot/.env.example).

Основные группы:

- `Common`
- `Admins`
- `Telegram`
- `MAX`
- `DeepSeek`
- `Gemini`
- `Payments`

Отдельные proxy задаются только для AI-интеграций и не влияют на Telegram, MAX или Payments:

- `DEEPSEEK_PROXY_URL`
- `GEMINI_PROXY_URL`

Для MAX дополнительно можно управлять общим HTTP timeout:

- `MAX_HTTP_TIMEOUT_SEC=120`

Это влияет в том числе на upload медиа в MAX и полезно для больших сгенерированных изображений.

## Расширение

Если новому боту нужна своя инициализация на старте, используй `WithStartupHooks(...)`.
Это удобное место для:

- создания своих таблиц
- проверки внешних сервисов
- инициализации доменных зависимостей

Если боту нужен task-flow, core уже содержит общие команды:

- `/task_create`
- `/task_edit <id>`
- `/task_list`
- `/earn_more`
- `/task_done <id>`

Task-проверка подписок уже работает для `tg` и `max`, а свой mandatory/onboarding flow можно строить отдельно поверх этих базовых методов.

Если нужна новая общая функциональность для всех будущих ботов, добавляй ее в `bot`, а не в `example`.
