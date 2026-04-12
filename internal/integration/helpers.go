package integration

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	internalbot "github.com/elum-bots/core/internal/bot"
)

func DownloadImageAttachment(ctx context.Context, services *Services, platform string, att internalbot.Attachment) ([]byte, string, error) {
	if services == nil {
		return nil, "", errors.New("integrations are not initialized")
	}
	if !attachmentLooksLikeImage(att) {
		return nil, "", errors.New("attachment is not an image")
	}

	switch normalizePlatform(platform) {
	case "tg":
		if services.TG == nil {
			return nil, "", errors.New("tg integration is not initialized")
		}
		fileID := strings.TrimSpace(att.ID)
		if fileID == "" {
			return nil, "", errors.New("tg attachment file_id is empty")
		}
		return services.TG.DownloadFileByID(ctx, fileID)
	case "max":
		if services.MAX == nil {
			return nil, "", errors.New("max integration is not initialized")
		}
		rawURL := strings.TrimSpace(att.URL)
		if rawURL == "" {
			return nil, "", errors.New("max attachment url is empty")
		}
		return services.MAX.DownloadFile(ctx, rawURL)
	default:
		return nil, "", fmt.Errorf("unsupported platform %q", platform)
	}
}

func DownloadImageFromUpdate(ctx context.Context, services *Services, upd internalbot.Update) ([]byte, string, error) {
	att, err := firstImageAttachment(upd.Message.Attachments)
	if err != nil {
		return nil, "", err
	}
	return DownloadImageAttachment(ctx, services, upd.Platform, att)
}

func IsSubscribedToChannel(ctx context.Context, services *Services, platform, platformUserID, channelID string) (bool, error) {
	if services == nil {
		return false, errors.New("integrations are not initialized")
	}

	switch normalizePlatform(platform) {
	case "tg":
		if services.TG == nil {
			return false, errors.New("tg integration is not initialized")
		}
		member, err := services.TG.GetChatMember(ctx, channelID, platformUserID)
		if err != nil {
			return false, err
		}
		return member.IsMember(), nil
	case "max":
		if services.MAX == nil {
			return false, errors.New("max integration is not initialized")
		}
		userID, err := strconv.ParseInt(strings.TrimSpace(platformUserID), 10, 64)
		if err != nil || userID <= 0 {
			return false, fmt.Errorf("invalid platform user id %q", platformUserID)
		}
		chatID, err := strconv.ParseInt(strings.TrimSpace(channelID), 10, 64)
		if err != nil || chatID == 0 {
			return false, fmt.Errorf("invalid channel id %q", channelID)
		}
		return services.MAX.IsUserMember(ctx, chatID, userID)
	default:
		return false, fmt.Errorf("unsupported platform %q", platform)
	}
}

func firstImageAttachment(attachments []internalbot.Attachment) (internalbot.Attachment, error) {
	for _, att := range attachments {
		if attachmentLooksLikeImage(att) {
			return att, nil
		}
	}
	return internalbot.Attachment{}, errors.New("image attachment not found")
}

func attachmentLooksLikeImage(att internalbot.Attachment) bool {
	mime := strings.ToLower(strings.TrimSpace(att.MIME))
	if strings.HasPrefix(mime, "image/") || mime == "image/*" {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(att.Name))
	if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".gif") || strings.HasSuffix(name, ".webp") || strings.HasSuffix(name, ".bmp") || strings.HasSuffix(name, ".heic") || strings.HasSuffix(name, ".heif") {
		return true
	}
	rawURL := strings.ToLower(strings.TrimSpace(att.URL))
	return strings.Contains(rawURL, ".jpg") || strings.Contains(rawURL, ".jpeg") || strings.Contains(rawURL, ".png") || strings.Contains(rawURL, ".gif") || strings.Contains(rawURL, ".webp") || strings.Contains(rawURL, ".bmp") || strings.Contains(rawURL, ".heic") || strings.Contains(rawURL, ".heif")
}

func normalizePlatform(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "telegram":
		return "tg"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}
