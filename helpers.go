package bot

import (
	"context"
	"errors"

	internalbot "github.com/elum-bots/core/internal/bot"
	integration "github.com/elum-bots/core/internal/integration"
)

var (
	ErrUnknownEvent       = internalbot.ErrUnknownEvent
	ErrDialogNotFound     = internalbot.ErrDialogNotFound
	ErrInvalidInput       = internalbot.ErrInvalidInput
	ErrUnsupportedFeature = internalbot.ErrUnsupportedFeature
)

const (
	MessageKindText  = internalbot.MessageKindText
	MessageKindPhoto = internalbot.MessageKindPhoto
	MessageKindVideo = internalbot.MessageKindVideo
	MessageKindAudio = internalbot.MessageKindAudio
	MessageKindVoice = internalbot.MessageKindVoice
	MessageKindFile  = internalbot.MessageKindFile
	MessageKindOther = internalbot.MessageKindOther

	ButtonKindCallback = internalbot.ButtonKindCallback
	ButtonKindURL      = internalbot.ButtonKindURL

	OutMediaImage = internalbot.OutMediaImage
	OutMediaVideo = internalbot.OutMediaVideo
	OutMediaAudio = internalbot.OutMediaAudio
	OutMediaFile  = internalbot.OutMediaFile
)

func Reply(ctx context.Context, text string, opts ...SendOption) error {
	return internalbot.Reply(ctx, text, opts...)
}

func Send(ctx context.Context, b *Bot, chatID string, text string, opts ...SendOption) error {
	return internalbot.Send(ctx, b, chatID, text, opts...)
}

func CurrentUpdate(ctx context.Context) (Update, bool) {
	return internalbot.CurrentUpdate(ctx)
}

func ImageBytesFromAttachment(ctx context.Context, deps Dependencies, platform string, att Attachment) ([]byte, string, error) {
	if deps.Integrations == nil {
		return nil, "", errors.New("integrations are not initialized")
	}
	return integration.DownloadImageAttachment(ctx, deps.Integrations, platform, att)
}

func ImageBytesFromUpdate(ctx context.Context, deps Dependencies, upd Update) ([]byte, string, error) {
	if deps.Integrations == nil {
		return nil, "", errors.New("integrations are not initialized")
	}
	return integration.DownloadImageFromUpdate(ctx, deps.Integrations, upd)
}

func IsSubscribedToChannel(ctx context.Context, deps Dependencies, platform, platformUserID, channelID string) (bool, error) {
	if deps.Integrations == nil {
		return false, errors.New("integrations are not initialized")
	}
	return integration.IsSubscribedToChannel(ctx, deps.Integrations, platform, platformUserID, channelID)
}

func NewDialog() *Dialog {
	return internalbot.NewDialog()
}

func WithValidator(v Validator) AskOption {
	return internalbot.WithValidator(v)
}

func IntRange(min, max int) Validator {
	return internalbot.IntRange(min, max)
}

func Row(btns ...Button) ButtonRow {
	return internalbot.Row(btns...)
}

func Btn(id, text string) Button {
	return internalbot.Btn(id, text)
}

func URLBtn(text, rawURL string) Button {
	return internalbot.URLBtn(text, rawURL)
}

func WithButtons(rows ...ButtonRow) SendOption {
	return internalbot.WithButtons(rows...)
}

func WithMedia(media ...OutMedia) SendOption {
	return internalbot.WithMedia(media...)
}

func Image(path string) OutMedia {
	return internalbot.Image(path)
}

func ImageBytes(name string, raw []byte) OutMedia {
	return internalbot.ImageBytes(name, raw)
}

func Video(path string) OutMedia {
	return internalbot.Video(path)
}

func VideoBytes(name string, raw []byte) OutMedia {
	return internalbot.VideoBytes(name, raw)
}

func Audio(path string) OutMedia {
	return internalbot.Audio(path)
}

func AudioBytes(name string, raw []byte) OutMedia {
	return internalbot.AudioBytes(name, raw)
}

func File(path string) OutMedia {
	return internalbot.File(path)
}

func FileBytes(name string, raw []byte) OutMedia {
	return internalbot.FileBytes(name, raw)
}
