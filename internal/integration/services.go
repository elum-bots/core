package integration

import (
	deepseekintegration "github.com/elum-bots/core/internal/integration/deepseek"
	geminiintegration "github.com/elum-bots/core/internal/integration/gemini"
	maxintegration "github.com/elum-bots/core/internal/integration/max"
	tgintegration "github.com/elum-bots/core/internal/integration/tg"
)

type Services struct {
	DeepSeek *deepseekintegration.Service
	Gemini   *geminiintegration.Service
	TG       *tgintegration.Client
	MAX      *maxintegration.Client
}
