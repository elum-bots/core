package app

import (
	"context"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
	integration "github.com/elum-bots/core/internal/integration"
	"github.com/elum-bots/core/payments"
)

type Option func(*options)

type Dependencies struct {
	Store        *db.Store
	Payments     *payments.Service
	Integrations *integration.Services
}

type CommandRegistrar func(*elumbot.Bot, Dependencies)
type StartupHook func(context.Context, Dependencies) error
type PaymentSuccessHook func(context.Context, *elumbot.Bot, Dependencies, db.PaymentTransaction) error
type ReferralSuccess struct {
	InviterUserID  string
	InvitedUserID  string
	GrantedCoins   int64
	RewardProgress float64
}
type ReferralSuccessHook func(context.Context, *elumbot.Bot, Dependencies, ReferralSuccess) error

type options struct {
	paymentProducts   []payments.Product
	commandRegistrars []CommandRegistrar
	startupHooks      []StartupHook
	paymentHooks      []PaymentSuccessHook
	referralHooks     []ReferralSuccessHook
	helpInfo          string
}

func WithPaymentProducts(products ...payments.Product) Option {
	return func(opts *options) {
		if opts == nil {
			return
		}
		opts.paymentProducts = append([]payments.Product(nil), products...)
	}
}

func WithCommandRegistrars(registrars ...CommandRegistrar) Option {
	return func(opts *options) {
		if opts == nil {
			return
		}
		for _, registrar := range registrars {
			if registrar != nil {
				opts.commandRegistrars = append(opts.commandRegistrars, registrar)
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

func resolveOptions(in []Option) options {
	var opts options
	for _, opt := range in {
		if opt != nil {
			opt(&opts)
		}
	}
	return opts
}
