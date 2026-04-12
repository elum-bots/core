package bot

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/elum-bots/core/internal/workers"
)

type Bot struct {
	adapter Adapter
	store   StateStore
	cfg     config
	pool    *workers.Pool
	sendFn  func(context.Context, string, OutMessage) error

	mu       sync.RWMutex
	handlers map[string]EventHandler
	mws      []Middleware
	dialogs  map[string]*DialogDef
	onError  func(ctx context.Context, err error)

	sessionsMu sync.Mutex
	sessions   map[string]*dialogSession
	workerPool *workers.Pool
}

func New(adapter Adapter, opts ...Option) *Bot {
	b := &Bot{
		adapter:  adapter,
		store:    NewMemoryStore(),
		cfg:      config{maxConcurrent: 1},
		handlers: make(map[string]EventHandler),
		dialogs:  make(map[string]*DialogDef),
		onError:  func(context.Context, error) {},
		sessions: make(map[string]*dialogSession),
	}
	for _, opt := range opts {
		opt(b)
	}
	if b.workerPool != nil {
		b.pool = b.workerPool
	} else if b.cfg.maxConcurrent > 0 {
		if p, err := workers.New(b.cfg.maxConcurrent); err == nil {
			b.pool = p
		}
	}
	return b
}

func (b *Bot) Event(name string, h EventHandler) *Bot {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, eventName := range eventAliases(name) {
		if prev, ok := b.handlers[eventName]; ok && shouldChainEvent(eventName) && prev != nil && h != nil {
			next := h
			b.handlers[eventName] = func(ctx context.Context, args ...string) error {
				if err := prev(withChainedNext(ctx), args...); err != nil {
					return err
				}
				return next(ctx, args...)
			}
			continue
		}
		b.handlers[eventName] = h
	}
	return b
}

func (b *Bot) Use(mw Middleware) *Bot {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mws = append(b.mws, mw)
	return b
}

func (b *Bot) OnError(h func(ctx context.Context, err error)) *Bot {
	if h == nil {
		return b
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onError = h
	return b
}

func (b *Bot) Run(ctx context.Context) error {
	if b.adapter == nil {
		return ErrUnsupportedFeature
	}
	return b.adapter.Start(ctx, b)
}

func (b *Bot) Stop(ctx context.Context) error {
	if b.pool != nil && b.pool != b.workerPool {
		b.pool.Release()
	}
	if b.adapter == nil {
		return nil
	}
	return b.adapter.Stop(ctx)
}

func (b *Bot) Send(ctx context.Context, chatID string, text string, opts ...SendOption) error {
	msg := OutMessage{Text: text}
	for _, opt := range opts {
		opt(&msg)
	}
	return b.SendMessage(ctx, chatID, msg)
}

func (b *Bot) SendMessage(ctx context.Context, chatID string, msg OutMessage) error {
	if b.sendFn != nil {
		return b.sendFn(ctx, chatID, msg)
	}
	return b.DirectSendMessage(ctx, chatID, msg)
}

func (b *Bot) DirectSendMessage(ctx context.Context, chatID string, msg OutMessage) error {
	if b.adapter == nil {
		return ErrUnsupportedFeature
	}
	return b.adapter.Send(ctx, chatID, msg)
}

func (b *Bot) SetSender(fn func(context.Context, string, OutMessage) error) {
	b.sendFn = fn
}

func (b *Bot) Dispatch(ctx context.Context, update Update) error {
	if b.pool != nil {
		done := make(chan error, 1)
		if err := b.pool.Submit(func() {
			done <- b.dispatchNow(ctx, update)
		}); err != nil {
			return err
		}
		return <-done
	}
	return b.dispatchNow(ctx, update)
}

func (b *Bot) Submit(task func()) error {
	if task == nil {
		return errors.New("nil task")
	}
	if b.pool != nil {
		return b.pool.Submit(task)
	}
	go task()
	return nil
}

func (b *Bot) dispatchNow(ctx context.Context, update Update) error {
	update = normalizeUpdate(update)

	if active, err := b.hasDialogSession(ctx, update); err != nil {
		b.handleErr(ctx, err)
	} else if active {
		// Any registered command can interrupt the current dialog.
		if shouldInterruptDialog(b, update) {
			b.deleteSession(dialogSessionKey(update))
		} else {
			if err := b.processDialog(ctx, update); err != nil {
				b.handleErr(ctx, err)
				return err
			}
			return nil
		}
	}

	eventNames, args := candidates(update)
	for _, name := range eventNames {
		h, ok := b.lookupHandler(name)
		if !ok {
			continue
		}
		rctx := withRuntime(ctx, runtimeData{bot: b, update: update})
		if err := h(rctx, args...); err != nil {
			b.handleErr(rctx, err)
			return err
		}
		return nil
	}
	if update.ButtonID != "" {
		if h, ok := b.lookupHandler("start"); ok {
			rctx := withRuntime(ctx, runtimeData{bot: b, update: update})
			if err := h(rctx); err != nil {
				b.handleErr(rctx, err)
				return err
			}
			return nil
		}
	}
	return ErrUnknownEvent
}

func shouldInterruptDialog(b *Bot, update Update) bool {
	if b == nil {
		return false
	}
	cmd := strings.TrimSpace(update.Command)
	if cmd == "" {
		return false
	}
	names := make([]string, 0, 3)
	if update.Event != "" {
		names = append(names, update.Event)
	}
	names = append(names, cmd, "command:"+cmd)
	for _, n := range uniq(names) {
		if _, ok := b.lookupHandler(n); ok {
			return true
		}
	}
	return false
}

func (b *Bot) lookupHandler(name string) (EventHandler, bool) {
	b.mu.RLock()
	h, ok := b.handlers[name]
	mws := append([]Middleware(nil), b.mws...)
	b.mu.RUnlock()
	if !ok {
		return nil, false
	}
	wrapped := h
	for i := len(mws) - 1; i >= 0; i-- {
		wrapped = mws[i](wrapped)
	}
	return wrapped, true
}

func (b *Bot) handleErr(ctx context.Context, err error) {
	b.mu.RLock()
	h := b.onError
	b.mu.RUnlock()
	if h != nil {
		h(ctx, err)
	}
}

func normalizeUpdate(u Update) Update {
	if u.Message.Kind == "" {
		u.Message.Kind = MessageKindText
	}
	if u.Command == "" {
		text := strings.TrimSpace(u.Message.Text)
		if strings.HasPrefix(text, "/") {
			parts := strings.Fields(strings.TrimPrefix(text, "/"))
			if len(parts) > 0 {
				u.Command = strings.ToLower(parts[0])
				if len(parts) > 1 {
					u.Args = append([]string(nil), parts[1:]...)
				}
			}
		}
	}
	if u.Event == "" && u.Command != "" {
		u.Event = "command:" + u.Command
	}
	return u
}

func candidates(u Update) ([]string, []string) {
	out := make([]string, 0, 6)
	if u.Event != "" {
		out = append(out, u.Event)
	}
	if u.Command != "" {
		out = append(out, u.Command, "command:"+u.Command)
	}
	if u.ButtonID != "" {
		out = append(out, "button:"+u.ButtonID)
	}
	out = append(out, "message", "fallback")
	return uniq(out), u.Args
}

func uniq(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func eventAliases(name string) []string {
	name = strings.TrimSpace(name)
	switch name {
	case "start", "command:start":
		return []string{"start", "command:start"}
	default:
		return []string{name}
	}
}

func shouldChainEvent(name string) bool {
	switch strings.TrimSpace(name) {
	case "start", "command:start":
		return true
	default:
		return false
	}
}

func dialogSessionKey(u Update) string {
	return "dlg:" + u.Platform + ":" + u.ChatID + ":" + u.UserID
}
