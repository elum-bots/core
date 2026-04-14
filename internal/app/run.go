package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	elumbot "github.com/elum-bots/core/internal/bot"
	maxadapter "github.com/elum-bots/core/internal/bot/adapters/max"
	tgadapter "github.com/elum-bots/core/internal/bot/adapters/tg"
	"github.com/elum-bots/core/internal/db"
	maxintegration "github.com/elum-bots/core/internal/integration/max"
	"github.com/elum-bots/core/system"
)

func Run(ctx context.Context, opts ...Option) error {
	cfg := LoadConfig()
	runtimeOpts := resolveOptions(opts)
	store, err := db.Open(ctx, cfg.SQLitePath)
	if err != nil {
		return err
	}
	defer store.Close()

	paymentSvc, err := newPaymentService(cfg, store, runtimeOpts)
	if err != nil {
		return err
	}

	integrationServices, err := newIntegrationServices(ctx, cfg, store)
	if err != nil {
		return err
	}

	deps := Dependencies{
		Store:        store,
		Payments:     paymentSvc,
		Integrations: integrationServices,
	}

	for _, hook := range runtimeOpts.startupHooks {
		if err := hook(ctx, deps); err != nil {
			return err
		}
	}

	switch cfg.Platform {
	case "tg", "telegram":
		return runTG(ctx, cfg, deps, runtimeOpts)
	case "max":
		return runMAX(ctx, cfg, deps, runtimeOpts)
	default:
		return fmt.Errorf("unsupported BOT_PLATFORM: %s", cfg.Platform)
	}
}

func runTG(
	ctx context.Context,
	cfg Config,
	deps Dependencies,
	opts options,
) error {
	if cfg.TGBotToken == "" {
		return errors.New("TG_BOT_TOKEN is empty")
	}

	adapter := tgadapter.NewAdapter(
		cfg.TGBotToken,
		tgadapter.WithPolling(cfg.TGPollTimeoutSec, cfg.TGPollLimit),
	)

	log.Printf("starting tg bot: poll_timeout=%d poll_limit=%d", cfg.TGPollTimeoutSec, cfg.TGPollLimit)
	b, err := newBot(ctx, adapter, cfg, deps, opts)
	if err != nil {
		return err
	}
	if deps.Payments == nil || !deps.Payments.IsEnabled() {
		return runBot(ctx, b)
	}

	log.Printf("starting payments callback server: addr=%s", cfg.HTTPAddr)
	return runBotWithServer(ctx, b, newHTTPServer(cfg, deps.Payments, nil, b, deps, opts))
}

func runMAX(
	ctx context.Context,
	cfg Config,
	deps Dependencies,
	opts options,
) error {
	if cfg.MAXBotToken == "" {
		return errors.New("MAX_BOT_TOKEN is empty")
	}

	if cfg.WebhookAutoRegister {
		if cfg.WebhookPublicURL == "" {
			return errors.New("WEBHOOK_PUBLIC_URL is required when WEBHOOK_AUTO_REGISTER=true")
		}

		client, err := maxintegration.NewClient(cfg.MAXBotToken)
		if err != nil {
			return err
		}
		if err := client.EnsureWebhookSubscription(
			ctx,
			cfg.WebhookPublicURL,
			cfg.WebhookUpdateTypes,
			cfg.WebhookSecret,
		); err != nil {
			return err
		}
	}

	adapter, err := maxadapter.NewAdapter(cfg.MAXBotToken)
	if err != nil {
		return err
	}
	maxadapter.WithHTTPTimeout(time.Duration(cfg.MAXHTTPTimeoutSec) * time.Second)(adapter)

	b, err := newBot(ctx, adapter, cfg, deps, opts)
	if err != nil {
		return err
	}
	log.Printf("starting max webhook server: addr=%s", cfg.HTTPAddr)
	return runBotWithServer(ctx, b, newHTTPServer(cfg, deps.Payments, adapter, b, deps, opts))
}

func newBot(
	ctx context.Context,
	adapter elumbot.Adapter,
	cfg Config,
	deps Dependencies,
	opts options,
) (*elumbot.Bot, error) {
	b := elumbot.New(
		adapter,
		elumbot.WithMaxConcurrentHandlers(cfg.BotMaxConcurrent),
	)
	b.OnError(func(_ context.Context, err error) {
		log.Printf("bot error: %v", err)
	})

	broadcasts := system.NewBroadcastRunner(ctx, b, deps.Store)
	system.Register(b, system.Dependencies{
		Store:         deps.Store,
		Integrations:  deps.Integrations,
		HelpInfo:      opts.helpInfo,
		ReferralHooks: systemReferralHooks(deps, opts),
		Broadcasts:    broadcasts,
	})
	for _, registrar := range opts.commandRegistrars {
		registrar(b, deps)
	}
	if err := broadcasts.Resume(ctx); err != nil {
		return nil, err
	}
	return b, nil
}

func systemReferralHooks(deps Dependencies, opts options) []system.ReferralSuccessHook {
	if len(opts.referralHooks) == 0 {
		return nil
	}
	hooks := make([]system.ReferralSuccessHook, 0, len(opts.referralHooks))
	for _, hook := range opts.referralHooks {
		hook := hook
		hooks = append(hooks, func(ctx context.Context, b *elumbot.Bot, _ system.Dependencies, event system.ReferralSuccess) error {
			return hook(ctx, b, deps, ReferralSuccess{
				InviterUserID:  event.InviterUserID,
				InvitedUserID:  event.InvitedUserID,
				GrantedCoins:   event.GrantedCoins,
				RewardProgress: event.RewardProgress,
			})
		})
	}
	return hooks
}

func runBot(ctx context.Context, b *elumbot.Bot) error {
	err := b.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func runBotWithServer(ctx context.Context, b *elumbot.Bot, srv *http.Server) error {
	errCh := make(chan error, 2)
	go func() {
		if err := runBot(ctx, b); err != nil {
			errCh <- err
		}
	}()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		_ = b.Stop(shutdownCtx)
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	if err := b.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
