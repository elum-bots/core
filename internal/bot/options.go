package bot

import "github.com/elum-bots/core/internal/workers"

type Option func(*Bot)

type config struct {
	maxConcurrent int
}

func WithStateStore(store StateStore) Option {
	return func(b *Bot) {
		if store != nil {
			b.store = store
		}
	}
}

func WithMaxConcurrentHandlers(n int) Option {
	return func(b *Bot) {
		if n > 0 {
			b.cfg.maxConcurrent = n
		}
	}
}

func WithWorkerPool(p *workers.Pool) Option {
	return func(b *Bot) {
		b.workerPool = p
	}
}
