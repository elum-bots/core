package bot

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithStateStoreAndNilAdapterPaths(t *testing.T) {
	s := NewMemoryStore()
	b := New(nil, WithStateStore(s), WithMaxConcurrentHandlers(2))
	if b.store != s {
		t.Fatalf("store option not applied")
	}
	if b.pool == nil {
		t.Fatalf("pool is nil")
	}
	b.OnError(nil)
	if err := b.Run(context.Background()); !errors.Is(err, ErrUnsupportedFeature) {
		t.Fatalf("expected unsupported, got %v", err)
	}
	if err := b.Stop(context.Background()); err != nil {
		t.Fatalf("stop nil adapter err: %v", err)
	}
}

func TestMaxConcurrentHandlersLimit(t *testing.T) {
	b := New(&testAdapter{}, WithMaxConcurrentHandlers(2))
	var inFlight atomic.Int32
	var peak atomic.Int32

	b.Event("message", func(context.Context, ...string) error {
		cur := inFlight.Add(1)
		for {
			prev := peak.Load()
			if cur <= prev || peak.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(15 * time.Millisecond)
		inFlight.Add(-1)
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "x"}})
		}()
	}
	wg.Wait()

	if peak.Load() > 2 {
		t.Fatalf("limit exceeded: peak=%d", peak.Load())
	}
}

func TestBotSendWithOptionsAndMediaHelpers(t *testing.T) {
	a := &testAdapter{}
	b := New(a)
	err := b.Send(
		context.Background(),
		"chat",
		"hello",
		WithButtons(Row(Btn("1", "One"))),
		WithMedia(Image("./a.png"), Video("./b.mp4"), Audio("./c.ogg")),
	)
	if err != nil {
		t.Fatalf("send err: %v", err)
	}
	if len(a.sent) != 1 || len(a.sent[0].Buttons) != 1 || len(a.sent[0].Media) != 3 {
		t.Fatalf("send options not applied: %#v", a.sent)
	}
}

func TestIntRangeParseError(t *testing.T) {
	if err := IntRange(1, 2)(Input{Text: "abc"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestDialogContextSetGetHelpers(t *testing.T) {
	a := &testAdapter{}
	b := New(a)
	b.Event("start", func(ctx context.Context, _ ...string) error { return b.StartDialog(ctx, "ctx", nil) })
	b.Dialog("ctx", func() *Dialog {
		return NewDialog().
			Send("go", WithButtons(Row(Btn("ok", "OK")))).
			OnClick("ok", func(dc DialogContext) *Dialog {
				dc.Set("name", "Ann")
				dc.Set("age", "22")
				if dc.MustString("name") != "Ann" || dc.MustInt("age") != 22 {
					t.Fatalf("helpers failed")
				}
				return nil
			}).
			OnFinish(func(dc DialogContext) error {
				return dc.Reply("done")
			})
	})
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", Message: Message{Text: "/start"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.Dispatch(context.Background(), Update{Platform: "tg", ChatID: "c", UserID: "u", ButtonID: "ok", Message: Message{Kind: MessageKindOther}}); err != nil {
		t.Fatal(err)
	}
	if got := a.sent[len(a.sent)-1].Text; got != "done" {
		t.Fatalf("unexpected finish msg: %q", got)
	}
}
