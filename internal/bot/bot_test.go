package bot

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type testAdapter struct {
	started  bool
	stopped  bool
	dispatch Dispatcher
	sent     []OutMessage
	chatIDs  []string
}

func (a *testAdapter) Name() string { return "test" }
func (a *testAdapter) Start(_ context.Context, d Dispatcher) error {
	a.started = true
	a.dispatch = d
	return nil
}
func (a *testAdapter) Stop(context.Context) error {
	a.stopped = true
	return nil
}
func (a *testAdapter) Send(_ context.Context, chatID string, msg OutMessage) error {
	a.chatIDs = append(a.chatIDs, chatID)
	a.sent = append(a.sent, msg)
	return nil
}

func TestRunAndStop(t *testing.T) {
	a := &testAdapter{}
	b := New(a)
	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("run err: %v", err)
	}
	if !a.started {
		t.Fatalf("adapter not started")
	}
	if err := b.Stop(context.Background()); err != nil {
		t.Fatalf("stop err: %v", err)
	}
	if !a.stopped {
		t.Fatalf("adapter not stopped")
	}
}

func TestDispatchCommandAndReply(t *testing.T) {
	a := &testAdapter{}
	b := New(a)
	called := false
	b.Event("stats", func(ctx context.Context, args ...string) error {
		called = true
		if !reflect.DeepEqual(args, []string{"one", "two"}) {
			t.Fatalf("args mismatch: %#v", args)
		}
		return Reply(ctx, "ok")
	})

	err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c1", UserID: "u1", Message: Message{Text: "/stats one two"}})
	if err != nil {
		t.Fatalf("dispatch err: %v", err)
	}
	if !called {
		t.Fatalf("handler not called")
	}
	if len(a.sent) != 1 || a.sent[0].Text != "ok" {
		t.Fatalf("unexpected send: %#v", a.sent)
	}
}

func TestDispatchFallback(t *testing.T) {
	a := &testAdapter{}
	b := New(a)
	called := false
	b.Event("fallback", func(context.Context, ...string) error {
		called = true
		return nil
	})
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u"}); err != nil {
		t.Fatalf("dispatch err: %v", err)
	}
	if !called {
		t.Fatalf("fallback not called")
	}
}

func TestDispatchUnknownEvent(t *testing.T) {
	b := New(&testAdapter{})
	err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "hi"}})
	if !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("expected ErrUnknownEvent, got %v", err)
	}
}

func TestMiddlewareOrder(t *testing.T) {
	b := New(&testAdapter{})
	chain := []string{}
	b.Use(func(next EventHandler) EventHandler {
		return func(ctx context.Context, args ...string) error {
			chain = append(chain, "mw1-before")
			err := next(ctx, args...)
			chain = append(chain, "mw1-after")
			return err
		}
	})
	b.Use(func(next EventHandler) EventHandler {
		return func(ctx context.Context, args ...string) error {
			chain = append(chain, "mw2-before")
			err := next(ctx, args...)
			chain = append(chain, "mw2-after")
			return err
		}
	})
	b.Event("message", func(context.Context, ...string) error {
		chain = append(chain, "handler")
		return nil
	})
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "hello"}}); err != nil {
		t.Fatalf("dispatch err: %v", err)
	}
	exp := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if !reflect.DeepEqual(chain, exp) {
		t.Fatalf("order mismatch: %#v", chain)
	}
}

func TestOnErrorCalled(t *testing.T) {
	b := New(&testAdapter{})
	var got error
	b.OnError(func(_ context.Context, err error) { got = err })
	exp := errors.New("boom")
	b.Event("message", func(context.Context, ...string) error { return exp })
	err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "hello"}})
	if !errors.Is(err, exp) {
		t.Fatalf("expected boom err, got %v", err)
	}
	if !errors.Is(got, exp) {
		t.Fatalf("onError not called")
	}
}

func TestReplyWithoutRuntime(t *testing.T) {
	if err := Reply(context.Background(), "x"); !errors.Is(err, ErrUnsupportedFeature) {
		t.Fatalf("expected ErrUnsupportedFeature, got %v", err)
	}
}

func TestSendHelperNilBot(t *testing.T) {
	if err := Send(context.Background(), nil, "1", "x"); !errors.Is(err, ErrUnsupportedFeature) {
		t.Fatalf("expected unsupported, got %v", err)
	}
}

func TestWithButtonsOption(t *testing.T) {
	msg := OutMessage{Text: "x"}
	WithButtons(Row(Btn("a", "A"), Btn("b", "B")))(&msg)
	if len(msg.Buttons) != 1 || len(msg.Buttons[0]) != 2 {
		t.Fatalf("buttons not set: %#v", msg.Buttons)
	}
}
