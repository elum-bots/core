package bot

import (
	"context"
	"errors"
	"testing"
)

func TestDialogAskStoresInputAndFinish(t *testing.T) {
	a := &testAdapter{}
	b := New(a)

	b.Event("start", func(ctx context.Context, _ ...string) error {
		return b.StartDialog(ctx, "profile", nil)
	})

	var captured Input
	b.Dialog("profile", func() *Dialog {
		return NewDialog().
			Ask("name", "Введите имя").
			Ask("age", "Введите возраст", WithValidator(IntRange(1, 120))).
			OnFinish(func(dc DialogContext) error {
				captured = dc.Input("name")
				return dc.Reply("done")
			})
	})

	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "/start"}}); err != nil {
		t.Fatalf("start err: %v", err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "Введите имя" {
		t.Fatalf("unexpected first prompt: %q", got)
	}

	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "Иван", Kind: MessageKindFile, Attachments: []Attachment{{ID: "f"}}}}); err != nil {
		t.Fatalf("name err: %v", err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "Введите возраст" {
		t.Fatalf("unexpected second prompt: %q", got)
	}

	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "abc"}}); err != nil {
		t.Fatalf("invalid age err: %v", err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "Некорректный ввод, попробуйте еще раз" {
		t.Fatalf("expected validator msg, got %q", got)
	}

	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "35"}}); err != nil {
		t.Fatalf("age err: %v", err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "done" {
		t.Fatalf("unexpected finish msg: %q", got)
	}
	if captured.Text != "Иван" || captured.Kind != MessageKindFile {
		t.Fatalf("captured mismatch: %#v", captured)
	}
}

func TestDialogSendOnClickTransition(t *testing.T) {
	a := &testAdapter{}
	b := New(a)

	b.Event("go", func(ctx context.Context, _ ...string) error {
		return b.StartDialog(ctx, "vote", nil)
	})

	b.Dialog("vote", func() *Dialog {
		return NewDialog().
			Send("Выберите", WithButtons(Row(Btn("first", "Первая"), Btn("second", "Вторая")))).
			OnClick("first", func(ctx DialogContext) *Dialog {
				_ = ctx.Reply("Вы выбрали: Первая")
				return b.Dialog("confirm").WithValue("choice", "Первая").Ask("comment", "Комментарий")
			}).
			OnFinish(func(ctx DialogContext) error {
				return ctx.Reply("vote closed")
			})
	})

	b.Dialog("confirm", func() *Dialog {
		return NewDialog().
			OnFinish(func(ctx DialogContext) error {
				choice, _ := ctx.Value("choice").(string)
				in := ctx.Input("comment")
				return ctx.Reply("ok: " + choice + ":" + in.Text)
			})
	})

	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Event: "go"}); err != nil {
		t.Fatalf("start err: %v", err)
	}
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", ButtonID: "first", Message: Message{Kind: MessageKindOther}}); err != nil {
		t.Fatalf("click err: %v", err)
	}
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "x"}}); err != nil {
		t.Fatalf("comment err: %v", err)
	}

	if got := a.sent[len(a.sent)-1].Text; got != "ok: Первая:x" {
		t.Fatalf("unexpected final msg: %q", got)
	}
}

func TestStaleAndOutOfStepButtonsAreIgnored(t *testing.T) {
	a := &testAdapter{}
	b := New(a)
	b.Event("go", func(ctx context.Context, _ ...string) error { return b.StartDialog(ctx, "vote", nil) })
	b.Dialog("vote", func() *Dialog {
		return NewDialog().
			Send("Выберите", WithButtons(Row(Btn("first", "Первая")))).
			OnClick("first", func(ctx DialogContext) *Dialog {
				return b.Dialog("ask").Ask("comment", "Комментарий")
			})
	})
	b.Dialog("ask", func() *Dialog {
		return NewDialog().OnFinish(func(dc DialogContext) error { return dc.Reply("ok") })
	})

	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Event: "go"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", ButtonID: "first", Message: Message{Kind: MessageKindOther}}); err != nil {
		t.Fatal(err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "Комментарий" {
		t.Fatalf("expected ask prompt, got %q", got)
	}

	// Old button from previous step must be ignored.
	before := len(a.sent)
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", ButtonID: "first", Message: Message{Kind: MessageKindOther}}); err != nil {
		t.Fatal(err)
	}
	if len(a.sent) != before {
		t.Fatalf("stale button should be ignored")
	}

	// Finish dialog.
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "x"}}); err != nil {
		t.Fatal(err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "ok" {
		t.Fatalf("unexpected final msg: %q", got)
	}

	// After completion button must not work.
	before = len(a.sent)
	err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", ButtonID: "first", Message: Message{Kind: MessageKindOther}})
	if !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("expected unknown event after dialog end, got %v", err)
	}
	if len(a.sent) != before {
		t.Fatalf("button after end should not send anything")
	}
}

func TestDialogCloseBackAndCommandBlock(t *testing.T) {
	a := &testAdapter{}
	b := New(a)
	b.Event("start", func(ctx context.Context, _ ...string) error { return b.StartDialog(ctx, "flow", nil) })
	b.Dialog("flow", func() *Dialog {
		return NewDialog().
			Ask("name", "Введите имя").
			Ask("surname", "Введите фамилию").
			OnFinish(func(dc DialogContext) error { return dc.Reply("done") })
	})

	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "/start"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "/stats"}}); err != nil {
		t.Fatal(err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "Во время диалога доступны только /back и /close" {
		t.Fatalf("unexpected blocked msg: %q", got)
	}
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "Иван"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "/back"}}); err != nil {
		t.Fatal(err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "Введите имя" {
		t.Fatalf("expected back to first ask, got %q", got)
	}
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "/close"}}); err != nil {
		t.Fatal(err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "Диалог закрыт" {
		t.Fatalf("unexpected close msg: %q", got)
	}
}

func TestStartDialogErrorsAndMissingSessionDialog(t *testing.T) {
	b := New(&testAdapter{})
	if err := b.StartDialog(context.Background(), "missing", nil); !errors.Is(err, ErrDialogNotFound) {
		t.Fatalf("expected ErrDialogNotFound, got %v", err)
	}

	b.Dialog("x", func() *Dialog { return NewDialog().Send("x") })
	if err := b.StartDialog(context.Background(), "x", nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	key := dialogSessionKey(Update{Platform: "tg", ChatID: "c", UserID: "u"})
	b.saveSession(key, &dialogSession{Frames: []dialogFrame{{Dialog: "missing", Values: map[string]any{}}}})
	err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "x"}})
	if !errors.Is(err, ErrDialogNotFound) {
		t.Fatalf("expected ErrDialogNotFound, got %v", err)
	}
}
