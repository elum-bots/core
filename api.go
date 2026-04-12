package bot

import (
	"context"

	internalapp "github.com/elum-bots/core/internal/app"
	internalbot "github.com/elum-bots/core/internal/bot"
	internaldb "github.com/elum-bots/core/internal/db"
	integration "github.com/elum-bots/core/internal/integration"
	"github.com/elum-bots/core/payments"
)

type Bot = internalbot.Bot
type Dialog = internalbot.Dialog
type DialogContext = internalbot.DialogContext
type Update = internalbot.Update
type Input = internalbot.Input
type Attachment = internalbot.Attachment
type Message = internalbot.Message
type MessageKind = internalbot.MessageKind
type Button = internalbot.Button
type ButtonKind = internalbot.ButtonKind
type ButtonRow = internalbot.ButtonRow
type OutMessage = internalbot.OutMessage
type OutMedia = internalbot.OutMedia
type OutMediaKind = internalbot.OutMediaKind
type SendOption = internalbot.SendOption
type AskOption = internalbot.AskOption
type Validator = internalbot.Validator

type Store = internaldb.Store
type Integrations = integration.Services
type PaymentTransaction = internaldb.PaymentTransaction
type ReferralSuccess struct {
	InviterUserID  string
	InvitedUserID  string
	GrantedCoins   int64
	RewardProgress float64
}

type Dependencies struct {
	Store        *Store
	Payments     *payments.Service
	Integrations *Integrations
}

type CommandRegistrar func(*Bot, Dependencies)
type StartupHook func(context.Context, Dependencies) error
type PaymentSuccessHook func(context.Context, *Bot, Dependencies, PaymentTransaction) error
type ReferralSuccessHook func(context.Context, *Bot, Dependencies, ReferralSuccess) error

type Option func(*options)

type options struct {
	paymentProducts []payments.Product
	registrars      []CommandRegistrar
	startupHooks    []StartupHook
	paymentHooks    []PaymentSuccessHook
	referralHooks   []ReferralSuccessHook
	helpInfo        string
}

func WithPaymentProducts(products ...payments.Product) Option {
	return func(opts *options) {
		if opts == nil {
			return
		}
		opts.paymentProducts = append([]payments.Product(nil), products...)
	}
}

func WithCommands(registrars ...CommandRegistrar) Option {
	return func(opts *options) {
		if opts == nil {
			return
		}
		for _, registrar := range registrars {
			if registrar != nil {
				opts.registrars = append(opts.registrars, registrar)
			}
		}
	}
}

func WithStartupHooks(hooks ...StartupHook) Option {
	return func(opts *options) {
		if opts == nil {
			return
		}
		for _, hook := range hooks {
			if hook != nil {
				opts.startupHooks = append(opts.startupHooks, hook)
			}
		}
	}
}

func WithPaymentSuccessHooks(hooks ...PaymentSuccessHook) Option {
	return func(opts *options) {
		if opts == nil {
			return
		}
		for _, hook := range hooks {
			if hook != nil {
				opts.paymentHooks = append(opts.paymentHooks, hook)
			}
		}
	}
}

func WithReferralSuccessHooks(hooks ...ReferralSuccessHook) Option {
	return func(opts *options) {
		if opts == nil {
			return
		}
		for _, hook := range hooks {
			if hook != nil {
				opts.referralHooks = append(opts.referralHooks, hook)
			}
		}
	}
}

func WithHelpInfo(text string) Option {
	return func(opts *options) {
		if opts == nil {
			return
		}
		opts.helpInfo = text
	}
}

func Run(ctx context.Context, opts ...Option) error {
	resolved := resolveOptions(opts)
	appOpts := make([]internalapp.Option, 0, 4)

	if len(resolved.paymentProducts) > 0 {
		appOpts = append(appOpts, internalapp.WithPaymentProducts(resolved.paymentProducts...))
	}

	if len(resolved.registrars) > 0 {
		internalRegistrars := make([]internalapp.CommandRegistrar, 0, len(resolved.registrars))
		for _, registrar := range resolved.registrars {
			registrar := registrar
			internalRegistrars = append(internalRegistrars, func(b *internalbot.Bot, deps internalapp.Dependencies) {
				registrar(b, Dependencies{
					Store:        deps.Store,
					Payments:     deps.Payments,
					Integrations: deps.Integrations,
				})
			})
		}
		appOpts = append(appOpts, internalapp.WithCommandRegistrars(internalRegistrars...))
	}

	if len(resolved.startupHooks) > 0 {
		internalHooks := make([]internalapp.StartupHook, 0, len(resolved.startupHooks))
		for _, hook := range resolved.startupHooks {
			hook := hook
			internalHooks = append(internalHooks, func(ctx context.Context, deps internalapp.Dependencies) error {
				return hook(ctx, Dependencies{
					Store:        deps.Store,
					Payments:     deps.Payments,
					Integrations: deps.Integrations,
				})
			})
		}
		appOpts = append(appOpts, internalapp.WithStartupHooks(internalHooks...))
	}

	if len(resolved.paymentHooks) > 0 {
		internalHooks := make([]internalapp.PaymentSuccessHook, 0, len(resolved.paymentHooks))
		for _, hook := range resolved.paymentHooks {
			hook := hook
			internalHooks = append(internalHooks, func(ctx context.Context, b *internalbot.Bot, deps internalapp.Dependencies, tx internaldb.PaymentTransaction) error {
				return hook(ctx, b, Dependencies{
					Store:        deps.Store,
					Payments:     deps.Payments,
					Integrations: deps.Integrations,
				}, tx)
			})
		}
		appOpts = append(appOpts, internalapp.WithPaymentSuccessHooks(internalHooks...))
	}

	if len(resolved.referralHooks) > 0 {
		internalHooks := make([]internalapp.ReferralSuccessHook, 0, len(resolved.referralHooks))
		for _, hook := range resolved.referralHooks {
			hook := hook
			internalHooks = append(internalHooks, func(ctx context.Context, b *internalbot.Bot, deps internalapp.Dependencies, event internalapp.ReferralSuccess) error {
				return hook(ctx, b, Dependencies{
					Store:        deps.Store,
					Payments:     deps.Payments,
					Integrations: deps.Integrations,
				}, ReferralSuccess{
					InviterUserID:  event.InviterUserID,
					InvitedUserID:  event.InvitedUserID,
					GrantedCoins:   event.GrantedCoins,
					RewardProgress: event.RewardProgress,
				})
			})
		}
		appOpts = append(appOpts, internalapp.WithReferralSuccessHooks(internalHooks...))
	}

	if resolved.helpInfo != "" {
		appOpts = append(appOpts, internalapp.WithHelpInfo(resolved.helpInfo))
	}

	return internalapp.Run(ctx, appOpts...)
}

func resolveOptions(in []Option) options {
	var opts options
	for _, opt := range in {
		if opt != nil {
			opt(&opts)
		}
	}
	return opts
}
