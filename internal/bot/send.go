package bot

import "context"

type runtimeKey struct{}

type runtimeData struct {
	bot    *Bot
	update Update
}

func withRuntime(ctx context.Context, data runtimeData) context.Context {
	return context.WithValue(ctx, runtimeKey{}, data)
}

func runtimeFrom(ctx context.Context) (runtimeData, bool) {
	v := ctx.Value(runtimeKey{})
	if v == nil {
		return runtimeData{}, false
	}
	rd, ok := v.(runtimeData)
	return rd, ok
}

func Reply(ctx context.Context, text string, opts ...SendOption) error {
	rd, ok := runtimeFrom(ctx)
	if !ok || rd.bot == nil {
		return ErrUnsupportedFeature
	}
	return rd.bot.Send(ctx, rd.update.ChatID, text, opts...)
}

func Send(ctx context.Context, b *Bot, chatID string, text string, opts ...SendOption) error {
	if b == nil {
		return ErrUnsupportedFeature
	}
	return b.Send(ctx, chatID, text, opts...)
}

func CurrentUpdate(ctx context.Context) (Update, bool) {
	rd, ok := runtimeFrom(ctx)
	if !ok {
		return Update{}, false
	}
	return rd.update, true
}
