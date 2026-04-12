package app

import (
	"context"
	"time"

	"github.com/elum-bots/core/internal/db"
	integration "github.com/elum-bots/core/internal/integration"
	deepseekintegration "github.com/elum-bots/core/internal/integration/deepseek"
	geminiintegration "github.com/elum-bots/core/internal/integration/gemini"
	maxintegration "github.com/elum-bots/core/internal/integration/max"
	tgintegration "github.com/elum-bots/core/internal/integration/tg"
)

func newIntegrationServices(ctx context.Context, cfg Config, store *db.Store) (*integration.Services, error) {
	services := &integration.Services{}

	if cfg.FeatureDeepSeek && store != nil {
		client, err := deepseekintegration.NewService(
			store.IntegrationTokens,
			cfg.DeepSeekModel,
			cfg.DeepSeekBaseURL,
			time.Duration(cfg.DeepSeekTimeoutSec)*time.Second,
			time.Minute,
		)
		if err != nil {
			return nil, err
		}
		services.DeepSeek = client
	}

	if cfg.FeatureGemini && store != nil {
		generator, err := geminiintegration.NewService(
			ctx,
			store.IntegrationTokens,
			cfg.GeminiModel,
			time.Duration(cfg.GeminiTimeoutSec)*time.Second,
			time.Minute,
		)
		if err != nil {
			return nil, err
		}
		services.Gemini = generator
	}

	if cfg.TGBotToken != "" {
		client, err := tgintegration.NewClient(cfg.TGBotToken)
		if err != nil {
			return nil, err
		}
		services.TG = client
	}

	if cfg.MAXBotToken != "" {
		client, err := maxintegration.NewClient(cfg.MAXBotToken)
		if err != nil {
			return nil, err
		}
		services.MAX = client
	}

	return services, nil
}
