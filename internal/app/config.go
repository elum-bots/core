package app

import (
	"strings"

	"github.com/elum-utils/env"
)

type Config struct {
	Platform              string
	TGBotToken            string
	TGPollTimeoutSec      int
	TGPollLimit           int
	MAXBotToken           string
	MAXHTTPTimeoutSec     int
	BotMaxConcurrent      int
	SQLitePath            string
	HTTPAddr              string
	WebhookSecret         string
	WebhookPublicURL      string
	WebhookAutoRegister   bool
	WebhookUpdateTypes    []string
	FeaturePayments       bool
	FeatureAimini         bool
	FeatureDeepSeek       bool
	FeatureGemini         bool
	PlategaBaseURL        string
	PlategaMerchantID     string
	PlategaSecret         string
	PlategaPaymentMethod  int
	PlategaReturnURL      string
	PlategaFailedURL      string
	AiminiBaseURL         string
	AiminiNodeID          string
	AiminiProxyURL        string
	AiminiTimeoutSec      int
	AiminiPollIntervalSec int
	DeepSeekModel         string
	DeepSeekBaseURL       string
	DeepSeekProxyURL      string
	DeepSeekTimeoutSec    int
	GeminiModel           string
	GeminiProxyURL        string
	GeminiTimeoutSec      int
}

func LoadConfig() Config {
	cfg := Config{
		Platform:              strings.ToLower(strings.TrimSpace(env.GetEnvString("BOT_PLATFORM", "tg"))),
		TGBotToken:            strings.TrimSpace(env.GetEnvString("TG_BOT_TOKEN", "")),
		TGPollTimeoutSec:      env.GetEnvInt("TG_POLL_TIMEOUT_SEC", 30),
		TGPollLimit:           env.GetEnvInt("TG_POLL_LIMIT", 100),
		MAXBotToken:           strings.TrimSpace(env.GetEnvString("MAX_BOT_TOKEN", "")),
		MAXHTTPTimeoutSec:     env.GetEnvInt("MAX_HTTP_TIMEOUT_SEC", 120),
		BotMaxConcurrent:      env.GetEnvInt("BOT_MAX_CONCURRENT", 50),
		SQLitePath:            strings.TrimSpace(env.GetEnvString("SQLITE_PATH", "./data/bot.sqlite")),
		HTTPAddr:              strings.TrimSpace(env.GetEnvString("HTTP_ADDR", ":8080")),
		WebhookSecret:         strings.TrimSpace(env.GetEnvString("WEBHOOK_SECRET", "")),
		WebhookPublicURL:      strings.TrimSpace(env.GetEnvString("WEBHOOK_PUBLIC_URL", "")),
		WebhookAutoRegister:   env.GetEnvBool("WEBHOOK_AUTO_REGISTER", false),
		FeaturePayments:       env.GetEnvBool("FEATURE_PAYMENTS", false),
		FeatureAimini:         env.GetEnvBool("FEATURE_AIMINI", false),
		FeatureDeepSeek:       env.GetEnvBool("FEATURE_DEEPSEEK", false),
		FeatureGemini:         env.GetEnvBool("FEATURE_GEMINI", false),
		PlategaBaseURL:        strings.TrimSpace(env.GetEnvString("PLATEGA_BASE_URL", "")),
		PlategaMerchantID:     strings.TrimSpace(env.GetEnvString("PLATEGA_MERCHANT_ID", "")),
		PlategaSecret:         strings.TrimSpace(env.GetEnvString("PLATEGA_SECRET", "")),
		PlategaPaymentMethod:  env.GetEnvInt("PLATEGA_PAYMENT_METHOD", 2),
		PlategaReturnURL:      strings.TrimSpace(env.GetEnvString("PLATEGA_RETURN_URL", "")),
		PlategaFailedURL:      strings.TrimSpace(env.GetEnvString("PLATEGA_FAILED_URL", "")),
		AiminiBaseURL:         strings.TrimSpace(env.GetEnvString("AIMINI_BASE_URL", "")),
		AiminiNodeID:          strings.TrimSpace(env.GetEnvString("AIMINI_NODE_ID", "")),
		AiminiProxyURL:        strings.TrimSpace(env.GetEnvString("AIMINI_PROXY_URL", "")),
		AiminiTimeoutSec:      env.GetEnvInt("AIMINI_TIMEOUT_SEC", 180),
		AiminiPollIntervalSec: env.GetEnvInt("AIMINI_POLL_INTERVAL_SEC", 5),
		DeepSeekModel:         strings.TrimSpace(env.GetEnvString("DEEPSEEK_MODEL", "deepseek-chat")),
		DeepSeekBaseURL:       strings.TrimSpace(env.GetEnvString("DEEPSEEK_BASE_URL", "https://api.deepseek.com")),
		DeepSeekProxyURL:      strings.TrimSpace(env.GetEnvString("DEEPSEEK_PROXY_URL", "")),
		DeepSeekTimeoutSec:    env.GetEnvInt("DEEPSEEK_TIMEOUT_SEC", 60),
		GeminiModel:           strings.TrimSpace(env.GetEnvString("GEMINI_MODEL", "gemini-2.5-flash-image")),
		GeminiProxyURL:        strings.TrimSpace(env.GetEnvString("GEMINI_PROXY_URL", "")),
		GeminiTimeoutSec:      env.GetEnvInt("GEMINI_TIMEOUT_SEC", 180),
		WebhookUpdateTypes: normalizeWebhookTypes(
			env.GetEnvArrayString(
				"WEBHOOK_UPDATE_TYPES",
				",",
				[]string{"message_created", "message_callback", "bot_started"},
			),
		),
	}

	if cfg.Platform == "" {
		cfg.Platform = "tg"
	}

	return cfg
}

func normalizeWebhookTypes(in []string) []string {
	out := make([]string, 0, len(in))
	for _, part := range in {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return []string{"message_created", "message_callback", "bot_started"}
	}
	return out
}
